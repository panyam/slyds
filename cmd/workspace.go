package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/slyds/core"
)

// Workspace is a handle to a set of decks plus the policies for opening,
// listing, and creating them. MCP tool and resource handlers resolve decks
// via a Workspace — never through raw filesystem paths.
//
// Today there is one implementation (LocalWorkspace). HostedWorkspace and
// MultiRootWorkspace are planned (see issue #74 and #76). The interface
// intentionally exposes the minimum surface needed by the current MCP
// handlers; new methods are added only when the feature that needs them
// lands.
type Workspace interface {
	// OpenDeck resolves a deck name to a *core.Deck. Returns an error if
	// the deck doesn't exist, the name is invalid, or the caller is not
	// authorized to see it. Missing and unauthorized are both surfaced as
	// ErrDeckNotFound so future backends don't leak existence across
	// tenants.
	OpenDeck(name string) (*core.Deck, error)

	// ListDecks returns the decks visible to this workspace.
	ListDecks() ([]DeckRef, error)

	// CreateDeck scaffolds a new deck under the workspace and returns the
	// opened Deck. The name becomes the deck's directory (or canonical
	// identifier in future backends). Returns ErrInvalidDeckName if the
	// name contains disallowed characters.
	CreateDeck(name, title, theme string, slides int) (*core.Deck, error)

	// AvailableThemes returns all theme names visible to this workspace:
	// built-in embedded themes plus any external themes discovered under
	// {workspace-root}/themes/ (subdirectories containing theme.yaml).
	AvailableThemes() []string

	// ExternalThemeFS returns the fs.FS for an external theme by name,
	// or nil if the theme is not an external theme.
	ExternalThemeFS(name string) fs.FS
}

// DeckRef identifies a deck within a workspace. Kept small for now; future
// fields (canonical URI, root name, shared-from identity) are added when
// the features that need them ship.
type DeckRef struct {
	// Name is the workspace-local identifier for the deck. For
	// LocalWorkspace this is the subdirectory name, or "." for a deck
	// located at the workspace root itself.
	Name string
}

// ErrDeckNotFound is returned by Workspace.OpenDeck when the deck does not
// exist or the caller cannot see it. Callers should use errors.Is to check
// for this error rather than string matching.
var ErrDeckNotFound = errors.New("deck not found")

// ErrInvalidDeckName is returned when a deck name contains a path separator
// or other disallowed character. Enforced early so future multi-root
// implementations can reliably use "/" as a root/deck separator without
// ambiguity.
var ErrInvalidDeckName = errors.New("invalid deck name")

// validateDeckName rejects names that would escape the workspace, collide
// with the multi-root separator, or otherwise confuse the resolver. The "."
// sentinel (meaning "the deck at the workspace root") is allowed.
func validateDeckName(name string) error {
	if name == "." || name == "" {
		return nil
	}
	if strings.ContainsRune(name, '/') || strings.ContainsRune(name, '\\') {
		return fmt.Errorf("%w: %q contains a path separator — deck names are simple identifiers, not file paths. Call list_decks to discover available decks", ErrInvalidDeckName, name)
	}
	if strings.HasPrefix(name, "..") || strings.HasPrefix(name, ".") {
		return fmt.Errorf("%w: %q begins with a dot", ErrInvalidDeckName, name)
	}
	return nil
}

// LocalWorkspace is a single-root workspace backed by the local filesystem
// via templar.LocalFS (indirectly through core.OpenDeckDir). All decks live
// as subdirectories under the workspace root; an index.html at the root
// itself is exposed as the "." deck.
type LocalWorkspace struct {
	root string
}

// NewLocalWorkspace constructs a LocalWorkspace rooted at the given path.
// The path is resolved to an absolute path immediately; relative paths are
// resolved against the current working directory at construction time.
func NewLocalWorkspace(root string) (*LocalWorkspace, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace root %q: %w", root, err)
	}
	return &LocalWorkspace{root: abs}, nil
}

// Root returns the absolute root path of this workspace. Used by the MCP
// landing page and stdio config printer to display the deck discovery root
// to humans; tool and resource handlers should not call this.
func (w *LocalWorkspace) Root() string { return w.root }

// OpenDeck resolves a deck name to a *core.Deck via core.OpenDeckDir.
// Errors from the underlying filesystem are mapped to ErrDeckNotFound so
// callers can treat "missing" and "unauthorized" uniformly.
func (w *LocalWorkspace) OpenDeck(name string) (*core.Deck, error) {
	if err := validateDeckName(name); err != nil {
		return nil, err
	}
	var dir string
	if name == "." || name == "" {
		dir = w.root
	} else {
		dir = filepath.Join(w.root, name)
	}
	d, err := core.OpenDeckDir(dir)
	if err != nil {
		return nil, fmt.Errorf("%w: %q — call list_decks to discover available decks", ErrDeckNotFound, name)
	}
	return d, nil
}

// ListDecks returns a DeckRef for each subdirectory of the workspace root
// that contains an index.html, plus a "." entry if the root itself is a
// deck. The scan is shallow — nested deck directories are not discovered.
func (w *LocalWorkspace) ListDecks() ([]DeckRef, error) {
	var refs []DeckRef
	if _, err := os.Stat(filepath.Join(w.root, "index.html")); err == nil {
		refs = append(refs, DeckRef{Name: "."})
	}
	entries, err := os.ReadDir(w.root)
	if err != nil {
		// ReadDir failing is not fatal — return what we have. An empty
		// workspace directory returns zero refs.
		return refs, nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(w.root, e.Name(), "index.html")); err == nil {
			refs = append(refs, DeckRef{Name: e.Name()})
		}
	}
	return refs, nil
}

// AvailableThemes returns built-in theme names plus any external themes
// discovered under {root}/themes/. An external theme is a subdirectory
// containing a theme.yaml file.
func (w *LocalWorkspace) AvailableThemes() []string {
	builtin := core.AvailableThemeNames()
	external := w.externalThemeNames()
	if len(external) == 0 {
		return builtin
	}
	// Merge, dedup (external overrides built-in with same name), preserve order.
	seen := make(map[string]bool, len(builtin)+len(external))
	var merged []string
	for _, name := range builtin {
		seen[name] = true
		merged = append(merged, name)
	}
	for _, name := range external {
		if !seen[name] {
			merged = append(merged, name)
		}
	}
	return merged
}

// ExternalThemeFS returns an os.DirFS for the named external theme, or nil
// if no such theme exists under {root}/themes/{name}/theme.yaml.
func (w *LocalWorkspace) ExternalThemeFS(name string) fs.FS {
	dir := filepath.Join(w.root, "themes", name)
	if _, err := os.Stat(filepath.Join(dir, "theme.yaml")); err != nil {
		return nil
	}
	return os.DirFS(dir)
}

// externalThemeNames scans {root}/themes/ for subdirectories containing
// theme.yaml and returns their names sorted alphabetically.
func (w *LocalWorkspace) externalThemeNames() []string {
	themesDir := filepath.Join(w.root, "themes")
	entries, err := os.ReadDir(themesDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(themesDir, e.Name(), "theme.yaml")); err == nil {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// CreateDeck scaffolds a new deck at {root}/{name} via core.CreateInDir and
// returns the opened Deck. Rejects names containing path separators so the
// new directory can never escape the workspace root.
func (w *LocalWorkspace) CreateDeck(name, title, theme string, slides int) (*core.Deck, error) {
	if err := validateDeckName(name); err != nil {
		return nil, err
	}
	if name == "." || name == "" {
		return nil, fmt.Errorf("%w: cannot create deck at workspace root", ErrInvalidDeckName)
	}
	outDir := filepath.Join(w.root, name)

	// Check if this is an external theme — if so, use ScaffoldFromThemeDir
	// which loads templates from the external theme's FS.
	if themeFS := w.ExternalThemeFS(theme); themeFS != nil {
		if _, err := core.CreateInDirWithThemeFS(title, slides, themeFS, outDir); err != nil {
			return nil, err
		}
		return w.OpenDeck(name)
	}

	if _, err := core.CreateInDir(title, slides, theme, outDir, true); err != nil {
		return nil, err
	}
	return w.OpenDeck(name)
}

// --- Workspace context plumbing ---

// workspaceKeyType is an unexported type used as the context key for the
// active Workspace. Using an unexported type prevents collisions with keys
// set by other packages.
type workspaceKeyType struct{}

// workspaceKey is the context key under which the request-scoped Workspace
// is stored. Installed by workspaceMiddleware and read by handlers via
// workspaceFromContext.
var workspaceKey = workspaceKeyType{}

// workspaceFromContext returns the Workspace previously installed on ctx
// by workspaceMiddleware. Returns nil if no workspace has been installed
// (which should never happen in a properly configured server — handlers
// should treat nil as an internal error and return it via ErrorResult).
func workspaceFromContext(ctx context.Context) Workspace {
	ws, _ := ctx.Value(workspaceKey).(Workspace)
	return ws
}

// withWorkspace returns a new context with the given Workspace installed.
// Exposed for tests that need to invoke a tool handler directly without
// going through the middleware chain.
func withWorkspace(ctx context.Context, ws Workspace) context.Context {
	return context.WithValue(ctx, workspaceKey, ws)
}

// workspaceMiddleware returns an mcpkit Middleware that installs the given
// Workspace on every request context before the handler runs. For localhost
// the workspace is a single LocalWorkspace constructed at server startup;
// a future HostedWorkspace variant would resolve a per-request workspace
// from authentication instead of capturing a constant.
func workspaceMiddleware(ws Workspace) server.Middleware {
	return func(ctx context.Context, req *mcpcore.Request, next server.MiddlewareFunc) *mcpcore.Response {
		ctx = withWorkspace(ctx, ws)
		return next(ctx, req)
	}
}

// requireWorkspace is a small helper for tool handlers: fetches the
// Workspace from context and returns a structured error result if it is
// missing. Centralizes the nil check so every handler doesn't repeat it.
func requireWorkspace(ctx context.Context) (Workspace, *mcpcore.ToolResult) {
	ws := workspaceFromContext(ctx)
	if ws == nil {
		r := mcpcore.ErrorResult("internal: no workspace on context")
		return nil, &r
	}
	return ws, nil
}

// openDeckFromContext resolves the workspace from ctx and opens the named
// deck in one call. Used by tool handlers that need a Deck immediately —
// collapses the "require workspace + open deck" idiom into a single line.
// Returns a non-nil *mcpcore.ToolResult (to be returned from the handler)
// when the workspace is missing or the deck cannot be opened.
func openDeckFromContext(ctx context.Context, name string) (*core.Deck, *mcpcore.ToolResult) {
	ws, errResult := requireWorkspace(ctx)
	if errResult != nil {
		return nil, errResult
	}
	d, err := ws.OpenDeck(name)
	if err != nil {
		r := mcpcore.ErrorResult(err.Error())
		return nil, &r
	}
	return d, nil
}
