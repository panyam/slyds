# Roadmap

## v0.1 — Go Rewrite (done)
Core CLI rewritten in Go with templar integration. Multi-file slide authoring with `{{# include #}}` composition. All commands working: init, serve, build, add/rm/mv/ls.

## v0.2 — Cleanup & Polish (done)
Removed legacy Node.js code. Fixed module path. Version command + build-time injection from git tags. Better error messages. Goreleaser for cross-platform binary releases. GitHub Actions CI.

## v0.3 — Theme System (done)
Multiple built-in themes (default, minimal, dark, corporate, hacker). `--theme` flag on init. Theme preview/switching. Theme-aware slide rendering from manifest.

## v0.4 — Slide Management (done)
`slyds insert` with auto-renumber. Index-based ordering (index.html as source of truth). Robust non-prefixed filename handling.

## v0.4 — Slide Folders
Support `slides/03-name/slide.html` with co-located assets (images, per-slide CSS). Auto-detect folder vs file slides.

## Future
- Markdown slide authoring (convert `.md` to slide HTML)
- PDF export
- Remote collaboration / sharing
- Plugin system for custom slide components
