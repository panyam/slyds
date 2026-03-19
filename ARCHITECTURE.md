# Architecture

## Overview

slyds is a Go binary that imports [templar](https://github.com/panyam/templar) as a library for template composition and serving. It uses Cobra for the CLI (same stack as templar).

```
slyds (Go binary)
  ├── imports templar for template include resolution + HTML rendering
  ├── uses Cobra for CLI
  └── embeds default assets (CSS, JS, theme templates) via go:embed
```

## Multi-File Slide Authoring

Presentations use one file per slide, composed via templar includes:

```
my-talk/
  index.html              # Organizer — templar {{# include #}} directives
  slyds.css               # Presentation engine styles (copied from embedded)
  slyds.js                # Client-side slide engine (copied from embedded)
  theme.css               # User's theme overrides
  slides/
    01-title.html          # Pure HTML slide files
    02-slide.html
    03-closing.html
```

### Key Design: Slides are pure HTML

Only `index.html` uses templar's `{{# include "slides/01-title.html" #}}` syntax. Individual slide files have zero template awareness — they're just `<div class="slide">` fragments.

## Theme System

Themes are sets of `.tmpl` files under `assets/templates/<theme>/`:

```
assets/templates/default/
  index.html.tmpl                  # Go text/template for index.html
  theme.css.tmpl                   # Go text/template for theme.css
  slides/title.html.tmpl           # Slide type templates
  slides/content.html.tmpl
  slides/closing.html.tmpl
```

Templates receive `{{.Title}}`, `{{.Number}}`, `{{.Includes}}` etc. Adding a new theme means adding a new directory with the same file names.

## Serve vs Build

**`slyds serve`**: Custom HTTP handler that routes `.html` requests through templar's `TemplateGroup.RenderHtmlTemplate()` (resolves includes on-the-fly) and serves everything else as static files.

**`slyds build`**: Uses templar's `TemplateGroup` to flatten all includes, then inlines CSS (`<link>` → `<style>`), JS (`<script src>` → `<script>`), and images (local files → base64 data URIs). Output is a single `dist/index.html` that works from `file://`.

## Templar Integration

slyds uses templar as a Go library, configuring it programmatically:

- `templar.NewTemplateGroup()` — creates template manager
- `templar.NewFileSystemLoader(root)` — loads templates from presentation directory
- `group.RenderHtmlTemplate()` — renders with include resolution

No `.templar.yaml` config files are generated or needed.

## Dependency Management

The `go.mod` uses a local replace directive for templar development:
```
replace github.com/panyam/templar => ./locallinks/newstack/templar/main
```

The `locallinks/` directory contains symlinks created by `make resymlink`.
