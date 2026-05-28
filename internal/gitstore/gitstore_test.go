package gitstore

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// makeBareRemote creates a bare git repo with one commit on `main`
// containing README.md, and returns its path. Suitable as a clone target.
func makeBareRemote(t *testing.T) string {
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

func openStore(t *testing.T) *Store {
	t.Helper()
	remote := makeBareRemote(t)
	clonePath := filepath.Join(t.TempDir(), "clone.git")
	s, err := Open(remote, clonePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

func TestOpen_ClonesIfMissing(t *testing.T) {
	remote := makeBareRemote(t)
	clonePath := filepath.Join(t.TempDir(), "clone.git")

	store, err := Open(remote, clonePath)
	if err != nil {
		t.Fatal(err)
	}
	if store == nil {
		t.Fatal("nil store")
	}
	if _, err := os.Stat(filepath.Join(clonePath, "HEAD")); err != nil {
		t.Errorf("HEAD missing in clone: %v", err)
	}
}

func TestOpen_OpensExistingClone(t *testing.T) {
	remote := makeBareRemote(t)
	clonePath := filepath.Join(t.TempDir(), "clone.git")

	if _, err := Open(remote, clonePath); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(remote, clonePath); err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
}

func TestReadBlob(t *testing.T) {
	s := openStore(t)
	got, err := s.ReadBlob("main", "README.md")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("# test\n")) {
		t.Errorf("got %q, want %q", got, "# test\n")
	}
}

func TestReadBlob_UnknownRef(t *testing.T) {
	s := openStore(t)
	if _, err := s.ReadBlob("nope", "README.md"); err == nil {
		t.Error("want error for unknown ref")
	}
}

func TestReadBlob_UnknownPath(t *testing.T) {
	s := openStore(t)
	if _, err := s.ReadBlob("main", "no-such-file.md"); err == nil {
		t.Error("want error for unknown path")
	}
}

func TestListRefs_IncludesMain(t *testing.T) {
	s := openStore(t)
	refs, err := s.ListRefs()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range refs {
		if r.Name == "main" && r.Kind == RefBranch && len(r.SHA) == 40 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("main not in refs: %+v", refs)
	}
}

func TestCommit_AndReadBack(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()

	content := []byte("<html>hello</html>")
	sha, err := s.Commit(ctx, "main", "plans/foo.html", content,
		"add foo plan", Author{Name: "Tester", Email: "t@t"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sha) != 40 {
		t.Errorf("sha = %q, want 40 hex chars", sha)
	}

	got, err := s.ReadBlob("main", "plans/foo.html")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestCommit_PushedToRemote(t *testing.T) {
	remote := makeBareRemote(t)
	clonePath := filepath.Join(t.TempDir(), "clone.git")
	s, err := Open(remote, clonePath)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	want := []byte("pushed content")
	if _, err := s.Commit(ctx, "main", "x.html", want, "x", Author{Name: "T", Email: "t@t"}); err != nil {
		t.Fatal(err)
	}

	// Independent clone reads the same blob — proves the commit was pushed.
	clone2 := filepath.Join(t.TempDir(), "clone2.git")
	s2, err := Open(remote, clone2)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s2.ReadBlob("main", "x.html")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCommit_NewNestedDirectory(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()

	if _, err := s.Commit(ctx, "main", "decisions/001/foo.html",
		[]byte("nested"), "nest", Author{Name: "T", Email: "t@t"}); err != nil {
		t.Fatal(err)
	}

	names, err := s.ListDir("main", "decisions/001")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "foo.html" {
		t.Errorf("names = %v, want [foo.html]", names)
	}
}

func TestCommit_RejectsBadInput(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	good := Author{Name: "T", Email: "t@t"}

	cases := []struct {
		name   string
		ref    string
		path   string
		author Author
	}{
		{"absolute path", "main", "/etc/passwd", good},
		{"empty path", "main", "", good},
		{"missing author", "main", "x.html", Author{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := s.Commit(ctx, c.ref, c.path, []byte("x"), "m", c.author); err == nil {
				t.Errorf("want error for %s", c.name)
			}
		})
	}
}

func TestListDir(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	auth := Author{Name: "T", Email: "t@t"}

	for _, f := range []string{"plans/a.html", "plans/b.html", "decisions/001.html"} {
		if _, err := s.Commit(ctx, "main", f, []byte("x"), "add", auth); err != nil {
			t.Fatalf("Commit %s: %v", f, err)
		}
	}

	names, err := s.ListDir("main", "plans")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "a.html" || names[1] != "b.html" {
		t.Errorf("plans/ = %v, want [a.html b.html]", names)
	}

	rootNames, err := s.ListDir("main", "")
	if err != nil {
		t.Fatal(err)
	}
	gotSet := map[string]bool{}
	for _, n := range rootNames {
		gotSet[n] = true
	}
	for _, want := range []string{"README.md", "plans/", "decisions/"} {
		if !gotSet[want] {
			t.Errorf("root listing %v missing %q", rootNames, want)
		}
	}
}

func TestCommit_AfterOutOfBandFetch(t *testing.T) {
	// Reproduces the dogfood bug from 2026-05-28: an external process
	// pushes a new branch to the remote and runs `agent-docs fetch`,
	// so the bare clone's refs/heads/{branch} ref appears and points
	// at a commit object the long-running Store's go-git handle has
	// never seen. Without Commit's pre-write refresh, this fails with
	// "object not found"; with it, the commit lands.
	remote := makeBareRemote(t)
	clonePath := filepath.Join(t.TempDir(), "clone.git")
	store, err := Open(remote, clonePath)
	if err != nil {
		t.Fatal(err)
	}

	// External producer: clone the remote, branch + push.
	work := filepath.Join(t.TempDir(), "external-work")
	run := func(dir, name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	run("", "git", "clone", remote, work)
	run(work, "git", "checkout", "-b", "feat/external")
	if err := os.WriteFile(filepath.Join(work, "EXT.md"), []byte("ext\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(work, "git", "add", "EXT.md")
	run(work, "git",
		"-c", "user.name=Ext", "-c", "user.email=e@e",
		"commit", "-m", "external add")
	run(work, "git", "push", "origin", "feat/external")

	// Out-of-band fetch from a parallel "process": go-git Fetch into
	// the bare clone via a fresh Store handle (mimicking
	// `agent-docs fetch` running in another shell). This updates the
	// bare clone's refs and objects on disk, but the original `store`
	// handle's packfile index has not been refreshed.
	freshStore, err := Open(remote, clonePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := freshStore.Fetch(context.Background()); err != nil {
		t.Fatalf("oob fetch: %v", err)
	}

	// Original `store` writes to the newly-arrived branch. Without
	// the refresh inside Commit, the parent-commit lookup would fail
	// with "object not found" because the packfile holding it landed
	// after this Store was opened.
	sha, err := store.Commit(context.Background(), "feat/external",
		"hello.html", []byte("<html>hi</html>"),
		"add from original store after oob fetch",
		Author{Name: "T", Email: "t@t"})
	if err != nil {
		t.Fatalf("Commit on out-of-band branch: %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("sha = %q, want 40 hex chars", sha)
	}
}

func TestFetch_NoOpWhenUpToDate(t *testing.T) {
	s := openStore(t)
	if err := s.Fetch(context.Background()); err != nil {
		t.Errorf("Fetch: %v", err)
	}
}

func TestFetch_PicksUpRemoteChanges(t *testing.T) {
	remote := makeBareRemote(t)
	clonePath := filepath.Join(t.TempDir(), "clone.git")
	s, err := Open(remote, clonePath)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// Push a new commit to the remote via a separate working clone.
	work := filepath.Join(t.TempDir(), "extra-work")
	run := func(dir, name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	run("", "git", "clone", remote, work)
	if err := os.WriteFile(filepath.Join(work, "NEW.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(work, "git", "add", "NEW.md")
	run(work, "git", "-c", "user.name=T", "-c", "user.email=t@t", "commit", "-m", "add")
	run(work, "git", "push", "origin", "main")

	// Before fetch: our clone doesn't see NEW.md
	if _, err := s.ReadBlob("main", "NEW.md"); err == nil {
		t.Fatal("expected NEW.md missing before fetch")
	}

	if err := s.Fetch(ctx); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	// After fetch: NEW.md is visible on main.
	got, err := s.ReadBlob("main", "NEW.md")
	if err != nil {
		t.Fatalf("ReadBlob after fetch: %v", err)
	}
	if !bytes.Equal(got, []byte("hi\n")) {
		t.Errorf("got %q, want %q", got, "hi\n")
	}
}
