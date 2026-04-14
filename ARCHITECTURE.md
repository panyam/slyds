# Architecture

## Overview

slyds is a Go binary that imports [templar](https://github.com/panyam/templar) as a library for template composition and serving. It uses Cobra for the CLI (same stack as templar).

```
slyds (Go binary)
  ├── imports templar for template include resolution + HTML rendering
  ├── uses Cobra for CLI
  └── embeds default assets (CSS, JS, theme templates) via go:embed
```

## Multi-File Slide Authoring

Presentations use one file per slide, composed via templar includes:

```
my-talk/
  .slyds.yaml             # Manifest (theme, title) — used by slyds update
  index.html              # Organizer — templar {{# include #}} directives
  slyds.css               # Presentation engine styles (copied from embedded)
  slyds.js                # Client-side slide engine (copied from embedded)
  slyds-export.js         # Client-side export/download (copied from embedded)
  theme.css               # Theme styles (rendered from theme template)
  images/                 # Optional theme assets (copied from theme)
  slides/
    01-title.html          # Pure HTML slide files
    02-slide.html
    03-closing.html
```

### Key Design: Slides are pure HTML

Only `index.html` uses templar's `{{# include "slides/01-title.html" #}}` syntax. Individual slide files have zero template awareness — they're just `<div class="slide">` fragments.

## Theme System

Themes are sets of `.tmpl` files under `core/templates/<theme>/`, plus optional static assets (images, fonts, etc.):

```
core/templates/
  index.html.tmpl                  # Shared Go text/template for index.html (used by all themes unless overridden)
  default/
    theme.css.tmpl                 # Go text/template for theme.css
    theme.yaml                     # Slide type → template mapping
    slides/title.html.tmpl         # Slide type templates
    slides/content.html.tmpl
    slides/closing.html.tmpl
    images/                        # Optional — copied verbatim during scaffold
  hacker/
    theme.css.tmpl
    slides/                        # Theme-specific slide templates
    images/                        # Theme static assets
    ...
```

Templates receive `{{.Title}}`, `{{.Number}}`, `{{.Includes}}` etc. Adding a new theme means adding a new directory with theme-specific files. Common templates like `index.html.tmpl` live at the shared level and are inherited unless a theme provides its own override. Non-template static files (images, fonts) are copied as-is during `slyds init`.

## Layout System

Layouts define the structural arrangement of slide content, independent of visual themes. Six built-in layouts live in `core/layouts/`:

| Layout | Description | Slots |
|--------|-------------|-------|
| `title` | Full-screen title with subtitle | title, subtitle |
| `content` | Standard heading + body | title, body |
| `two-col` | Two-column side-by-side | title, left, right |
| `section` | Section divider | title, subtitle |
| `blank` | Empty slide | body |
| `closing` | Closing/thank-you | title, body |

Each layout template sets a `data-layout` attribute on the slide div and uses `data-slot` attributes for named content regions. `slyds add --layout two-col "Name"` scaffolds a slide from the layout template. `slyds ls` shows the layout per slide.

Layouts use CSS classes (`.layout-two-col`, `.title-slide`, etc.) for structural styling, referencing `--slyds-*` variables so any theme can skin any layout. The layout registry lives in `core/layouts/layouts.yaml`.

### Presentation Layout

The presentation uses a border layout (flexbox column): slide content fills the center and a navigation bar is pinned to the bottom. The nav bar contains Prev/Next buttons with a slide counter between them, and icon buttons for theme switching, export (download), and speaker notes on the far right.

## Manifest & Update

Each scaffolded presentation gets a `.slyds.yaml` manifest:

```yaml
theme: dark
title: "My Presentation"
sources:
  core:
    url: github.com/panyam/slyds
    path: core
```

`slyds update` reads this manifest and refreshes engine files (`slyds.css`, `slyds.js`, `slyds-export.js`, `theme.css`, `index.html` layout, theme images) from the latest embedded assets — without touching `slides/`. It parses existing `{{# include #}}` directives from `index.html` to preserve slide ordering.

If `.slyds.yaml` is missing (pre-existing presentations), `update` prompts for theme and title interactively.

## Serve vs Build

**`slyds serve`**: Custom HTTP handler that routes `.html` requests through templar's `TemplateGroup.RenderHtmlTemplate()` (resolves includes on-the-fly) and serves everything else as static files.

**`slyds build`**: Uses templar's `TemplateGroup` to flatten all includes, then inlines CSS (`<link>` → `<style>`), JS (`<script src>` → `<script>`), and images (local files → base64 data URIs). Output is a single `dist/index.html` that works from `file://`.

## Templar Integration

slyds uses templar as a Go library, configuring it programmatically:

- `templar.NewTemplateGroup()` — creates template manager
- `templar.NewFileSystemLoader(root)` — loads templates from presentation directory
- `group.RenderHtmlTemplate()` — renders with include resolution

No `.templar.yaml` config files are generated or needed.

## Slide Ordering

`index.html` is the **source of truth** for slide ordering. All commands (`ls`, `rm`, `mv`, `add`, `insert`) use `listSlidesFromIndex()` which parses include directives from `index.html`. The filesystem sort (`listSlideFiles`) is a fallback only when index.html has no includes.

`rewriteSlidesAndIndex()` is the core mutation function — it renames slide files (via temp files to avoid collisions) and rebuilds all include directives atomically.

## Content Access Layer

`slyds query` provides CSS selector-based read/write access to slide HTML content using `PuerkitoBio/goquery`. Slide files are HTML fragments, not full documents — the query layer wraps them in a sentinel div for parsing and extracts the fragment on write-back (no `<html><body>` wrappers leak).

This is the approved path for all programmatic slide content access. Regex-based HTML mutation is prohibited (see CONSTRAINTS.md).

**Batch writes** (`slyds query --batch`): same goquery path; JSON lists operations per slide. With default `--atomic`, all mutations apply in memory and all affected slide files are written only if every step succeeds.

## MCP and agent-facing surfaces

The CLI exposes machine-readable **`slyds introspect`** (layouts, themes, command catalog), per-deck **`slyds describe`**, and **`slyds ws info` / `slyds ws list`** for inspecting the MCP workspace without starting a server.

### Workspace layer

MCP tool and resource handlers never touch raw filesystem paths. They resolve decks through a `Workspace` interface (`cmd/workspace.go`). The `LocalWorkspace` implementation is a thin wrapper around `core.OpenDeckDir` rooted at `--deck-root`. A workspace middleware installs the active `Workspace` into every request's `context.Context`, and handlers call `workspaceFromContext(ctx).OpenDeck(name)` to get a `*core.Deck`.

The abstraction exists so that:
- a future hosted deployment can resolve a per-request `Workspace` from an authenticated session instead of a single startup-constructed one, without changing tool/resource handler code
- deck names with path separators (`../escape`, `root/sub`) are rejected at the API boundary, reserving `/` for future multi-root workspace qualification
- unit tests can install a workspace directly via `withWorkspace(ctx, ws)` without going through the full middleware chain

The `slyds ws` CLI subcommand exercises the same `LocalWorkspace` implementation the MCP server uses, making it a single-process smoke test for the wiring.

### Slide identity

Slides have three overlapping identifiers, each stable across a different set of mutations:

| Reference | Format | Stable across inserts? | Stable across renames? |
|-----------|--------|----------------------|----------------------|
| **Position** | `2` | No | Yes |
| **Slug** | `metrics` | Yes | No |
| **Slide ID** | `sl_a1b2c3d4` | Yes | Yes |

**Slug** is the non-prefix portion of `NN-slug.html`. Uniqueness is enforced at every creation path — `InsertSlide` auto-suffixes on collision (`intro` → `intro-2`), and the scaffolder produces unique slugs. `ResolveSlide` accepts slug, filename, or position and returns `ErrAmbiguousSlideRef` on multi-match.

**Slide ID** (`sl_` + 8 hex chars) is an opaque identifier assigned by slyds on slide creation and stored in `.slyds.yaml` as per-slide `{id, file}` records. The ID never changes; only the `file` value updates when a slide is renamed or renumbered. `ResolveSlide` checks slide_id as its highest-priority match (before numeric, filename, slug, and substring).

Mutation methods (`AddSlide`, `InsertSlide`, `RemoveSlide`, `MoveSlide`, `SlugifySlides`, `RewriteSlideOrder`) maintain the id→file mapping atomically: `ensureSlideIDs()` is called before the mutation (auto-migrating legacy decks without IDs), and `saveManifest()` is called after. Read-only operations (`Describe`, `list_slides`) do not trigger auto-migration — legacy decks return empty `slide_id` until their first mutation.

The `DeckManifest` type has been eliminated; `Deck.Manifest` is now the full `*Manifest` type from `core/manifest.go`. The unexported `writeManifestFS` in `scaffold_fs.go` now delegates to the exported `WriteManifestFS` (`yaml.Marshal` path) ensuring all manifest fields are serialized correctly.

**Model Context Protocol** — Two parallel server paths: `slyds mcp` (hand-written, production) and `slyds mcp-proto` (proto-generated, experimental). Both use **mcpkit** v0.2.15 (typed handler contexts, `ToolCallFull`, `NotifyResourceUpdated`) + **ext/ui** v0.2.15 (MCP Apps with auto-fallback template URIs) + **ext/protogen** v0.2.16 (proto→MCP codegen). Exposes **13 tools** (11 core + 2 preview), **9 resources** (7 data + 2 preview), and **completions** for deck names and slide positions. Optimistic versioning via `expected_version` / `expected_deck_version`. `--allow-origin '*'` for tunnel/remote deployments. Transports: Streamable HTTP (default), SSE (`--sse`), stdio (`--stdio`). See [docs/MCP.md](docs/MCP.md).

**Proto MCP** (`proto/slyds/v1/`, `gen/go/slyds/v1/`, `cmd/mcp_proto_impl.go`): The proto definition declares all tools as `mcp_tool` RPCs and resources as `mcp_resource` RPCs with `completable_fields` annotations. `protoc-gen-go-mcp` generates `RegisterSlydsServiceMCP()`, `RegisterSlydsServiceMCPResources()`, and `RegisterSlydsServiceMCPCompletions()`. `SlydsServiceImpl` wraps Workspace with typed proto messages and gRPC status codes (ABORTED for version conflicts). Parity E2E tests validate both paths produce identical output. Dev: `cd proto && make setupdev && make buf`.

**Tools**: `list_decks`, `create_deck`, `describe_deck`, `list_slides`, `read_slide`, `edit_slide`, `query_slide`, `add_slide`, `remove_slide`, `check_deck`, `build_deck`, `preview_deck`, `preview_slide`.

**MCP Apps** (`cmd/mcp_apps.go`): `preview_deck` and `preview_slide` register as app tools via the `io.modelcontextprotocol/ui` extension with `supportedDisplayModes: [inline, fullscreen]`. `preview_deck` accepts an optional `display_mode` parameter; when `"fullscreen"`, it sends `RequestDisplayMode` to the host for presentation mode. Both tools use **template resource URIs** (`ui://slyds/decks/{deck}/preview`, `ui://slyds/decks/{deck}/slides/{position}/preview`) — the deck name and slide position are extracted from URI params, eliminating mutable preview state. Preview HTML is built on demand through the workspace on every resource read — no HTML cache. This ensures authz is always enforced (critical for multi-tenant hosted deployments). Mutation tools (`edit_slide`, `add_slide`, `remove_slide`) send `NotifyResourceUpdated` with the affected deck's preview URI for targeted change propagation.

**Resources**: `slyds://decks`, `slyds://decks/{name}`, `slyds://decks/{name}/slides/{n}`, `slyds://decks/{name}/config`, `slyds://decks/{name}/agent`, `ui://slyds/decks/{deck}/preview`, `ui://slyds/decks/{deck}/slides/{position}/preview`.

Theme/manifest notes for remote agents: [docs/AGENT-THEMES.md](docs/AGENT-THEMES.md).

## Slide Lifecycle Hooks

`slyds.js` dispatches `slideEnter` and `slideLeave` CustomEvents during navigation. These fire on the slide element itself and bubble to `document`.

- `slideLeave` fires on the outgoing slide **before** `.active` is removed (slide still has dimensions)
- `slideEnter` fires on the incoming slide **after** `.active` is added (slide has dimensions)
- Event `detail` includes: `index`, `slideNum`, `title`, `layout`, `total`, `direction`, `data` (all `data-*` attrs)
- `window.slydsContext` provides persistent state: `totalSlides`, `currentSlide`, `direction`, and a `state` bag for user/agent code

The hooks are documented in AGENT.md (auto-generated per deck) so that coding agents use them correctly for Chart.js, D3, and other libraries that need real canvas dimensions.

## Client-Side Export

Built presentations include `slyds-export.js` which provides a download button in the nav bar. When clicked, it:

1. Extracts all `<style>` blocks from the page (already inlined CSS)
2. Adds the full deck as `index.html` to a ZIP
3. Wraps each `<div class="slide">` in standalone HTML with the extracted styles
4. Generates a ZIP using a minimal store-only ZIP writer (no external dependencies)
5. Triggers a browser download via `Blob` URL

This works entirely client-side — no server required, including from `file://` protocol. The ZIP writer is ~120 lines of vanilla JS implementing the ZIP format with store-only compression (no deflate needed for small HTML files).

## Filesystem Abstraction

All core/ production code uses `templar.WritableFS` for I/O. The only file with `os.*`/`filepath.*` is `core/osfs.go`, which provides OS-boundary convenience functions (`Create`, `CreateInDir`, `FindDeckRoot`, etc.) that create a `LocalFS` and delegate to FS-based implementations.

- `templar.WritableFS` — interface: ReadFile, ReadDir, WriteFile, MkdirAll, Remove, Rename
- `templar.LocalFS` — OS disk adapter (CLI usage)
- `templar.MemFS` — in-memory (tests, future WASM)

## Dependency Management

The `go.mod` uses a local replace directive for templar development:
```
replace github.com/panyam/templar => ./locallinks/newstack/templar/main
```

The `locallinks/` directory contains symlinks created by `make resymlink`. The replace directive must be **commented out** before pushing — the pre-push hook enforces this.

## Release

Version is injected from git tags at build time via ldflags (`-X cmd.Version`). Goreleaser builds cross-platform binaries on `v*` tag push via GitHub Actions.
