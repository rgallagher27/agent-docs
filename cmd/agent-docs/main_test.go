package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
			err := run(tt.args, &stdout, &stderr)

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

func TestServeWithValidConfig(t *testing.T) {
	cfgPath := writeTempConfig(t, validConfig)

	var stdout, stderr bytes.Buffer
	err := run([]string{"serve", "--config", cfgPath}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("err = %v, want 'not implemented'", err)
	}
	if !strings.Contains(stdout.String(), "would serve on 127.0.0.1:8080") {
		t.Errorf("stdout = %q, missing serve preview", stdout.String())
	}
	if !strings.Contains(stdout.String(), "1 project(s) loaded") {
		t.Errorf("stdout = %q, missing project count", stdout.String())
	}
}

func TestServeAddrOverridesConfig(t *testing.T) {
	cfgPath := writeTempConfig(t, validConfig)

	var stdout, stderr bytes.Buffer
	_ = run([]string{"serve", "--config", cfgPath, "--addr", "127.0.0.1:9999"}, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "127.0.0.1:9999") {
		t.Errorf("stdout = %q, expected --addr override to appear", stdout.String())
	}
}

func TestFetchUnknownProject(t *testing.T) {
	cfgPath := writeTempConfig(t, validConfig)

	var stdout, stderr bytes.Buffer
	err := run([]string{"fetch", "--config", cfgPath, "--project", "does-not-exist"}, &stdout, &stderr)
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
	if err := run([]string{"fetch", "--config", cfgPath}, &stdout, &stderr); err != nil {
		t.Fatalf("fetch: %v\nstderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "fetched fixture") {
		t.Errorf("stdout = %q, missing 'fetched fixture'", stdout.String())
	}
}

const validConfig = `
[[project]]
slug = "test"
remote = "/tmp/test.git"
clone_path = "/tmp/clones/test"
`

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

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
