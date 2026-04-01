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

## Phase 9 — Slide Folders
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
