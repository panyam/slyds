# slyds

## Version
v0.0.10

## Provides
- **slide-cli**: Go CLI for multi-file HTML presentations (one file per slide, composed via templar includes)
- **slide-scaffolding**: Theme-aware presentation scaffolding with manifest-driven updates
- **slide-build**: Flatten includes + inline CSS/JS/images into single self-contained HTML
- **slide-serve**: Dev server with live include resolution
- **slide-management**: Add, remove, reorder, insert slides via CLI commands
- **slide-query**: CSS selector-based read/write access to slide HTML content (goquery), including atomic batch writes
- **slide-introspect**: JSON discovery of layouts, `data-slot` names, themes, and CLI catalog — for agents and automation
- **slide-mcp**: MCP server (stdio and HTTP+SSE) exposing the CLI as a single tool — see `docs/MCP.md`
- **slide-export**: Client-side ZIP export/download of built presentations
- **theme-system**: Config-driven theme templates with shared fallback, runtime theme switching
- **slide-hooks**: Client-side `slideEnter`/`slideLeave` lifecycle events + `window.slydsContext` persistent state
- **layout-system**: Six built-in layouts (title, content, two-col, section, blank, closing) independent of themes
- **agent-onboarding**: AGENT.md auto-generated per deck with commands, layouts, hooks, and conventions; plus `docs/AGENT-THEMES.md` for manifest/theme packs

## Module
github.com/panyam/slyds

## Location
~/projects/slyds

## Stack Dependencies
- templar (github.com/panyam/templar) — template composition, include resolution, serving
- goutils (github.com/panyam/goutils) — indirect, via templar

*(Align versions with `go.mod`; Stackfile.md lists pinned stack versions.)*

## Integration

### Go Module
```go
// go.mod
require github.com/panyam/slyds v0.0.10

// Local development
replace github.com/panyam/templar => ./locallinks/newstack/templar/main
```

### Key Imports
```go
// slyds is a binary, not typically imported as a library
// Use via CLI: slyds init, slyds build, slyds serve, etc.
```

## Status
Active

## Conventions
- No hardcoded HTML in Go code — use embedded `.tmpl` files under `core/templates/`
- Configure templar programmatically — no `.templar.yaml` files
- Slide files are pure HTML — only `index.html` uses templar include syntax
- No regex-based HTML mutation — use `slyds query` (goquery/CSS selectors)
- Local deps via `locallinks/` symlinks; replace directive must be commented before push
- Version injected from git tags via ldflags

## Migrations

_None yet._
