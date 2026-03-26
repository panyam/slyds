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
- [ ] Slide navigation hooks — drive demos from slide transitions (issue #1)
- [ ] Interactive slides with TypeScript/esbuild support (issue #3)
- [x] ~~Theme static asset copying (images, fonts) during scaffold~~
- [x] ~~`slyds update` command — refresh engine/theme files without touching slides~~
- [x] ~~`.slyds.yaml` manifest for tracking theme and title~~
- [x] ~~Bottom navigation bar with border layout (Prev | counter | Next + Notes icon)~~
- [ ] Slide folders with co-located assets (e.g., `slides/03-architecture/slide.html` + `diagram.png`)
- [ ] Live reload on file changes during `slyds serve`
- [ ] Theme composability via templar `extend`/`namespace` directives

## Polish
- [x] ~~Better error messages — centralized `findRootIn` with actionable hints~~
- [x] ~~`slyds version` command + build-time version injection from git tags~~
- [x] ~~Release automation / goreleaser setup — tag `v*` triggers cross-platform binary release~~
- [x] Publish module path (`github.com/panyam/slyds`)
