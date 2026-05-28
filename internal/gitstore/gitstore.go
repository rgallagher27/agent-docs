// Package gitstore wraps go-git to provide a bare-clone-backed store
// for project documentation. Reads serve from any ref; writes commit
// to a target branch and push to origin.
package gitstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Store is a bare clone of one project's git repository.
type Store struct {
	repo *git.Repository
	path string
}

// Author is the identity recorded on commits made by the server.
type Author struct {
	Name  string
	Email string
}

// RefKind is whether a ref points at a branch or a tag.
type RefKind int

const (
	RefBranch RefKind = iota
	RefTag
)

// Ref is a local ref in the bare clone.
type Ref struct {
	Name string // short name: "main", "feat/foo", "v1.0"
	SHA  string // 40-char hex commit SHA the ref resolves to
	Kind RefKind
}

// Open returns a Store for path. If the bare clone doesn't exist yet,
// it is created by cloning from remote. remote may be a URL or a local
// filesystem path.
//
// After a fresh clone, Open also runs a mirror-semantic Fetch so all
// remote branches land in refs/heads/* — go-git's default clone refspec
// leaves non-default branches in refs/remotes/origin/* only, which
// would break multi-branch reads.
func Open(remote, path string) (*Store, error) {
	if isBareRepo(path) {
		repo, err := git.PlainOpen(path)
		if err != nil {
			return nil, fmt.Errorf("open clone at %s: %w", path, err)
		}
		return &Store{repo: repo, path: path}, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create clone parent dir: %w", err)
	}
	repo, err := git.PlainClone(path, true, &git.CloneOptions{URL: remote})
	if err != nil {
		return nil, fmt.Errorf("clone %s into %s: %w", remote, path, err)
	}
	store := &Store{repo: repo, path: path}

	// Re-fetch with mirror refspec so non-default branches populate
	// refs/heads/*. Fast — the objects are already local, only refs move.
	if err := store.Fetch(context.Background()); err != nil {
		return nil, fmt.Errorf("mirror refs after clone: %w", err)
	}
	return store, nil
}

// Fetch updates the local bare clone from origin using mirror semantics:
// remote branches land in refs/heads/*, remote tags in refs/tags/*. This
// is what a docs-mirror bare clone needs — go-git's default refspec maps
// to refs/remotes/origin/*, which would leave refs/heads stale after each
// fetch.
func (s *Store) Fetch(ctx context.Context) error {
	err := s.repo.FetchContext(ctx, &git.FetchOptions{
		RefSpecs: []config.RefSpec{
			"+refs/heads/*:refs/heads/*",
			"+refs/tags/*:refs/tags/*",
		},
		Force: true,
	})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

// ListRefs returns the local branches and tags.
func (s *Store) ListRefs() ([]Ref, error) {
	iter, err := s.repo.References()
	if err != nil {
		return nil, fmt.Errorf("list refs: %w", err)
	}
	defer iter.Close()

	var refs []Ref
	err = iter.ForEach(func(r *plumbing.Reference) error {
		if r.Type() != plumbing.HashReference {
			return nil
		}
		switch n := r.Name(); {
		case n.IsBranch():
			refs = append(refs, Ref{Name: n.Short(), SHA: r.Hash().String(), Kind: RefBranch})
		case n.IsTag():
			refs = append(refs, Ref{Name: n.Short(), SHA: r.Hash().String(), Kind: RefTag})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Name < refs[j].Name })
	return refs, nil
}

// ReadBlob returns the contents of path at ref. ref may be a branch
// name, a tag name, or a commit SHA (full or abbreviated).
func (s *Store) ReadBlob(ref, path string) ([]byte, error) {
	commit, err := s.resolveCommit(ref)
	if err != nil {
		return nil, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("tree for %s: %w", ref, err)
	}
	file, err := tree.File(path)
	if err != nil {
		return nil, fmt.Errorf("file %s at %s: %w", path, ref, err)
	}
	r, err := file.Reader()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// ListDir returns the entries under dir at ref. Directory entries
// carry a trailing "/". An empty or "." dir lists the root tree.
func (s *Store) ListDir(ref, dir string) ([]string, error) {
	commit, err := s.resolveCommit(ref)
	if err != nil {
		return nil, err
	}
	root, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("tree for %s: %w", ref, err)
	}

	t := root
	dir = strings.Trim(dir, "/")
	if dir != "" && dir != "." {
		t, err = root.Tree(dir)
		if err != nil {
			return nil, fmt.Errorf("dir %s at %s: %w", dir, ref, err)
		}
	}

	names := make([]string, 0, len(t.Entries))
	for _, e := range t.Entries {
		if e.Mode == filemode.Dir {
			names = append(names, e.Name+"/")
		} else {
			names = append(names, e.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// Commit writes content at path on ref, builds a commit with msg/author,
// updates the local ref, and pushes to origin. Returns the new commit's
// 40-char SHA. ref must already exist as a branch.
func (s *Store) Commit(ctx context.Context, ref, path string, content []byte, msg string, author Author) (string, error) {
	if path == "" || strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("path %q must be relative and non-empty", path)
	}
	if author.Name == "" || author.Email == "" {
		return "", errors.New("author name and email are required")
	}

	refName := plumbing.NewBranchReferenceName(ref)
	parentRef, err := s.repo.Reference(refName, true)
	if err != nil {
		return "", fmt.Errorf("resolve branch %s: %w", ref, err)
	}
	parentCommit, err := s.repo.CommitObject(parentRef.Hash())
	if err != nil {
		return "", fmt.Errorf("parent commit %s: %w", parentRef.Hash(), err)
	}
	parentTree, err := parentCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("parent tree: %w", err)
	}

	newTreeHash, err := s.buildTree(parentTree, strings.Split(path, "/"), content)
	if err != nil {
		return "", fmt.Errorf("build tree: %w", err)
	}

	now := time.Now()
	sig := object.Signature{Name: author.Name, Email: author.Email, When: now}
	newCommit := &object.Commit{
		Author:       sig,
		Committer:    sig,
		Message:      msg,
		TreeHash:     newTreeHash,
		ParentHashes: []plumbing.Hash{parentRef.Hash()},
	}
	commitHash, err := s.encodeObject(newCommit)
	if err != nil {
		return "", fmt.Errorf("encode commit: %w", err)
	}

	newRefObj := plumbing.NewHashReference(refName, commitHash)
	if err := s.repo.Storer.SetReference(newRefObj); err != nil {
		return "", fmt.Errorf("update ref %s: %w", ref, err)
	}

	pushSpec := config.RefSpec(string(refName) + ":" + string(refName))
	err = s.repo.PushContext(ctx, &git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{pushSpec},
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return "", fmt.Errorf("push %s: %w", ref, err)
	}
	return commitHash.String(), nil
}

// resolveCommit returns the commit object that ref points at. ref may
// be a branch, a tag, or a (possibly abbreviated) commit SHA.
func (s *Store) resolveCommit(ref string) (*object.Commit, error) {
	hash, err := s.resolveRef(ref)
	if err != nil {
		return nil, err
	}
	return s.repo.CommitObject(hash)
}

func (s *Store) resolveRef(ref string) (plumbing.Hash, error) {
	if r, err := s.repo.Reference(plumbing.NewBranchReferenceName(ref), true); err == nil {
		return r.Hash(), nil
	}
	if r, err := s.repo.Reference(plumbing.NewTagReferenceName(ref), true); err == nil {
		return r.Hash(), nil
	}
	if h, err := s.repo.ResolveRevision(plumbing.Revision(ref)); err == nil {
		return *h, nil
	}
	return plumbing.ZeroHash, fmt.Errorf("ref %q not found", ref)
}

// buildTree returns the hash of a tree object identical to base except
// that segments (a path split on "/") points to content. Walks subtrees
// recursively, creating intermediate trees as needed.
func (s *Store) buildTree(base *object.Tree, segments []string, content []byte) (plumbing.Hash, error) {
	if len(segments) == 0 {
		return plumbing.ZeroHash, errors.New("empty path segments")
	}
	name := segments[0]

	var entries []object.TreeEntry
	if base != nil {
		entries = append(entries, base.Entries...)
	}

	if len(segments) == 1 {
		blobHash, err := s.writeBlob(content)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		entries = withoutEntry(entries, name)
		entries = append(entries, object.TreeEntry{
			Name: name,
			Mode: filemode.Regular,
			Hash: blobHash,
		})
	} else {
		var sub *object.Tree
		for _, e := range entries {
			if e.Name == name && e.Mode == filemode.Dir {
				sub, _ = s.repo.TreeObject(e.Hash)
				break
			}
		}
		subHash, err := s.buildTree(sub, segments[1:], content)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		entries = withoutEntry(entries, name)
		entries = append(entries, object.TreeEntry{
			Name: name,
			Mode: filemode.Dir,
			Hash: subHash,
		})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	tree := &object.Tree{Entries: entries}
	return s.encodeObject(tree)
}

func (s *Store) writeBlob(content []byte) (plumbing.Hash, error) {
	obj := s.repo.Storer.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)
	obj.SetSize(int64(len(content)))
	w, err := obj.Writer()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if _, err := w.Write(content); err != nil {
		_ = w.Close()
		return plumbing.ZeroHash, err
	}
	if err := w.Close(); err != nil {
		return plumbing.ZeroHash, err
	}
	return s.repo.Storer.SetEncodedObject(obj)
}

// encodeObject serialises a go-git object (commit, tree) into the repo's storage.
type encoder interface {
	Encode(o plumbing.EncodedObject) error
}

func (s *Store) encodeObject(e encoder) (plumbing.Hash, error) {
	obj := s.repo.Storer.NewEncodedObject()
	if err := e.Encode(obj); err != nil {
		return plumbing.ZeroHash, err
	}
	return s.repo.Storer.SetEncodedObject(obj)
}

func withoutEntry(entries []object.TreeEntry, name string) []object.TreeEntry {
	out := entries[:0]
	for _, e := range entries {
		if e.Name != name {
			out = append(out, e)
		}
	}
	return out
}

// isBareRepo returns true if path appears to be an existing bare clone.
// (A bare repo has its objects + HEAD directly under path, not under .git.)
func isBareRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, "HEAD"))
	return err == nil
}
