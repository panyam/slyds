# CLAUDE.md — slyds

## What is this?

slyds is a Go CLI for multi-file HTML presentations. Each slide lives in its own file; `index.html` composes them via templar's `{{# include #}}` directives. `slyds build` flattens everything into a single self-contained HTML file.

See [ARCHITECTURE.md](ARCHITECTURE.md) for design details.

## Build & Test

```bash
make resymlink   # Set up locallinks/ for local templar dependency (first time only)
make build       # Build the slyds binary
make test        # Run all tests
make install     # Install to $GOBIN
make setup-tools # Install required Go tools (cobra-cli)
```

## Key Commands

```bash
slyds init "Title" [-n count] [--theme dark]      # Scaffold presentation
slyds update [dir]                               # Refresh engine/theme files, preserve slides
slyds serve [dir] [-p port]                      # Dev server with live include resolution
slyds build [dir]                                # Flatten to dist/index.html
slyds add "name" [--after N] [--type content]    # Add slide (append or after position)
slyds insert <pos> "name" [--type T] [--title T] # Insert slide at position
slyds rm <name-or-number>                        # Remove slide
slyds mv <from> <to>                             # Reorder slides
slyds ls                                         # List slides (index.html order)
```

## Conventions

- **No hardcoded HTML in Go code** — use embedded `.tmpl` files under `assets/templates/<theme>/`. New themes = new template dirs, not Go changes.
- **Slide types are config-driven** — each theme has a `theme.yaml` that maps type names to template files. Add a custom type by adding a `.tmpl` file and registering it in `theme.yaml`.
- **Configure templar programmatically** — don't generate `.templar.yaml` files. Use `TemplateGroup`, `FileSystemLoader`, etc. directly.
- **Local deps via locallinks/** — `go.mod` uses `replace => ./locallinks/newstack/templar/main`. Run `make resymlink` to create the symlink.
- **Slide files are pure HTML** — only `index.html` uses templar `{{# include #}}` syntax.
- **`.slyds.yaml` manifest** — created by `init`, stores `theme` and `title`. Used by `update` to know how to re-render templates. If missing, `update` prompts interactively.

## Project Layout

```
main.go                     # Entry point
cmd/                        # Cobra commands (init, update, serve, build, add/rm/mv/ls)
internal/scaffold/          # Presentation scaffolding, update, and manifest management
internal/builder/           # Include flattening + CSS/JS/image inlining
assets/                     # go:embed package — slyds.css, slyds.js, theme templates
assets/templates/<theme>/   # Theme template files (.tmpl) — default, minimal, dark, corporate, hacker
```
## Gotchas

- **macOS /private symlinks**: `filepath.Abs` on temp dirs returns `/var/...` but the actual path is `/private/var/...`. Don't compare paths directly in tests; check file existence instead.
- **templar BasicServer `/` routing**: It maps `/` to template name `""` which fails. slyds uses a custom HTTP handler that maps `/` → `index.html` instead.
- **`go:embed` paths are relative to the Go file** — can't use `../` paths. The `assets/embed.go` file must live alongside the files it embeds.

## Memories

Memories are stored in-repo under `memories/` (not in the global `~/.claude/` config) so they're tracked in version control. See [memories/MEMORY.md](memories/MEMORY.md) for the index. When saving new memories, write them to `memories/` in this repo.
