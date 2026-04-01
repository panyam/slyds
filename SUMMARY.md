# Summary

slyds is a Go CLI for creating, serving, and building self-contained HTML presentations with multi-file slide authoring.

## What it does

- **`init`**: Scaffolds a presentation from theme templates — one HTML file per slide, composed via templar includes
- **`update`**: Refreshes engine/theme files (CSS, JS, index layout, theme images) without touching slides
- **`serve`**: Dev server that resolves templar includes on-the-fly
- **`build`**: Flattens all includes and inlines CSS/JS/images into a single `dist/index.html`
- **`add/rm/mv/ls`**: Slide management — create, delete, reorder slides with auto-renumbering
- **`insert`**: Insert slide at any position with auto-renumber
- **`slugify`**: Bulk rename slides to slug-based filenames from `<h1>` content
- **`check`**: Validate deck — sync, missing notes, broken assets, talk time estimate
- **`query`**: CSS selector interface for reading/writing slide HTML content (goquery); optional **`--batch`** JSON for atomic multi-slide writes
- **`introspect`**: Machine-readable JSON listing layouts (with `data-slot` names), built-in themes, and CLI catalog — for agents and MCP clients
- **`describe`**: Per-deck structured summary (`--json` for tools)
- **`mcp` / `mcp serve`**: Model Context Protocol — stdio (local) or HTTP+SSE (remote) thin wrapper around the CLI; see `docs/MCP.md`
- **Export button**: Client-side download in built presentations — extracts slides from DOM, zips, triggers browser download (works from `file://`)
- **`version`**: Print version (injected from git tags at build time)

## Current State

The Go rewrite is complete with all core commands working, 130+ tests passing, CI/CD via GitHub Actions, and cross-platform binary releases via goreleaser. Five built-in themes, six built-in layouts, slide lifecycle hooks (`slideEnter`/`slideLeave`), and runtime theme switching. Published as `github.com/panyam/slyds` at v0.0.10.

## Key Patterns

- Theme templates embedded via `go:embed` under `core/templates/` (shared) and `core/templates/<theme>/` (overrides)
- Templar used as a Go library (programmatic config, no YAML files)
- Slide files are pure HTML fragments — no template syntax
- Only `index.html` uses templar's `{{# include #}}` directives
- `index.html` is the source of truth for slide ordering (not filesystem sort)
- `.slyds.yaml` manifest tracks theme and title for `slyds update`
- HTML content access via goquery/CSS selectors (`slyds query` and `query --batch`), not regex
- Agent onboarding: `introspect`, `describe`, optional `--slots-file` on `add`/`insert`, and docs under `docs/AGENT-THEMES.md` and `docs/MCP.md`
- MCP HTTP transport: integration tests in `cmd/mcp_http_test.go` (`go test ./cmd/... -run MCP`) exercise SSE `endpoint` → POST → `message` and auth/origin paths
- Version injected from git tags via ldflags at build time
