# Design: Templates, Layouts, and Themes

> Refactoring slyds to separate structural layouts from visual themes,
> enable runtime theme switching, support third-party packages via
> templar's module system, and improve LLM friendliness.

## Status: Proposed

## Problem

Today a slyds "theme" (dark, hacker, corporate) conflates two concerns:

1. **Layout** — structural arrangement of content on a slide (title slide,
   two-column, image-full, section divider, etc.)
2. **Visual style** — colors, fonts, spacing, backgrounds.

This means:
- Adding a "light" variant of the dark theme requires duplicating all templates.
- Layouts can't be mixed across visual styles.
- Third-party contributions require duplicating the full template set.
- Runtime theme switching is impossible — the theme is baked into static HTML.

## Inspiration: PowerPoint's Three-Layer Model

PowerPoint cleanly separates:

| Layer | What it defines | slyds equivalent |
|-------|----------------|------------------|
| **Slide Layout** | Placeholder positions and types (title, two-col, blank) | Layout templates + CSS classes |
| **Slide Master** | Default formatting for all layouts in a set | Base structural CSS |
| **Theme** | Color palette, font pair, effects — pure visual skin | CSS custom property bundles |

The key insight: layouts reference **semantic tokens** (`accent1`, `dk1`), not
literal values. Themes map those tokens to actual colors/fonts. Swap the theme,
everything re-skins.

## Proposed Architecture

### CSS Custom Properties as the Contract

All visual styling goes through `--slyds-*` CSS custom properties:

```css
/* Base theme — defines the full contract, always loaded first */
[data-theme] {
  --slyds-bg: #ffffff;
  --slyds-fg: #1a1a1a;
  --slyds-accent1: #0066cc;
  --slyds-accent2: #4488ee;
  --slyds-heading-font: 'Inter', sans-serif;
  --slyds-body-font: 'Inter', sans-serif;
  --slyds-code-font: 'Fira Code', monospace;
  --slyds-code-bg: #f5f5f5;
  --slyds-divider: #e0e0e0;
  --slyds-radius: 4px;
  --slyds-bg-secondary: #f9f9f9;
  --slyds-fg-muted: #666666;
  --slyds-shadow: 0 2px 4px rgba(0,0,0,0.1);
}

/* Dark theme — only overrides what it changes */
[data-theme="dark"] {
  --slyds-bg: #1a1a1a;
  --slyds-fg: #e0e0e0;
  --slyds-accent1: #4fc3f7;
  --slyds-code-bg: #2d2d2d;
  --slyds-divider: #333;
}
```

### Variable Contract Tiers

| Tier | Stability | Rule |
|------|-----------|------|
| **Core** | Stable — breaking change to rename | `--slyds-bg`, `--slyds-fg`, `--slyds-accent1`, `--slyds-heading-font`, `--slyds-body-font`, `--slyds-code-font`, `--slyds-code-bg`, `--slyds-radius` |
| **Extended** | Base provides defaults — themes may override | `--slyds-accent2`, `--slyds-accent3`, `--slyds-divider`, `--slyds-shadow`, `--slyds-bg-secondary`, `--slyds-fg-muted` |
| **Private** | Namespaced to a theme/layout, no cross-package reliance | `--slyds-hacker-scanline-opacity`, `--slyds-timeline-marker-size` |

### Layouts Are Structure, Themes Are Paint

**Layouts** — CSS classes + scaffold templates:
```html
<section class="slide" data-layout="two-col" data-theme="dark">
  <h1 class="slide-title">Architecture</h1>
  <div class="col">...</div>
  <div class="col">...</div>
</section>
```

**Layout CSS** references only `--slyds-*` variables:
```css
.slide            { background: var(--slyds-bg); color: var(--slyds-fg); }
.slide h1, .slide h2 { color: var(--slyds-accent1); font-family: var(--slyds-heading-font); }
.layout-two-col   { display: grid; grid-template-columns: 1fr 1fr; }
```

**Theme CSS** is pure variable overrides — no structural rules.

### Runtime Theme Switching

One line of JS swaps the theme:
```js
document.documentElement.setAttribute('data-theme', 'dark');
```

Per-slide override via CSS cascade:
```html
<html data-theme="light">
  <section class="slide">...</section>                    <!-- inherits light -->
  <section class="slide" data-theme="dark">...</section>  <!-- overrides to dark -->
```

A theme switcher button in the bottom toolbar (same pattern as speaker notes)
cycles through available themes.

### Named Slots for LLM Friendliness

Layouts define named slots via `data-slot`:
```html
<section class="slide" data-layout="comparison">
  <h1 class="slide-title">Before vs After</h1>
  <div class="slot" data-slot="before">...</div>
  <div class="slot" data-slot="after">...</div>
  <div class="slot" data-slot="verdict">...</div>
</section>
```

Enables targeted content manipulation:
```bash
slyds query 03 --slot before --set "Old Architecture"
```

### Fallback Chain for Missing Variables

CSS specificity handles this naturally:
1. `[data-theme]` selector defines all variables with sensible defaults (the base theme).
2. `[data-theme="dark"]` overrides only what it changes.
3. If hacker doesn't define `--slyds-divider`, the base `[data-theme]` value applies.
4. New variables added to base are automatically available to all themes.

No theme can have missing variables. Themes stay minimal — only declare what makes them distinctive.

## Dependency Management via Templar Modules

### Why Not npm

- slyds is a single Go binary — adding npm means requiring Node.js
- Templar already has a module system: `templar.yaml`, `templar.lock`, `templar get`
- What we're distributing (CSS variable blocks, scaffold templates) is trivially small
- Templar's `SourceLoader` already resolves `@sourcename/path` references

### Per-Deck Module Structure

Everything lives in the presentation directory:

```
my-talk/
├── .slyds.yaml               # manifest: title, theme, sources
├── .slyds.lock                # pinned versions (generated)
├── .slyds-modules/            # vendored dependencies (gitignored)
│   ├── core/                  # @slyds/core
│   │   ├── slyds.css
│   │   ├── slyds.js
│   │   ├── slyds-export.js
│   │   ├── themes/
│   │   │   ├── _base.css
│   │   │   ├── light.css
│   │   │   ├── dark.css
│   │   │   └── hacker.css
│   │   └── layouts/
│   │       ├── title.tmpl
│   │       ├── content.tmpl
│   │       └── two-col.tmpl
│   └── slyds-theme-nord/     # 3P theme
│       ├── theme.css
│       └── theme.yaml
├── slides/
│   ├── 01-title.html
│   └── 02-content.html
├── index.html
└── dist/
    └── index.html             # self-contained build output
```

### `.slyds.yaml` Absorbs Templar Config

```yaml
title: "My Talk"
theme: dark

sources:
  core:
    url: github.com/panyam/slyds-core
    version: v1.0.0
  nord:
    url: github.com/someone/slyds-theme-nord
    version: v1.0.0

modules_dir: .slyds-modules
```

slyds reads `.slyds.yaml`, constructs a `templar.VendorConfig` programmatically,
and hands it to templar's library APIs. The user never sees `templar.yaml`.

### Third-Party Packages

**Themes** are trivial — just CSS variable overrides in a git repo:
```
github.com/someone/slyds-theme-nord/
├── theme.css          # [data-theme="nord"] { --slyds-bg: ... }
├── theme.yaml         # metadata: name, contract-version
└── README.md
```

**Layouts** are CSS + optional scaffold template:
```
github.com/someone/slyds-layout-timeline/
├── timeline.css       # .layout-timeline { ... } using --slyds-* vars
├── timeline.tmpl      # scaffold for `slyds add --layout timeline`
└── theme.yaml
```

Layouts reference only `--slyds-*` variables, so they work with every theme.

### Version Pinning

- `.slyds.yaml` declares version ranges (like `go.mod`)
- `.slyds.lock` pins exact resolved commits (like `go.sum`)
- `.slyds-modules/` is the local cache (gitignored)
- `slyds update` re-resolves and downloads (like `go get -u`)

### Validation: `slyds check`

Extended to validate the variable contract:
```
$ slyds check
✓ All slides referenced in index.html exist
✓ Theme "dark" defines all core variables
⚠ Layout "timeline" uses --slyds-accent3 (extended) — theme "hacker" doesn't override it, will use base default
✓ All assets referenced in slides exist
✓ Engine version 1.2.3 matches lock file
```

## Templar Library Customizability

Templar's CLI hardcodes `templar.yaml` as the config file name. When used as a
library, applications should control:

- Config file name(s) — slyds uses `.slyds.yaml`
- Vendor directory name — slyds uses `.slyds-modules`
- Lock file name — slyds uses `.slyds.lock`

The library APIs (`VendorConfig`, `SourceLoader`, `FetchSource`) already accept
these as struct fields/function parameters. `FindVendorConfig()` is the only
function that hardcodes file names — it should accept configurable names, or
(better) applications construct `VendorConfig` directly and skip file discovery.

## LLM Friendliness

### Semantic Structure

Consistent layout classes + `data-layout` + `data-slot` make slides
machine-parseable. An LLM can reason about slide structure without parsing
arbitrary HTML.

### `slyds describe`

Outputs a structured deck summary for LLM consumption:
```yaml
title: "System Architecture"
theme: dark
slide_count: 8
layouts_used: [title, content, two-col]
slides:
  1: {title: "System Architecture", layout: title, words: 12}
  2: {title: "The Problem", layout: content, words: 85}
themes_available: [light, dark, corporate, hacker]
layouts_available: [title, content, two-col, section, image-full]
```

### Auto-Generated CLAUDE.md

`slyds init` drops a `CLAUDE.md` in the deck directory documenting available
commands, layouts, themes, and conventions — any LLM agent that reads it
immediately knows how to work with the deck.

### `slyds prompt`

Generates a self-contained LLM-ready prompt with deck manifest, relevant slide
contents, available layouts/themes, and the exact `slyds` commands to run.

## Implementation Phases

| Phase | Summary | Depends On |
|-------|---------|------------|
| **0** | Architecture doc + CSS variable contract spec | — |
| **1** | CSS variable extraction + theme switcher in toolbar | Phase 0 |
| **2** | Layout/theme split in CLI (`--layout` flag, scaffold templates) | Phase 1 |
| **3** | Templar SourceLoader integration in slyds | Phase 1 |
| **4** | Extract slyds-core to separate repo, remove go:embed for CSS/JS | Phase 3 |
| **5** | LLM features: `describe`, `CLAUDE.md` gen, named slots, `prompt` | Phase 2 |
| **6** | Templar library: configurable file names for embedding apps | Phase 3 |

Phases 2 and 3 can run in parallel. Phase 5 and 6 can run in parallel.
