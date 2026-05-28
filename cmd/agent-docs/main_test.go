package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantStdout  string
		wantErr     bool
		errContains string
	}{
		{name: "no args prints usage", args: nil, wantStdout: "Usage:"},
		{name: "help prints usage", args: []string{"help"}, wantStdout: "Usage:"},
		{name: "--help prints usage", args: []string{"--help"}, wantStdout: "Usage:"},
		{name: "-h prints usage", args: []string{"-h"}, wantStdout: "Usage:"},
		{name: "version prints version", args: []string{"version"}, wantStdout: "dev"},
		{name: "--version prints version", args: []string{"--version"}, wantStdout: "dev"},
		{name: "-v prints version", args: []string{"-v"}, wantStdout: "dev"},
		{name: "unknown command errors", args: []string{"banana"}, wantErr: true, errContains: "unknown command"},
		{
			name:        "serve with missing config errors",
			args:        []string{"serve", "--config", "/no/such/config.toml"},
			wantErr:     true,
			errContains: "read config",
		},
		{
			name:        "fetch with missing config errors",
			args:        []string{"fetch", "--config", "/no/such/config.toml"},
			wantErr:     true,
			errContains: "read config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run(context.Background(), tt.args, &stdout, &stderr)

			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errContains != "" && (err == nil || !strings.Contains(err.Error(), tt.errContains)) {
				t.Errorf("err = %v, want containing %q", err, tt.errContains)
			}
			if tt.wantStdout != "" && !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want containing %q", stdout.String(), tt.wantStdout)
			}
		})
	}
}

func TestFetchUnknownProject(t *testing.T) {
	cfgPath := writeTempConfig(t, validConfig)

	var stdout, stderr bytes.Buffer
	err := run(context.Background(),
		[]string{"fetch", "--config", cfgPath, "--project", "does-not-exist"},
		&stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want 'not found'", err)
	}
}

func TestFetchAllProjects_RealRemote(t *testing.T) {
	remote := makeBareRemoteForTest(t)
	clonePath := filepath.Join(t.TempDir(), "clone.git")
	cfg := fmt.Sprintf(`
[[project]]
slug = "fixture"
remote = %q
clone_path = %q
`, remote, clonePath)
	cfgPath := writeTempConfig(t, cfg)

	var stdout, stderr bytes.Buffer
	if err := run(context.Background(),
		[]string{"fetch", "--config", cfgPath}, &stdout, &stderr); err != nil {
		t.Fatalf("fetch: %v\nstderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "fetched fixture") {
		t.Errorf("stdout = %q, missing 'fetched fixture'", stdout.String())
	}
}

func TestServe_GracefulShutdownAndServesDocs(t *testing.T) {
	remote := makeBareRemoteForTest(t)
	clonePath := filepath.Join(t.TempDir(), "clone.git")

	// Pick a free port so the test doesn't collide with anything.
	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	cfg := fmt.Sprintf(`
[[project]]
slug = "fixture"
remote = %q
clone_path = %q
`, remote, clonePath)
	cfgPath := writeTempConfig(t, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var stdout, stderr bytes.Buffer
	var runErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runErr = run(ctx, []string{"serve", "--config", cfgPath, "--addr", addr}, &stdout, &stderr)
	}()

	// Wait until the server is accepting connections (max ~2 s).
	waitForListen(t, addr, 2*time.Second)

	// Hit the README we set up in the bare-remote fixture.
	res, err := http.Get("http://" + addr + "/p/fixture/README.md")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", res.StatusCode)
	}
	if !strings.Contains(string(body), "# test") {
		t.Errorf("body = %q, want README content", body)
	}

	// Trigger graceful shutdown.
	cancel()
	wg.Wait()
	if runErr != nil {
		t.Errorf("run returned: %v\nstderr=%s", runErr, stderr.String())
	}
	if !strings.Contains(stdout.String(), "listening on") {
		t.Errorf("stdout = %q, missing listen banner", stdout.String())
	}
}

func TestServe_AddrOverridesConfigBind(t *testing.T) {
	remote := makeBareRemoteForTest(t)
	clonePath := filepath.Join(t.TempDir(), "clone.git")
	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// Config says one bind; --addr should win.
	cfg := fmt.Sprintf(`
bind = "127.0.0.1:1"

[[project]]
slug = "fixture"
remote = %q
clone_path = %q
`, remote, clonePath)
	cfgPath := writeTempConfig(t, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var stdout, stderr bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = run(ctx, []string{"serve", "--config", cfgPath, "--addr", addr}, &stdout, &stderr)
	}()
	waitForListen(t, addr, 2*time.Second)
	cancel()
	wg.Wait()

	if !strings.Contains(stdout.String(), addr) {
		t.Errorf("stdout = %q, want listen banner with %q", stdout.String(), addr)
	}
}

const validConfig = `
[[project]]
slug = "test"
remote = "/tmp/test.git"
clone_path = "/tmp/clones/test"
`

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// freePort returns an OS-assigned free TCP port on the loopback
// interface. The listener is closed before returning so the caller can
// bind to the port; there is a small race window where another process
// could grab it, acceptable for tests.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

// waitForListen polls until a TCP connect to addr succeeds, or fails the
// test after timeout. Used to wait for serve to be ready.
func waitForListen(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server did not start listening on %s within %s", addr, timeout)
}

// makeBareRemoteForTest creates a bare git repo with one commit on main
// and returns its path. Mirrors the helper in internal/gitstore/gitstore_test.go;
// the small duplication keeps the cmd tests self-contained.
func makeBareRemoteForTest(t *testing.T) string {
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

	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(workDir, "git", "init", "--initial-branch=main")
	run(workDir, "git", "remote", "add", "origin", remoteDir)
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(workDir, "git", "add", "README.md")
	run(workDir, "git",
		"-c", "user.name=Tester",
		"-c", "user.email=t@t",
		"commit", "-m", "initial")
	run(workDir, "git", "push", "origin", "main")
	return remoteDir
}
