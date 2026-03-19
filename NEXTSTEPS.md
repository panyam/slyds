# Next Steps

## Cleanup
- [ ] Remove legacy Node.js code (`bin/`, `lib/`, `templates/`, `package.json`, `package-lock.json`, `node_modules/`)
- [ ] Update `.gitignore` to remove `node_modules/`
- [ ] Update README.md to reflect Go CLI (currently documents the Node.js version)

## Features
- [ ] Additional themes beyond "default"
- [ ] `slyds add` should use theme templates for new slide types (currently uses default theme only)
- [ ] Slide folders with co-located assets (e.g., `slides/03-architecture/slide.html` + `diagram.png`)
- [ ] Live reload on file changes during `slyds serve`
- [ ] `--theme` flag on `slyds init` to select theme

## Polish
- [ ] Better error messages (e.g., when running commands outside a presentation directory)
- [ ] `slyds version` command
- [ ] Release automation / goreleaser setup
- [ ] Publish module path (currently `github.com/user/slyds` — placeholder)
