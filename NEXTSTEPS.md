# Next Steps

## Cleanup
- [ ] Remove legacy Node.js code (`bin/`, `lib/`, `templates/`, `package.json`, `package-lock.json`, `node_modules/`)
- [ ] Update `.gitignore` to remove `node_modules/`

## Features
- [x] ~~Additional themes beyond "default"~~ — added minimal, dark, corporate, hacker
- [x] ~~`--theme` flag on `slyds init` to select theme~~
- [x] ~~Layout slide types (two-column, section)~~
- [x] ~~Position-aware CSS (--slide-index, --slide-progress custom properties)~~
- [x] ~~theme.yaml config for slide type → template mapping~~
- [ ] Slide navigation hooks — drive demos from slide transitions (issue #1)
- [ ] Interactive slides with TypeScript/esbuild support (issue #3)
- [ ] Slide folders with co-located assets (e.g., `slides/03-architecture/slide.html` + `diagram.png`)
- [ ] Live reload on file changes during `slyds serve`
- [ ] Theme composability via templar `extend`/`namespace` directives

## Polish
- [ ] Better error messages (e.g., when running commands outside a presentation directory)
- [ ] `slyds version` command
- [ ] Release automation / goreleaser setup
- [ ] Publish module path (currently `github.com/user/slyds` — placeholder)
