package cmd

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/slyds/core"
)

// TestLocalWorkspace_OpenDeck_Basic verifies that a LocalWorkspace can open
// a deck that exists as a subdirectory under the workspace root. This is
// the happy-path replacement for the old TestOpenDeck which exercised the
// package-private openDeck(root, name) helper.
func TestLocalWorkspace_OpenDeck_Basic(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Alpha Talk", 3, "default", filepath.Join(root, "alpha"), true)

	ws, err := NewLocalWorkspace(root)
	if err != nil {
		t.Fatalf("NewLocalWorkspace: %v", err)
	}

	d, err := ws.OpenDeck("alpha")
	if err != nil {
		t.Fatalf("OpenDeck: %v", err)
	}
	if d.Title() != "Alpha Talk" {
		t.Errorf("Title = %q, want %q", d.Title(), "Alpha Talk")
	}
	count, _ := d.SlideCount()
	if count != 3 {
		t.Errorf("SlideCount = %d, want 3", count)
	}
}

// TestLocalWorkspace_OpenDeck_InvalidName verifies that deck names containing
// path separators or leading dots are rejected with ErrInvalidDeckName before
// any filesystem access happens. This protects against path escape bugs and
// reserves "/" for future multi-root workspace name qualification.
func TestLocalWorkspace_OpenDeck_InvalidName(t *testing.T) {
	root := t.TempDir()
	ws, _ := NewLocalWorkspace(root)

	cases := []string{
		"../escape",
		"root/sub",
		"..secret",
		`root\sub`,
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ws.OpenDeck(name)
			if !errors.Is(err, ErrInvalidDeckName) {
				t.Errorf("OpenDeck(%q) error = %v, want ErrInvalidDeckName", name, err)
			}
		})
	}
}

// TestLocalWorkspace_OpenDeck_NotFound verifies that opening a non-existent
// deck returns ErrDeckNotFound rather than a low-level filesystem error.
// Uniform not-found mapping matters for the future HostedWorkspace where
// leaking "exists but unauthorized" vs "doesn't exist" would be a tenancy
// information leak.
func TestLocalWorkspace_OpenDeck_NotFound(t *testing.T) {
	root := t.TempDir()
	ws, _ := NewLocalWorkspace(root)

	_, err := ws.OpenDeck("ghost")
	if !errors.Is(err, ErrDeckNotFound) {
		t.Errorf("OpenDeck(\"ghost\") error = %v, want ErrDeckNotFound", err)
	}
}

// TestLocalWorkspace_OpenDeck_RootDeck verifies that the "." sentinel name
// opens the deck at the workspace root itself (when one exists). This
// supports the "my project IS a deck" case where --deck-root points at a
// deck directory directly.
func TestLocalWorkspace_OpenDeck_RootDeck(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Root Deck", 2, "default", root, true)

	ws, _ := NewLocalWorkspace(root)
	d, err := ws.OpenDeck(".")
	if err != nil {
		t.Fatalf("OpenDeck(\".\"): %v", err)
	}
	if d.Title() != "Root Deck" {
		t.Errorf("Title = %q, want %q", d.Title(), "Root Deck")
	}
}

// TestLocalWorkspace_ListDecks_Empty verifies that an empty workspace
// directory yields zero refs (and not an error). Regression guard for
// the old TestDiscoverDecksEmpty.
func TestLocalWorkspace_ListDecks_Empty(t *testing.T) {
	ws, _ := NewLocalWorkspace(t.TempDir())
	refs, err := ws.ListDecks()
	if err != nil {
		t.Fatalf("ListDecks: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("ListDecks = %v, want 0 refs", refs)
	}
}

// TestLocalWorkspace_ListDecks_Multiple verifies that every subdirectory
// containing index.html is returned as a DeckRef. Regression guard for
// the old TestDiscoverDecks.
func TestLocalWorkspace_ListDecks_Multiple(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Alpha", 2, "default", filepath.Join(root, "alpha"), true)
	core.CreateInDir("Beta", 3, "dark", filepath.Join(root, "beta"), true)
	core.CreateInDir("Gamma", 4, "default", filepath.Join(root, "gamma"), true)

	ws, _ := NewLocalWorkspace(root)
	refs, err := ws.ListDecks()
	if err != nil {
		t.Fatalf("ListDecks: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("ListDecks: got %d refs, want 3: %v", len(refs), refs)
	}

	got := map[string]bool{}
	for _, r := range refs {
		got[r.Name] = true
	}
	for _, want := range []string{"alpha", "beta", "gamma"} {
		if !got[want] {
			t.Errorf("ListDecks missing %q; got %v", want, refs)
		}
	}
}

// TestLocalWorkspace_ListDecks_RootIsDeck verifies that when the workspace
// root itself contains index.html, ListDecks returns a "." entry. Regression
// guard for the old TestDiscoverDecksRootIsDeck.
func TestLocalWorkspace_ListDecks_RootIsDeck(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Root Deck", 2, "default", root, true)

	ws, _ := NewLocalWorkspace(root)
	refs, _ := ws.ListDecks()

	found := false
	for _, r := range refs {
		if r.Name == "." {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("root deck not listed as \".\": %v", refs)
	}
}

// TestLocalWorkspace_CreateDeck verifies that CreateDeck scaffolds a new
// deck, returns the opened Deck with correct title/theme/slide count, and
// that the new deck is subsequently visible to ListDecks.
func TestLocalWorkspace_CreateDeck(t *testing.T) {
	root := t.TempDir()
	ws, _ := NewLocalWorkspace(root)

	d, err := ws.CreateDeck("new", "New Deck", "default", 3)
	if err != nil {
		t.Fatalf("CreateDeck: %v", err)
	}
	if d.Title() != "New Deck" {
		t.Errorf("Title = %q, want %q", d.Title(), "New Deck")
	}
	if d.Theme() != "default" {
		t.Errorf("Theme = %q, want \"default\"", d.Theme())
	}
	count, _ := d.SlideCount()
	if count != 3 {
		t.Errorf("SlideCount = %d, want 3", count)
	}

	refs, _ := ws.ListDecks()
	found := false
	for _, r := range refs {
		if r.Name == "new" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("created deck not in ListDecks: %v", refs)
	}
}

// TestLocalWorkspace_CreateDeck_InvalidName verifies that CreateDeck rejects
// path-containing names the same way OpenDeck does. This is critical — a
// name like "../../etc/passwd" must never reach CreateInDir.
func TestLocalWorkspace_CreateDeck_InvalidName(t *testing.T) {
	ws, _ := NewLocalWorkspace(t.TempDir())

	_, err := ws.CreateDeck("../escape", "Title", "default", 1)
	if !errors.Is(err, ErrInvalidDeckName) {
		t.Errorf("CreateDeck error = %v, want ErrInvalidDeckName", err)
	}
}

// TestWorkspaceMiddleware_InjectsIntoContext verifies the wiring contract:
// when workspaceMiddleware(ws) wraps a handler, the handler sees ws via
// workspaceFromContext(ctx). This is the primitive every tool handler
// relies on — if this test fails, every tool in the server is broken.
func TestWorkspaceMiddleware_InjectsIntoContext(t *testing.T) {
	want, _ := NewLocalWorkspace(t.TempDir())

	var seen Workspace
	next := func(ctx context.Context, req *mcpcore.Request) *mcpcore.Response {
		seen = workspaceFromContext(ctx)
		return &mcpcore.Response{}
	}

	mw := workspaceMiddleware(want)
	mw(context.Background(), &mcpcore.Request{}, next)

	if seen == nil {
		t.Fatal("workspaceFromContext returned nil inside middleware-wrapped handler")
	}
	if seen != want {
		t.Errorf("workspaceFromContext returned %p, want %p", seen, want)
	}
}

// TestWorkspaceFromContext_MissingReturnsNil verifies that reading a
// workspace from a bare context (no middleware applied) returns nil. This
// is the contract requireWorkspace relies on to produce a clean error
// result instead of panicking on a nil-pointer dereference.
func TestWorkspaceFromContext_MissingReturnsNil(t *testing.T) {
	if ws := workspaceFromContext(context.Background()); ws != nil {
		t.Errorf("workspaceFromContext(bare ctx) = %v, want nil", ws)
	}
}
