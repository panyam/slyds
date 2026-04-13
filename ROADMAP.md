# Roadmap

## Phase 1 — Go Rewrite (done)
Core CLI rewritten in Go with templar integration. Multi-file slide authoring with `{{# include #}}` composition. All commands working: init, serve, build, add/rm/mv/ls.

## Phase 2 — Cleanup & Polish (done)
Removed legacy Node.js code. Fixed module path. Version command + build-time injection from git tags. Better error messages. Goreleaser for cross-platform binary releases. GitHub Actions CI.

## Phase 3 — Theme System (done)
Multiple built-in themes (default, minimal, dark, corporate, hacker). `--theme` flag on init. Theme preview/switching. Theme-aware slide rendering from manifest.

## Phase 4 — Slide Management (done)
`slyds insert` with auto-renumber. Index-based ordering (index.html as source of truth). Robust non-prefixed filename handling. `slyds slugify` for diff-friendly filenames.

## Phase 5 — Content Tooling (done)
`slyds check` for deck validation. `slyds query` for CSS selector-based slide content access via goquery. No-regex-HTML-mutation constraint established.

## Phase 6 — Export & Sharing (done)
Client-side slide export/download from built presentations. Download button in nav bar extracts slides from DOM, wraps in standalone HTML, zips, and triggers browser download. Works from `file://`, static hosts, and `slyds serve` — no server required. Shared template system for `index.html.tmpl` to reduce cross-theme duplication.

## Phase 6a — Examples & Documentation (done)
Three example presentations (slyds-intro, rich-content, hacker-showcase) demonstrating themes and CSS components. GitHub Pages deployment via `make gh-pages`. `make examples` build target.

## Phase 6b — Presenter Timer (done)
Elapsed presentation timer, per-slide reading time (~200 WPM), and remaining deck time in the speaker notes window. Toggle with T key. Timer state persists across notes window close/reopen.

## Phase 7 — Layout/Theme Separation (done)
Layouts (structural templates) separated from themes (visual skins). `--layout` flag on `slyds add`/`insert` selects from 6 built-in layouts: title, content, two-col, section, blank, closing. `data-layout` attribute on slides enables machine-parseable structure. CSS variable-based theming with runtime theme switcher. `slyds ls` shows layout per slide. `slyds check` validates layout attributes. Module system via templar SourceLoader for 3P themes/layouts.

## Phase 8 — Slide Lifecycle Hooks (done)
Client-side `slideEnter`/`slideLeave` CustomEvents dispatched during navigation. `window.slydsContext` persistent presentation state with a `state` bag for caching chart instances and cross-slide data. AGENT.md documents the recommended cache-friendly hook pattern for charts and dynamic content. AGENT.md generation refactored from hardcoded Go strings to an embedded `.tmpl` template.

## Phase 8a — Floating Overlays (done)
Generic `data-slot="floater"` with `.slide-floater` CSS for pinned overlays (footers, watermarks, logos, badges). Available in content, two-col, and closing layouts. Empty by default — populated via `slyds query`. Documented in AGENT.md with common patterns.

## Phase 8b — Agent tooling & MCP (done)
`slyds introspect` emits JSON for layouts, slots, themes, and command catalog. `slyds query --batch` applies multiple writes atomically. `add`/`insert` accept `--slots-file` (slot name → HTML fragment). MCP: **`slyds mcp`** (stdio) and **`slyds mcp serve`** (HTTP+SSE per MCP 2024-11-05) as a thin CLI wrapper. See `docs/MCP.md` and `docs/AGENT-THEMES.md`.

## Phase 9 — MCP migration to mcpkit (done)
Migrated hand-rolled MCP transport to mcpkit v0.0.6. Streamable HTTP default, SSE via `--sse` flag. Constant-time bearer auth, graceful shutdown via servicekit.

## Phase 9a — WritableFS abstraction (done)
Migrated all core/ production code to use `templar.WritableFS`. Zero `os.*`/`filepath.*` in core/ except `osfs.go` (OS boundary). templar upgraded to v0.1.0 with breaking FSFolder API. modules.go, manifest.go, scaffold.go, builder.go, inline.go all FS-based. examples_test.go migrated to use Deck API.

## Phase 9b — MCP Resources + Semantic Tools (done)
Replaced subprocess-based MCP tool with 10 semantic tools calling the Deck API directly (no subprocess, structured JSON returns). Added 7 browsable MCP resources for deck/slide content discovery. Extracted `assets/` package separating Go code from static files. Migrated e2e tests to mcpkit/testutil.TestClient. mcpkit upgraded to v0.0.7 (Go MCP client with Streamable HTTP + SSE transports).

## Phase 9c — mcpkit split-package upgrade + stdio (done)
Upgraded mcpkit from v0.0.7 (flat package) to v0.1.5 (split packages: `core/`, `server/`, `client/`). Enabled stdio transport (`--stdio` flag) for editor-spawned MCP servers (Cursor, Claude Desktop, VS Code). Added stdio E2E test. Updated agent setup docs with stdio configuration examples.

## Phase 9d — End-to-end setup (done)
Added `--json` flags to `check`, `ls`, `build` for CLI-direct agent mode. `SLYDS_MCP_TOKEN` env var fallback for container deployments. Makefile targets for demo decks (`make demo`) and dev servers (`make dev-http`, `dev-sse`, `dev-stdio`). Tunnel helper script for remote access. Agent-readable setup guide (`AGENT-SETUP.md`) and human-readable setup guide (`docs/SETUP.md`).

## Phase 9e — Agent skills scaffolding (done)
`slyds init` scaffolds `.claude/skills/` with 5 skills: `/preview` (build + open), `/check` (validate deck), `/slides` (list slides), `/build` (build HTML), `/add-slide` (guided slide insertion). Skills are static SKILL.md files embedded in `assets/skills/` and copied during scaffold. `slyds update` refreshes skills alongside engine files.

## Phase 9f — MCP Apps / inline previews (done)
Added MCP Apps extension (`io.modelcontextprotocol/ui`) with two preview tools: `preview_deck` (full navigable presentation via `d.Build()`) and `preview_slide` (single slide with theme CSS via embedded template). LLM hosts that support apps render slides inline as iframes. Non-UI clients get text summaries. Mutation tools (`edit_slide`, `add_slide`, `remove_slide`, `create_deck`) now send `notifications/resources/list_changed`. Added `github.com/panyam/mcpkit/ext/ui` dependency.

## Phase 9g — mcpkit v0.1.15 adoption (done)
Adopted mcpkit v0.1.15 features: single-struct registration (`srv.Register` with `server.Tool`/`server.Resource`/`server.ResourceTemplate`), per-tool timeouts (`build_deck` 30s, `check_deck` 10s), `StructuredResult` for typed tool output, `ToolCallTyped` in E2E tests, `ErrorHandler` for session lifecycle logging, `EventStore` for Streamable HTTP reconnection. Typed result structs replace `map[string]any` in tool handlers. No behavioral changes — purely internal improvements.

## Phase 9h — Workspace abstraction (done)
Introduced a `Workspace` interface (`cmd/workspace.go`) to decouple MCP tool and resource handlers from raw `--deck-root` filesystem paths. The `LocalWorkspace` implementation is installed on every MCP request via `workspaceMiddleware`; handlers resolve decks via `workspaceFromContext(ctx).OpenDeck(name)`. New `slyds ws` CLI subcommand (`ws info`, `ws list`) exercises the same workspace path as the MCP server, and `make demo-smoke` drives both through a single end-to-end test. This is the prep refactor for hosted multi-tenancy (#76), optimistic concurrency, and slug-as-ID slide identity — all tracked as follow-ups under #74. No behavioral changes for existing agents or CLI users.

## Phase 9i — Slug-as-ID in MCP tools (done)
Made slug (the non-prefix portion of `NN-slug.html`) the stable handle for slide references in the MCP API. `SlideDescription` grew a `Slug` field returned by `describe_deck` / `list_slides`. `ResolveSlide` rewritten with a priority chain (numeric → exact filename → exact slug → substring fallback) and new `ErrAmbiguousSlideRef` for multi-match cases. `InsertSlide` auto-suffixes colliding slugs via the same `-N` pattern `SlugifySlides` already uses, and now returns `(finalSlug, error)` so callers can surface the actual name. Scaffolder fixed so `slyds init --slides 5` produces unique slugs by default (`slide`, `slide-2`, `slide-3`). MCP `read_slide` and `edit_slide` accept a new `slide` string parameter (slug, filename, or position) alongside the existing `position` int for backward compat. `TestE2E_SlugRefSurvivesInsert` is the canonical integration test proving slug stability across position shifts.

## Phase 9j — Persistent slide IDs (done)
Added a rename-safe `slide_id` per slide, stored as `slides: [{id, file}]` records in `.slyds.yaml`. IDs are `sl_` + 8 hex chars, generated at slide creation time via `crypto/rand`, and survive every mutation including inserts, removes, moves, and slugify renames. `Deck.Manifest` unified from the minimal `DeckManifest` to the full `Manifest` type. `writeManifestFS` unified from a hand-formatted string to the exported `yaml.Marshal` path. `ResolveSlide` gains a priority-0 branch for `sl_`-prefixed references. `SlideDescription` gains a `SlideID` field populated by `Describe()`. Legacy decks auto-migrate on first mutation. Scaffolder assigns IDs at creation time. Canonical test: `TestE2E_SlideIDSurvivesRename`.

## Phase 9k — NamingScheme abstraction (done)
Introduced a `NamingScheme` interface to decouple slide filename generation from the hardcoded `NN-slug.html` format. Two implementations: `NumberedScheme` (default, current behavior) and `SlugOnlyScheme` (no numeric prefix, no renames on reorder). Configured per-deck via `filename_style` in `.slyds.yaml`. `RewriteSlideOrder` skips the entire two-pass rename loop in slug-only mode — only rewrites `index.html`. All 7+1 hardcoded `%02d-` format strings replaced with `scheme.Format()`. `SlideFilenames()` gains a manifest-based fallback before alphabetical sort. Scaffolder supports `ScaffoldOpts.FilenameStyle`.

## Phase 9l — MCP Apps display modes + template resources (done)
Adopted mcpkit v0.1.31 MCP Apps extensions: `supportedDisplayModes` (inline, fullscreen) on preview tools, `RequestDisplayMode` for presentation mode, template resource URIs (`ui://slyds/decks/{deck}/preview`, `ui://slyds/decks/{deck}/slides/{position}/preview`) eliminating mutable preview state, `NotifyResourceUpdated` for targeted resource change notifications. App elicitation deferred to follow-up.

## Phase 10 — Slide Folders
Support `slides/03-name/slide.html` with co-located assets (images, per-slide CSS). Auto-detect folder vs file slides.

## Future
- Structured slide formats (YAML, JSON, MD) with format-aware query dispatch
- Decouple slyds.js/css into independently publishable npm package (issue #12)
- Slide animations — PowerPoint-style entry/exit/emphasis (issue #5)
- Interactive slides with TypeScript/esbuild (issue #3)
- Slide navigation hooks — server-side execution + declarative config (issue #1, steps 2-5)
- Markdown slide authoring (convert `.md` to slide HTML)
- WASM-based browser editor and source-level rebuild (issue #21)
- PDF export
- Plugin system for custom slide components
