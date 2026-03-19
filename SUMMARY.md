# Summary

slyds is a Go CLI for creating, serving, and building self-contained HTML presentations with multi-file slide authoring.

## What it does

- **`init`**: Scaffolds a presentation from theme templates — one HTML file per slide, composed via templar includes
- **`serve`**: Dev server that resolves templar includes on-the-fly
- **`build`**: Flattens all includes and inlines CSS/JS/images into a single `dist/index.html`
- **`add/rm/mv/ls`**: Slide management — create, delete, reorder slides with auto-renumbering

## Current State

The Go rewrite is functional with all core commands working and 21+ unit/integration tests passing. The legacy Node.js code (`bin/`, `lib/`, `templates/`, `package.json`) is still present but unused — pending cleanup.

## Key Patterns

- Theme templates embedded via `go:embed` under `assets/templates/<theme>/`
- Templar used as a Go library (programmatic config, no YAML files)
- Slide files are pure HTML fragments — no template syntax
- Only `index.html` uses templar's `{{# include #}}` directives
