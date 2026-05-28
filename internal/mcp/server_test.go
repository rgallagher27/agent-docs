package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rgallagher/agent-docs/internal/config"
	"github.com/rgallagher/agent-docs/internal/gitstore"
)

// setupServer builds an mcp.Server backed by a fresh bare remote with
// one commit on main containing README.md and plans/foo.html.
func setupServer(t *testing.T) (*Server, string) {
	t.Helper()
	remote := makeBareRemoteWithDocs(t)
	clonePath := filepath.Join(t.TempDir(), "clone.git")
	cfg := &config.Config{
		Projects: []config.Project{{
			Slug:      "demo",
			Remote:    remote,
			ClonePath: clonePath,
			TrunkRef:  "main",
		}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("config: %v", err)
	}
	srv, err := New(cfg, gitstore.Author{Name: "Tester", Email: "t@t"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv, remote
}

// exchange sends one or more NDJSON requests through Serve and returns
// the responses parsed as rpcMessages.
func exchange(t *testing.T, s *Server, requests ...string) []rpcMessage {
	t.Helper()
	in := strings.NewReader(strings.Join(requests, "\n") + "\n")
	var out bytes.Buffer
	if err := s.Serve(context.Background(), in, &out, io.Discard); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	var msgs []rpcMessage
	dec := json.NewDecoder(&out)
	for {
		var m rpcMessage
		if err := dec.Decode(&m); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("decode response: %v", err)
		}
		msgs = append(msgs, m)
	}
	return msgs
}

func TestNew_RequiresAuthor(t *testing.T) {
	cfg := &config.Config{Projects: []config.Project{{Slug: "x", Remote: "x", ClonePath: "/x"}}}
	_, err := New(cfg, gitstore.Author{})
	if err == nil || !strings.Contains(err.Error(), "author") {
		t.Errorf("expected author-required error, got %v", err)
	}
}

func TestInitialize(t *testing.T) {
	s, _ := setupServer(t)

	resps := exchange(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if len(resps) != 1 {
		t.Fatalf("got %d responses, want 1", len(resps))
	}

	var result initializeResult
	if err := json.Unmarshal(resps[0].Result, &result); err != nil {
		t.Fatalf("unmarshal initialize result: %v", err)
	}
	if result.ProtocolVersion != protocolVersion {
		t.Errorf("protocolVersion = %q, want %q", result.ProtocolVersion, protocolVersion)
	}
	if result.ServerInfo.Name != "agent-docs" {
		t.Errorf("serverInfo.name = %q", result.ServerInfo.Name)
	}
	if _, hasTools := result.Capabilities["tools"]; !hasTools {
		t.Errorf("capabilities missing tools: %+v", result.Capabilities)
	}
}

func TestInitializedNotification_NoResponse(t *testing.T) {
	s, _ := setupServer(t)

	// notifications/initialized has no id and expects no response.
	resps := exchange(t, s, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if len(resps) != 0 {
		t.Errorf("got %d responses, want 0 (notifications don't get responses)", len(resps))
	}
}

func TestToolsList(t *testing.T) {
	s, _ := setupServer(t)

	resps := exchange(t, s, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if len(resps) != 1 {
		t.Fatalf("got %d responses, want 1", len(resps))
	}

	var result toolsListResult
	if err := json.Unmarshal(resps[0].Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	gotNames := map[string]bool{}
	for _, t := range result.Tools {
		gotNames[t.Name] = true
	}
	for _, want := range []string{"write_doc", "read_doc", "list_docs"} {
		if !gotNames[want] {
			t.Errorf("tool %q missing; got %v", want, gotNames)
		}
	}
}

func TestToolsCall_WriteAndRead(t *testing.T) {
	s, _ := setupServer(t)

	writeReq := `{
		"jsonrpc": "2.0", "id": 10, "method": "tools/call",
		"params": {
			"name": "write_doc",
			"arguments": {
				"project": "demo",
				"ref": "main",
				"path": "plans/2026-05-28-hello.html",
				"content": "<html>hello world</html>"
			}
		}
	}`
	readReq := `{
		"jsonrpc": "2.0", "id": 11, "method": "tools/call",
		"params": {
			"name": "read_doc",
			"arguments": {
				"project": "demo",
				"path": "plans/2026-05-28-hello.html"
			}
		}
	}`

	resps := exchange(t, s, oneline(writeReq), oneline(readReq))
	if len(resps) != 2 {
		t.Fatalf("got %d responses, want 2", len(resps))
	}

	var write toolCallResult
	if err := json.Unmarshal(resps[0].Result, &write); err != nil {
		t.Fatalf("unmarshal write result: %v", err)
	}
	if write.IsError {
		t.Fatalf("write_doc returned isError=true: %s", write.Content[0].Text)
	}
	if !strings.Contains(write.Content[0].Text, "wrote plans/2026-05-28-hello.html") {
		t.Errorf("write result = %q, want confirmation", write.Content[0].Text)
	}

	var read toolCallResult
	if err := json.Unmarshal(resps[1].Result, &read); err != nil {
		t.Fatalf("unmarshal read result: %v", err)
	}
	if read.IsError {
		t.Fatalf("read_doc returned isError=true: %s", read.Content[0].Text)
	}
	if read.Content[0].Text != "<html>hello world</html>" {
		t.Errorf("read result = %q, want exact doc body", read.Content[0].Text)
	}
}

func TestToolsCall_ListDocs(t *testing.T) {
	s, _ := setupServer(t)

	resps := exchange(t, s, oneline(`{
		"jsonrpc": "2.0", "id": 20, "method": "tools/call",
		"params": {
			"name": "list_docs",
			"arguments": {"project": "demo", "section": "plans"}
		}
	}`))
	if len(resps) != 1 {
		t.Fatalf("got %d responses, want 1", len(resps))
	}

	var result toolCallResult
	if err := json.Unmarshal(resps[0].Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("list_docs returned isError=true: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "foo.html") {
		t.Errorf("list_docs body missing foo.html: %s", result.Content[0].Text)
	}
}

func TestToolsCall_MissingArgs_ReturnsToolError(t *testing.T) {
	s, _ := setupServer(t)

	// Missing 'content' on write_doc should come back as a tool-level
	// error (isError=true), not a protocol-level error.
	resps := exchange(t, s, oneline(`{
		"jsonrpc": "2.0", "id": 30, "method": "tools/call",
		"params": {
			"name": "write_doc",
			"arguments": {"project": "demo", "ref": "main", "path": "plans/x.html"}
		}
	}`))
	if len(resps) != 1 {
		t.Fatalf("got %d responses, want 1", len(resps))
	}
	if resps[0].Error != nil {
		t.Errorf("got protocol error %v, want tool-level error", resps[0].Error)
	}

	var result toolCallResult
	if err := json.Unmarshal(resps[0].Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Errorf("expected isError=true, got %+v", result)
	}
	if !strings.Contains(result.Content[0].Text, "required") {
		t.Errorf("error text = %q, expected 'required'", result.Content[0].Text)
	}
}

func TestToolsCall_UnknownTool(t *testing.T) {
	s, _ := setupServer(t)

	resps := exchange(t, s, oneline(`{
		"jsonrpc": "2.0", "id": 40, "method": "tools/call",
		"params": {"name": "no_such_tool", "arguments": {}}
	}`))
	if len(resps) != 1 {
		t.Fatalf("got %d responses, want 1", len(resps))
	}
	if resps[0].Error == nil || resps[0].Error.Code != errMethodNotFound {
		t.Errorf("expected method-not-found error, got %+v", resps[0])
	}
}

func TestUnknownMethod(t *testing.T) {
	s, _ := setupServer(t)

	resps := exchange(t, s, `{"jsonrpc":"2.0","id":50,"method":"no/such/method"}`)
	if len(resps) != 1 {
		t.Fatalf("got %d responses, want 1", len(resps))
	}
	if resps[0].Error == nil || resps[0].Error.Code != errMethodNotFound {
		t.Errorf("expected method-not-found, got %+v", resps[0])
	}
}

func TestParseError(t *testing.T) {
	s, _ := setupServer(t)

	// Malformed JSON.
	in := strings.NewReader("this is not json\n")
	var out bytes.Buffer
	if err := s.Serve(context.Background(), in, &out, io.Discard); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	sc := bufio.NewScanner(&out)
	if !sc.Scan() {
		t.Fatalf("no response written")
	}
	var resp rpcMessage
	if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != errParseError {
		t.Errorf("expected parse error, got %+v", resp)
	}
}

func TestWriteThenServerReadsItBack(t *testing.T) {
	// End-to-end: write a doc via MCP, then verify subsequent read sees
	// the new content (proves the commit landed on the bare clone).
	s, _ := setupServer(t)

	writeReq := oneline(`{
		"jsonrpc": "2.0", "id": 100, "method": "tools/call",
		"params": {
			"name": "write_doc",
			"arguments": {
				"project": "demo", "ref": "main",
				"path": "decisions/001-test.html",
				"content": "<html>adr 001</html>"
			}
		}
	}`)
	listReq := oneline(`{
		"jsonrpc": "2.0", "id": 101, "method": "tools/call",
		"params": {
			"name": "list_docs",
			"arguments": {"project": "demo", "section": "decisions"}
		}
	}`)

	resps := exchange(t, s, writeReq, listReq)
	if len(resps) != 2 {
		t.Fatalf("got %d responses", len(resps))
	}

	var list toolCallResult
	if err := json.Unmarshal(resps[1].Result, &list); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(list.Content[0].Text, "001-test.html") {
		t.Errorf("decisions/ listing missing new doc: %s", list.Content[0].Text)
	}
}

// oneline collapses a multi-line JSON literal into a single NDJSON line.
func oneline(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return s
}

// makeBareRemoteWithDocs creates a bare git repo with README.md and
// plans/foo.html committed on main. Mirrors the helpers in other
// internal test packages — small duplication for self-contained tests.
func makeBareRemoteWithDocs(t *testing.T) string {
	t.Helper()
	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	workDir := filepath.Join(t.TempDir(), "work")

	run := func(dir, name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
	}

	if err := os.MkdirAll(remoteDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run("", "git", "init", "--bare", "--initial-branch=main", remoteDir)

	if err := os.MkdirAll(filepath.Join(workDir, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	run(workDir, "git", "init", "--initial-branch=main")
	run(workDir, "git", "remote", "add", "origin", remoteDir)

	files := map[string]string{
		"README.md":      "# test\n",
		"plans/foo.html": "<html>plan foo</html>",
	}
	for path, content := range files {
		full := filepath.Join(workDir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run(workDir, "git", "add", ".")
	run(workDir, "git",
		"-c", "user.name=T", "-c", "user.email=t@t",
		"commit", "-m", "initial")
	run(workDir, "git", "push", "origin", "main")

	return remoteDir
}

