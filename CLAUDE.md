# CLAUDE.md — slyds

## What is this?

slyds is a Go CLI + library for multi-file HTML presentations. Each slide lives in its own file; `index.html` composes them via templar's `{{# include #}}` directives. `slyds build` flattens everything into a single self-contained HTML file.

See [ARCHITECTURE.md](ARCHITECTURE.md) for design details.

## Build & Test

```bash
make resymlink   # Set up locallinks/ for local templar dependency (first time only)
make build       # Build the slyds binary (injects version from git tags)
make test        # Run all tests
make install     # Install to $GOBIN
make audit       # govulncheck + gosec + gitleaks
```

## Key Commands

```bash
slyds init "Title" [-n count] [--theme dark]      # Scaffold presentation
slyds serve [dir] [-p port]                       # Dev server with live include resolution
slyds build [dir]                                # Flatten to dist/index.html
slyds add "name" [--after N] [--layout content] [--slots-file map.json]
slyds insert <pos> "name" [--layout T] [--title T]
slyds rm <name-or-number>                        # Remove slide
slyds mv <from> <to>                             # Reorder slides
slyds ls [dir]                                   # List slides
slyds slugify [dir]                              # Rename slides from <h1>
slyds check [dir]                                # Validate deck
slyds describe [dir] [--json]                    # Structured summary
slyds query <slide> <sel> [--set|--html|...]     # CSS selector read/write
slyds mcp [--sse] [--listen :6274] [--token T]   # MCP server (Streamable HTTP default)
slyds preview <theme-dir> [-p 3000]              # Preview a theme
```

### MCP Server

Uses **mcpkit** v0.0.6. Default: Streamable HTTP. Use `--sse` for legacy SSE transport.

```bash
slyds mcp                          # Streamable HTTP on :6274
slyds mcp --sse                    # Legacy SSE transport
STREAMABLE=1 slyds mcp             # Also Streamable HTTP
```

Setup docs: [docs/MCP.md](docs/MCP.md)

## Architecture: Deck is the API

All business logic lives in `core/` as methods on `*Deck`. CLI commands are thin wrappers.

```
core/                        ← THE library
├── deck.go                  Deck type, WritableFS, slide CRUD, InsertSlide, ApplySlots
├── theme.go                 Theme type backed by fs.FS, RenderSlide
├── check.go                 Deck.Check() → Issues
├── describe.go              Deck.Describe() → DeckDescription
├── query.go                 Deck.Query(), BatchQuery(), ResolveSlide
├── scaffold.go              Create, Update, Slugify
├── manifest.go              .slyds.yaml read/write
├── layout.go                Layout templates (title, content, two-col, etc.)
├── builder.go               Build pipeline (include resolution + asset inlining)
├── embed.go                 Embedded CSS, JS, themes, layouts
├── theme.go                 Theme loading from any fs.FS
└── 130+ tests via MemFS

cmd/                         ← thin CLI wiring (~2100 lines)
├── slides.go                add/rm/mv/ls/insert/slugify
├── query.go                 CSS selector read/write
├── check.go                 validate deck
├── describe.go              structured summary
├── mcp.go                   MCP server (mcpkit)
└── tests (CLI-specific only)
```

### Key types

```go
d, _ := core.OpenDeckDir(".")       // Open from local path
d, _ := core.OpenDeck(memFS)        // Open from any templar.WritableFS

d.SlideFilenames()                   // List slides
d.GetSlideContent(1)                 // Read slide HTML
d.EditSlideContent(1, html)          // Write slide HTML
d.InsertSlide(2, "name", "layout", "title")  // Render + insert
d.RemoveSlide("02-name.html")       // Delete + renumber
d.MoveSlide(3, 1)                   // Reorder + renumber
d.ApplySlots(2, map[string]string{"body": "<p>...</p>"})
d.Query("1", "h1", core.QueryOpts{})  // CSS selector query
d.Check()                            // Validate → Issues
d.Describe()                         // Structured summary
d.SlugifySlides(core.Slugify)        // Rename from headings
d.Build()                            // Self-contained HTML

theme, _ := core.LoadTheme(fs)       // Load from any fs.FS
theme, _ := core.LoadEmbeddedTheme("dark")
theme.RenderSlide("content", data)   // Render slide template
```

### Filesystem abstraction

All Deck I/O goes through `templar.WritableFS`. No `os.*` in core/.

- `templar.LocalFS` — local disk (CLI)
- `templar.MemFS` — in-memory (tests, WASM)
- Future: S3, IndexedDB

## Conventions

- **Deck is the single API** — cmd/ never touches DeckFS directly, only calls Deck methods
- **No hardcoded HTML in Go** — use embedded `.tmpl` files under `core/templates/`
- **Layouts are theme-independent** — `core/layouts/` (title, content, two-col, section, blank, closing)
- **Themes are fs.FS** — `core.LoadTheme(fs)` works with local, embedded, or remote
- **Index.html is source of truth** for slide ordering
- **No regex HTML mutation** — use `d.Query()` (goquery/CSS selectors)
- **Version from git tags** — `make build`/`make install` inject via ldflags
- **Local deps via locallinks/** — `replace => ./locallinks/...` in go.mod; must be commented out before push

## Gotchas

- **macOS /private symlinks**: `filepath.Abs` on temp dirs returns `/var/...` but actual path is `/private/var/...`. Don't compare paths in tests.
- **`go:embed` paths relative to Go file** — `core/embed.go` must live alongside embedded files.
- **MCP uses mcpkit** — no more hand-rolled protocol code. Bearer token uses constant-time comparison. Graceful shutdown via servicekit.
- **Theme render fallback** — `InsertSlide` uses layout system first; falls back to theme templates for backward compat.

## Memories

Stored in-repo under `memories/`. See [memories/MEMORY.md](memories/MEMORY.md).

<!-- stack-brain:start -->
## Constraints

### No Regex-Based HTML Mutation
All HTML reads/writes must use `d.Query()` (goquery/CSS selectors), not regex.
*Why: Regex HTML manipulation breaks on nested tags, attributes, whitespace.*
<!-- stack-brain:end -->
