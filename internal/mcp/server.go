// Package mcp implements the Model Context Protocol server over stdio
// JSON-RPC. The agent-docs MVP exposes three tools (write_doc, read_doc,
// list_docs) that route through the per-project gitstore.
//
// Transport: NDJSON over stdin/stdout per the MCP stdio spec — one
// JSON-RPC 2.0 envelope per line. Logs and diagnostics go to stderr;
// stdout is reserved for the protocol.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/rgallagher/agent-docs/internal/config"
	"github.com/rgallagher/agent-docs/internal/gitstore"
)

// protocolVersion is the MCP spec version advertised on initialize.
const protocolVersion = "2024-11-05"

// Server holds open gitstore handles per project and dispatches MCP
// tool calls onto them. One Server instance per agent-docs process.
type Server struct {
	projects map[string]*projectEntry
	author   gitstore.Author
}

type projectEntry struct {
	cfg   config.Project
	store *gitstore.Store
}

// New opens the bare clone for each project in cfg. author is the
// identity recorded on commits made through write_doc.
func New(cfg *config.Config, author gitstore.Author) (*Server, error) {
	if author.Name == "" || author.Email == "" {
		return nil, errors.New("author name and email are required")
	}

	projects := make(map[string]*projectEntry, len(cfg.Projects))
	for _, p := range cfg.Projects {
		store, err := gitstore.Open(p.Remote, p.ClonePath)
		if err != nil {
			return nil, fmt.Errorf("open project %s: %w", p.Slug, err)
		}
		projects[p.Slug] = &projectEntry{cfg: p, store: store}
	}
	return &Server{projects: projects, author: author}, nil
}

// Serve runs the JSON-RPC loop. Reads NDJSON requests from in, writes
// NDJSON responses to out, logs diagnostics to stderr. Returns when
// in EOFs (clean client disconnect) or ctx cancels.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer, stderr io.Writer) error {
	scanner := bufio.NewScanner(in)
	// Allow up to 4 MiB per message — doc bodies can be large.
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	enc := json.NewEncoder(out)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req rpcMessage
		if err := json.Unmarshal(line, &req); err != nil {
			fmt.Fprintf(stderr, "mcp: parse error: %v\n", err)
			if encErr := enc.Encode(errorResponse(nil, errParseError, err.Error())); encErr != nil {
				return encErr
			}
			continue
		}

		resp := s.dispatch(ctx, &req)
		if resp != nil {
			if err := enc.Encode(resp); err != nil {
				fmt.Fprintf(stderr, "mcp: encode response: %v\n", err)
				return err
			}
		}

		if ctx.Err() != nil {
			return nil
		}
	}
	return scanner.Err()
}

// dispatch routes one JSON-RPC message to its handler. Returns nil for
// notifications (no response should be written).
func (s *Server) dispatch(ctx context.Context, req *rpcMessage) *rpcMessage {
	if req.JSONRPC != jsonRPCVersion {
		return errorResponse(req.ID, errInvalidRequest, "jsonrpc must be \"2.0\"")
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized":
		// Client confirmation that init is complete — no response.
		return nil
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "ping":
		return resultResponse(req.ID, map[string]any{})
	default:
		// Unknown notifications are dropped silently per JSON-RPC 2.0 §6.
		if req.ID == nil {
			return nil
		}
		return errorResponse(req.ID, errMethodNotFound, "method not found: "+req.Method)
	}
}

type initializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      serverInfo     `json:"serverInfo"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (s *Server) handleInitialize(req *rpcMessage) *rpcMessage {
	return resultResponse(req.ID, initializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities: map[string]any{
			"tools": map[string]any{},
		},
		ServerInfo: serverInfo{Name: "agent-docs", Version: "dev"},
	})
}

type toolsListResult struct {
	Tools []tool `json:"tools"`
}

func (s *Server) handleToolsList(req *rpcMessage) *rpcMessage {
	return resultResponse(req.ID, toolsListResult{Tools: tools})
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func (s *Server) handleToolsCall(ctx context.Context, req *rpcMessage) *rpcMessage {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, errInvalidParams, err.Error())
	}

	var result toolCallResult
	var err error

	switch params.Name {
	case "write_doc":
		result, err = s.callWriteDoc(ctx, params.Arguments)
	case "read_doc":
		result, err = s.callReadDoc(params.Arguments)
	case "list_docs":
		result, err = s.callListDocs(params.Arguments)
	default:
		return errorResponse(req.ID, errMethodNotFound, "unknown tool: "+params.Name)
	}

	if err != nil {
		// Tool errors come back as isError=true content, not as protocol
		// errors. This lets the agent see the message and recover.
		return resultResponse(req.ID, toolCallResult{
			Content: []contentBlock{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
	}
	return resultResponse(req.ID, result)
}

// Tool implementations ------------------------------------------------

func (s *Server) callWriteDoc(ctx context.Context, args map[string]any) (toolCallResult, error) {
	project, _ := args["project"].(string)
	ref, _ := args["ref"].(string)
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	message, _ := args["message"].(string)

	if project == "" || ref == "" || path == "" || content == "" {
		return toolCallResult{}, errors.New("project, ref, path, and content are required")
	}

	entry, ok := s.projects[project]
	if !ok {
		return toolCallResult{}, fmt.Errorf("unknown project: %s", project)
	}

	if message == "" {
		message = fmt.Sprintf("agent-docs: write %s", path)
	}

	sha, err := entry.store.Commit(ctx, ref, path, []byte(content), message, s.author)
	if err != nil {
		return toolCallResult{}, err
	}

	short := sha
	if len(short) > 7 {
		short = short[:7]
	}
	return toolCallResult{
		Content: []contentBlock{
			{Type: "text", Text: fmt.Sprintf("wrote %s at %s on %s/%s", path, short, project, ref)},
		},
	}, nil
}

func (s *Server) callReadDoc(args map[string]any) (toolCallResult, error) {
	project, _ := args["project"].(string)
	ref, _ := args["ref"].(string)
	path, _ := args["path"].(string)

	if project == "" || path == "" {
		return toolCallResult{}, errors.New("project and path are required")
	}

	entry, ok := s.projects[project]
	if !ok {
		return toolCallResult{}, fmt.Errorf("unknown project: %s", project)
	}
	if ref == "" {
		ref = entry.cfg.TrunkRef
	}

	content, err := entry.store.ReadBlob(ref, path)
	if err != nil {
		return toolCallResult{}, err
	}

	return toolCallResult{
		Content: []contentBlock{{Type: "text", Text: string(content)}},
	}, nil
}

func (s *Server) callListDocs(args map[string]any) (toolCallResult, error) {
	project, _ := args["project"].(string)
	ref, _ := args["ref"].(string)
	section, _ := args["section"].(string)

	if project == "" {
		return toolCallResult{}, errors.New("project is required")
	}

	entry, ok := s.projects[project]
	if !ok {
		return toolCallResult{}, fmt.Errorf("unknown project: %s", project)
	}
	if ref == "" {
		ref = entry.cfg.TrunkRef
	}

	entries, err := entry.store.ListDir(ref, section)
	if err != nil {
		return toolCallResult{}, err
	}

	var sb strings.Builder
	if section == "" {
		fmt.Fprintf(&sb, "Listing root of %s @ %s (%d entries):\n", project, ref, len(entries))
	} else {
		fmt.Fprintf(&sb, "Listing %s @ %s/%s (%d entries):\n", section, project, ref, len(entries))
	}
	for _, e := range entries {
		sb.WriteString("  " + e + "\n")
	}

	return toolCallResult{
		Content: []contentBlock{{Type: "text", Text: sb.String()}},
	}, nil
}
