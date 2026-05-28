// Package server is the agent-docs HTTP layer. It routes branch-aware
// URLs (per ADR-003) onto the per-project gitstore and serves doc
// content verbatim — no rendering of doc bodies (per ADR-002).
package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/rgallagher/agent-docs/internal/config"
	"github.com/rgallagher/agent-docs/internal/gitstore"
)

// Server holds the open gitstore handles for each configured project
// and serves HTTP requests by routing onto them.
type Server struct {
	mu       sync.RWMutex
	projects map[string]*projectEntry
}

type projectEntry struct {
	cfg   config.Project
	store *gitstore.Store
}

// New opens the bare clone for each project in cfg and returns a Server
// ready to serve. An error from any project's Open aborts setup.
func New(cfg *config.Config) (*Server, error) {
	projects := make(map[string]*projectEntry, len(cfg.Projects))
	for _, p := range cfg.Projects {
		store, err := gitstore.Open(p.Remote, p.ClonePath)
		if err != nil {
			return nil, fmt.Errorf("open project %s: %w", p.Slug, err)
		}
		projects[p.Slug] = &projectEntry{cfg: p, store: store}
	}
	return &Server{projects: projects}, nil
}

// Handler returns the HTTP handler this server exposes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /p/{proj}/{rest...}", s.handleDoc)
	mux.HandleFunc("GET /p/{proj}", s.handleProjectRoot)
	return mux
}

// handleProjectRoot redirects /p/{slug} → /p/{slug}/ so relative links
// in served docs resolve correctly.
func (s *Server) handleProjectRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
}

// handleDoc serves the doc at /p/{proj}/{rest...}, resolving rest into
// (ref, path) per ADR-003: if the first segment of rest is a known ref
// in the project's bare clone, treat it as explicit; otherwise rest is
// a trunk-relative path.
func (s *Server) handleDoc(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("proj")
	rest := r.PathValue("rest")

	entry := s.lookupProject(slug)
	if entry == nil {
		http.NotFound(w, r)
		return
	}

	ref, path, err := resolveRefAndPath(entry, rest)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	content, err := entry.store.ReadBlob(ref, path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", contentType(path))
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(content)
}

func (s *Server) lookupProject(slug string) *projectEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.projects[slug]
}

// resolveRefAndPath splits rest into a (ref, doc-path) pair. If rest is
// empty, defaults to the project's trunk ref and index.html. If the
// first segment of rest matches a known ref in the bare clone, treat as
// explicit; otherwise rest is trunk-relative.
func resolveRefAndPath(entry *projectEntry, rest string) (string, string, error) {
	if rest == "" {
		return entry.cfg.TrunkRef, "index.html", nil
	}

	first, remainder, _ := strings.Cut(rest, "/")
	if isKnownRef(entry.store, first) {
		path := remainder
		if path == "" {
			path = "index.html"
		}
		return first, path, nil
	}
	return entry.cfg.TrunkRef, rest, nil
}

// isKnownRef returns true if name resolves to a branch or tag in the
// store. SHA-prefix permalinks (per ADR-003) are not yet supported and
// will fall through to trunk-relative interpretation — Q11 follow-up.
func isKnownRef(store *gitstore.Store, name string) bool {
	if name == "" {
		return false
	}
	refs, err := store.ListRefs()
	if err != nil {
		return false
	}
	for _, r := range refs {
		if r.Name == name {
			return true
		}
	}
	return false
}

// contentType returns the MIME type for path's extension. MVP knows
// html, css, js, svg, png, json, txt; anything else gets the
// generic application/octet-stream so the browser does not auto-render.
func contentType(path string) string {
	switch {
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(path, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".txt"), strings.HasSuffix(path, ".md"):
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// ErrNoProjects is returned by New when cfg has no projects to open.
// Currently the config validator catches this earlier; keeping it as a
// belt-and-braces signal for in-code construction.
var ErrNoProjects = errors.New("no projects configured")
