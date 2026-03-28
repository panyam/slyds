# Roadmap

## v0.1 — Go Rewrite (done)
Core CLI rewritten in Go with templar integration. Multi-file slide authoring with `{{# include #}}` composition. All commands working: init, serve, build, add/rm/mv/ls.

## v0.2 — Cleanup & Polish (done)
Removed legacy Node.js code. Fixed module path. Version command + build-time injection from git tags. Better error messages. Goreleaser for cross-platform binary releases. GitHub Actions CI.

## v0.3 — Theme System (done)
Multiple built-in themes (default, minimal, dark, corporate, hacker). `--theme` flag on init. Theme preview/switching. Theme-aware slide rendering from manifest.

## v0.4 — Slide Management (done)
`slyds insert` with auto-renumber. Index-based ordering (index.html as source of truth). Robust non-prefixed filename handling. `slyds slugify` for diff-friendly filenames.

## v0.5 — Content Tooling (done)
`slyds check` for deck validation. `slyds query` for CSS selector-based slide content access via goquery. No-regex-HTML-mutation constraint established.

## v0.6 — Export & Sharing (done)
Client-side slide export/download from built presentations. Download button in nav bar extracts slides from DOM, wraps in standalone HTML, zips, and triggers browser download. Works from `file://`, static hosts, and `slyds serve` — no server required. Shared template system for `index.html.tmpl` to reduce cross-theme duplication.

## v0.6.1 — Examples & Documentation
Three example presentations (slyds-intro, rich-content, hacker-showcase) demonstrating themes and CSS components. GitHub Pages deployment via `make gh-pages`. `make examples` build target.

## v0.6.2 — Presenter Timer
Elapsed presentation timer, per-slide reading time (~200 WPM), and remaining deck time in the speaker notes window. Toggle with T key. Timer state persists across notes window close/reopen.

## v0.7 — Slide Folders
Support `slides/03-name/slide.html` with co-located assets (images, per-slide CSS). Auto-detect folder vs file slides.

## Future
- Structured slide formats (YAML, JSON, MD) with format-aware query dispatch
- Decouple slyds.js/css into independently publishable npm package (issue #12)
- Slide animations — PowerPoint-style entry/exit/emphasis (issue #5)
- Interactive slides with TypeScript/esbuild (issue #3)
- Slide navigation hooks (issue #1)
- Markdown slide authoring (convert `.md` to slide HTML)
- WASM-based browser editor and source-level rebuild (issue #21)
- PDF export
- Plugin system for custom slide components
