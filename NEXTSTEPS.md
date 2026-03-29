# Next Steps

## Features
- [x] ~~Additional themes beyond "default"~~ — added minimal, dark, corporate, hacker
- [x] ~~`--theme` flag on `slyds init` to select theme~~
- [x] ~~Layout slide types (two-column, section)~~
- [x] ~~Position-aware CSS (--slide-index, --slide-progress custom properties)~~
- [x] ~~theme.yaml config for slide type → template mapping~~
- [x] ~~`slyds insert` command — insert slide at position with auto-renumber (issue #6)~~
- [x] ~~Index-based slide ordering — `index.html` is source of truth, not filesystem sort~~
- [x] ~~Robust non-prefixed filename handling — files without `NN-` prefix are preserved during renumber~~
- [x] ~~Theme-aware slide rendering — `add`/`insert` use manifest theme, not hardcoded "default"~~
- [x] ~~GitHub Actions CI + pre-push hook running tests~~
- [x] ~~`slyds slugify` — bulk rename slides to slug-based filenames from `<h1>` (issue #10)~~
- [x] ~~`slyds check` — validate deck sync, speaker notes, broken assets, talk time (issue #9 partial)~~
- [x] ~~`slyds query` — CSS selector read/write interface for slide HTML via goquery (issue #18)~~
- [x] ~~Client-side slide export/download from built presentations (issue #20)~~
- [x] ~~Shared `index.html.tmpl` with theme-specific override support — reduces cross-theme duplication~~
- [x] ~~Example presentations — 3 demo decks (intro, rich-content, hacker) with GitHub Pages deployment~~
- [x] ~~Presenter timer + reading time in speaker notes window (issue #22 phase 1)~~
- [x] ~~Layout/theme separation — `--layout` flag, data-layout attribute, 6 built-in layouts (issue #30)~~
- [x] ~~Runtime theme switching — theme switcher in toolbar, CSS variable-based theming (issue #29)~~
- [x] ~~Templar module system integration — SourceLoader, .slyds.lock, slyds install (issue #31)~~
- [ ] Slide navigation hooks — drive demos from slide transitions (issue #1)
- [ ] Interactive slides with TypeScript/esbuild support (issue #3)
- [x] ~~Theme static asset copying (images, fonts) during scaffold~~
- [x] ~~`slyds update` command — refresh engine/theme files without touching slides~~
- [x] ~~`.slyds.yaml` manifest for tracking theme and title~~
- [x] ~~Bottom navigation bar with border layout (Prev | counter | Next + Notes icon)~~
- [ ] Slide folders with co-located assets (e.g., `slides/03-architecture/slide.html` + `diagram.png`)
- [ ] Live reload on file changes during `slyds serve`
- [ ] Theme composability via templar `extend`/`namespace` directives
- [ ] WASM-based browser editor and source-level rebuild (issue #21)
- [ ] Decouple slyds.js/css/export.js into npm package (issue #12)

## Polish
- [x] ~~Better error messages — centralized `findRootIn` with actionable hints~~
- [x] ~~`slyds version` command + build-time version injection from git tags~~
- [x] ~~Release automation / goreleaser setup — tag `v*` triggers cross-platform binary release~~
- [x] Publish module path (`github.com/panyam/slyds`)
