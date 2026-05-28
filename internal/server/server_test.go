package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rgallagher/agent-docs/internal/config"
	"github.com/rgallagher/agent-docs/internal/gitstore"
)

// makeRemoteWithDocs creates a bare git remote and commits a small
// docs tree on main: index.html, plans/foo.html.
func makeRemoteWithDocs(t *testing.T) string {
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
		"index.html":      "<html>root</html>",
		"plans/foo.html":  "<html>plan foo</html>",
		"plans/bar.html":  "<html>plan bar</html>",
		"reviews/r1.html": "<html>review r1</html>",
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

	// Create a feature branch with a different doc.
	run(workDir, "git", "checkout", "-b", "feat-x")
	if err := os.WriteFile(filepath.Join(workDir, "plans/foo.html"),
		[]byte("<html>plan foo on feat-x</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(workDir, "git", "add", "plans/foo.html")
	run(workDir, "git",
		"-c", "user.name=T", "-c", "user.email=t@t",
		"commit", "-m", "feat-x change")
	run(workDir, "git", "push", "origin", "feat-x")

	return remoteDir
}

func setupServer(t *testing.T) (*Server, string) {
	t.Helper()
	remote := makeRemoteWithDocs(t)
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
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv, remote
}

func get(t *testing.T, h http.Handler, url string) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return res, string(body)
}

func TestHandler_TrunkAliasServesIndex(t *testing.T) {
	srv, _ := setupServer(t)
	h := srv.Handler()

	res, body := get(t, h, "/p/demo/")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if !strings.Contains(body, "root") {
		t.Errorf("body = %q, want trunk root index", body)
	}
}

func TestHandler_TrunkAliasServesNestedDoc(t *testing.T) {
	srv, _ := setupServer(t)
	h := srv.Handler()

	res, body := get(t, h, "/p/demo/plans/foo.html")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if !strings.Contains(body, "plan foo") {
		t.Errorf("body = %q, want trunk plan foo content", body)
	}
	if got := res.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Errorf("content-type = %q, want text/html", got)
	}
}

func TestHandler_ExplicitRefBranch(t *testing.T) {
	srv, _ := setupServer(t)
	h := srv.Handler()

	res, body := get(t, h, "/p/demo/feat-x/plans/foo.html")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if !strings.Contains(body, "on feat-x") {
		t.Errorf("body = %q, want feat-x branch content", body)
	}
}

func TestHandler_ExplicitRefRootIndex(t *testing.T) {
	srv, _ := setupServer(t)
	h := srv.Handler()

	// /p/demo/main/ should serve trunk's index.html, demonstrating
	// the explicit-ref form (it's the same content as the trunk alias).
	res, body := get(t, h, "/p/demo/main/")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if !strings.Contains(body, "root") {
		t.Errorf("body = %q, want trunk root index", body)
	}
}

func TestHandler_ExplicitRefMain(t *testing.T) {
	srv, _ := setupServer(t)
	h := srv.Handler()

	res, body := get(t, h, "/p/demo/main/plans/foo.html")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if !strings.Contains(body, "plan foo") {
		t.Errorf("body = %q, want main's plan foo content", body)
	}
}

func TestHandler_UnknownProject(t *testing.T) {
	srv, _ := setupServer(t)
	h := srv.Handler()

	res, _ := get(t, h, "/p/no-such-project/")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", res.StatusCode)
	}
}

func TestHandler_UnknownDoc(t *testing.T) {
	srv, _ := setupServer(t)
	h := srv.Handler()

	res, _ := get(t, h, "/p/demo/plans/missing.html")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", res.StatusCode)
	}
}

func TestHandler_UnknownRefFallsThroughToTrunk(t *testing.T) {
	// /p/demo/nonexistent/plans/foo.html: "nonexistent" isn't a ref,
	// so this is interpreted as trunk-relative path
	// "nonexistent/plans/foo.html" — which doesn't exist either, so 404.
	srv, _ := setupServer(t)
	h := srv.Handler()

	res, _ := get(t, h, "/p/demo/nonexistent/plans/foo.html")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", res.StatusCode)
	}
}

func TestHandler_ProjectRootRedirect(t *testing.T) {
	srv, _ := setupServer(t)
	h := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/p/demo", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Errorf("status = %d, want 301", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/p/demo/" {
		t.Errorf("location = %q, want /p/demo/", loc)
	}
}

func TestHandler_ServesViaHTTPTestServer(t *testing.T) {
	// End-to-end through a real httptest.Server so context, headers,
	// and the actual http.Server stack are all in the loop.
	srv, _ := setupServer(t)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Get(ts.URL + "/p/demo/plans/foo.html")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", res.StatusCode)
	}
	if !strings.Contains(string(body), "plan foo") {
		t.Errorf("body = %q, want plan foo content", body)
	}
}

func TestResolveRefAndPath(t *testing.T) {
	remote := makeRemoteWithDocs(t)
	clonePath := filepath.Join(t.TempDir(), "clone.git")
	store, err := gitstore.Open(remote, clonePath)
	if err != nil {
		t.Fatal(err)
	}
	entry := &projectEntry{
		cfg:   config.Project{Slug: "demo", TrunkRef: "main"},
		store: store,
	}

	tests := []struct {
		name     string
		rest     string
		wantRef  string
		wantPath string
	}{
		{"empty resolves to trunk index", "", "main", "index.html"},
		{"explicit main + nested", "main/plans/foo.html", "main", "plans/foo.html"},
		{"explicit main + root", "main/", "main", "index.html"},
		{"explicit feat-x", "feat-x/plans/foo.html", "feat-x", "plans/foo.html"},
		{"unknown first segment falls through", "weird/path.html", "main", "weird/path.html"},
		{"trunk-relative nested", "plans/foo.html", "main", "plans/foo.html"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRef, gotPath, err := resolveRefAndPath(entry, tt.rest)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotRef != tt.wantRef || gotPath != tt.wantPath {
				t.Errorf("got (%q, %q), want (%q, %q)", gotRef, gotPath, tt.wantRef, tt.wantPath)
			}
		})
	}
}

func TestHandler_AutoIndex_DirectoryWithSlash(t *testing.T) {
	srv, _ := setupServer(t)
	h := srv.Handler()

	res, body := get(t, h, "/p/demo/main/plans/")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody=%s", res.StatusCode, body)
	}
	if got := res.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Errorf("content-type = %q, want text/html", got)
	}
	for _, want := range []string{
		"auto-generated",
		"foo.html",
		"bar.html",
		`<h1>plans/</h1>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n--- body ---\n%s", want, body)
		}
	}
}

func TestHandler_AutoIndex_RootOfRef(t *testing.T) {
	srv, _ := setupServer(t)
	h := srv.Handler()

	// /p/demo/main/ → root of main. Committed index.html exists, so the
	// committed one wins over auto-index.
	res, body := get(t, h, "/p/demo/main/")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if !strings.Contains(body, "<html>root</html>") {
		t.Errorf("body = %q, want committed root index", body)
	}
	if strings.Contains(body, "auto-generated") {
		t.Errorf("committed index.html should win over auto-index: %q", body)
	}
}

func TestHandler_AutoIndex_DirectoryNoSlashRedirects(t *testing.T) {
	srv, _ := setupServer(t)
	h := srv.Handler()

	// /p/demo/main/plans → 301 to /p/demo/main/plans/
	req := httptest.NewRequest(http.MethodGet, "/p/demo/main/plans", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want 301", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/p/demo/main/plans/" {
		t.Errorf("location = %q, want /p/demo/main/plans/", loc)
	}
}

func TestHandler_AutoIndex_ServesAtSectionIndexPath(t *testing.T) {
	srv, _ := setupServer(t)
	h := srv.Handler()

	// reviews/ has no committed index.html, so requesting /reviews/index.html
	// hits the auto-index fallback rather than 404'ing.
	res, body := get(t, h, "/p/demo/main/reviews/index.html")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody=%s", res.StatusCode, body)
	}
	if !strings.Contains(body, "r1.html") {
		t.Errorf("auto-index for /reviews missing r1.html: %s", body)
	}
	if !strings.Contains(body, "auto-generated") {
		t.Errorf("missing auto-generated marker: %s", body)
	}
}

func TestHandler_AutoIndex_BreadcrumbLinks(t *testing.T) {
	srv, _ := setupServer(t)
	h := srv.Handler()

	_, body := get(t, h, "/p/demo/main/plans/")
	for _, want := range []string{
		`<a href="/p/demo/">demo</a>`,
		`<a href="/p/demo/main/">main</a>`,
		`<span>plans</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("breadcrumb missing %q\n--- body ---\n%s", want, body)
		}
	}
}

func TestConvertEntries(t *testing.T) {
	got := convertEntries([]string{"foo.html", "index.html", "subdir/", "bar.html"})
	if len(got) != 3 {
		t.Fatalf("got %d entries, want 3 (index.html should be skipped)", len(got))
	}
	for _, e := range got {
		if e.Label == "index.html" {
			t.Errorf("index.html not omitted: %+v", got)
		}
	}

	// Spot-check dir-flag derivation
	var foundSubdir bool
	for _, e := range got {
		if e.Label == "subdir/" {
			foundSubdir = true
			if !e.IsDir {
				t.Errorf("subdir/ should be flagged IsDir=true: %+v", got)
			}
		}
	}
	if !foundSubdir {
		t.Errorf("subdir/ missing from converted entries: %+v", got)
	}
}

func TestDirToList(t *testing.T) {
	tests := []struct {
		name    string
		rest    string
		path    string
		wantDir string
		wantOK  bool
	}{
		{"project root", "", "index.html", "", true},
		{"ref root", "main/", "index.html", "", true},
		{"section trailing slash", "main/plans/", "plans/", "plans", true},
		{"section index.html", "main/plans/index.html", "plans/index.html", "plans", true},
		{"deep section", "main/decisions/001/", "decisions/001/", "decisions/001", true},
		{"plain file", "main/plans/foo.html", "plans/foo.html", "", false},
		{"no-slash dir name", "main/plans", "plans", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDir, gotOK := dirToList(tt.rest, tt.path)
			if gotDir != tt.wantDir || gotOK != tt.wantOK {
				t.Errorf("got (%q, %v), want (%q, %v)", gotDir, gotOK, tt.wantDir, tt.wantOK)
			}
		})
	}
}

func TestContentType(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"foo.html", "text/html; charset=utf-8"},
		{"foo.css", "text/css; charset=utf-8"},
		{"foo.svg", "image/svg+xml"},
		{"foo.json", "application/json; charset=utf-8"},
		{"foo.md", "text/plain; charset=utf-8"},
		{"foo.unknown", "application/octet-stream"},
	}
	for _, tt := range tests {
		if got := contentType(tt.path); got != tt.want {
			t.Errorf("contentType(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

