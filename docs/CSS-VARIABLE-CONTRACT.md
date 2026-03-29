# CSS Variable Contract

> Defines the `--slyds-*` CSS custom properties that form the interface
> between layouts (structure) and themes (visual style).

## How It Works

All visual styling flows through CSS custom properties prefixed `--slyds-`.
Layouts and structural CSS reference these variables. Themes define them.

```
Layout CSS:  background: var(--slyds-bg);
Theme CSS:   [data-theme="dark"] { --slyds-bg: #1a1a2e; }
```

Swap `data-theme` on the root element, everything re-skins.

## Selector Pattern

```css
/* Base — defines the FULL contract. Always loaded. Provides fallback values. */
[data-theme] {
  --slyds-bg: #ffffff;
  --slyds-fg: #1a1a1a;
  /* ... all variables ... */
}

/* Named themes — override only what they change */
[data-theme="dark"] {
  --slyds-bg: #1e1e2e;
  --slyds-fg: #e0e0e0;
}
```

CSS specificity ensures `[data-theme="dark"]` overrides `[data-theme]`.
Any variable NOT overridden inherits the base default. No theme can have
missing variables.

Per-slide overrides work via CSS cascade:
```html
<html data-theme="light">
  <section class="slide">...</section>                    <!-- inherits light -->
  <section class="slide" data-theme="dark">...</section>  <!-- overrides to dark -->
```

## Variable Tiers

### Core (stable — breaking change to rename or remove)

These are the minimum variables every theme MUST meaningfully define.
Layouts can rely on them unconditionally.

| Variable | Role | Base Default |
|----------|------|--------------|
| `--slyds-bg` | Slide background | `#ffffff` |
| `--slyds-fg` | Primary text color | `#1a1a1a` |
| `--slyds-accent1` | Primary accent — headings, borders, buttons, links | `#0066cc` |
| `--slyds-heading-font` | Heading font stack | `'Segoe UI', system-ui, sans-serif` |
| `--slyds-body-font` | Body text font stack | `'Segoe UI', system-ui, sans-serif` |
| `--slyds-code-font` | Monospace/code font stack | `'Fira Code', 'SF Mono', monospace` |
| `--slyds-code-bg` | Code block background | `#f5f5f5` |
| `--slyds-radius` | Default border-radius | `8px` |

### Extended (base provides defaults — themes SHOULD override for polish)

These have sensible fallback values in the base theme. Themes can override
for a more polished look but aren't required to.

| Variable | Role | Base Default |
|----------|------|--------------|
| `--slyds-accent2` | Secondary accent — hover states, secondary buttons | `#004488` |
| `--slyds-accent3` | Tertiary accent — progress indicators, gradients | `#003366` |
| `--slyds-fg-muted` | Muted/secondary text (counters, labels, captions) | `#6c757d` |
| `--slyds-bg-secondary` | Alternate/secondary background (even slides, stat boxes) | `#f8f9fa` |
| `--slyds-bg-highlight` | Highlight/callout background | `rgba(0, 102, 204, 0.08)` |
| `--slyds-divider` | Border/divider color | `#e0e0e0` |
| `--slyds-shadow` | Box shadow for elevated surfaces | `0 4px 20px rgba(0,0,0,0.08)` |
| `--slyds-code-fg` | Code block text color | `#1a1a1a` |
| `--slyds-code-border` | Code block border color | `#e0e0e0` |
| `--slyds-title-bg` | Title/special slide gradient background | `linear-gradient(135deg, #003366, #0066cc)` |
| `--slyds-title-fg` | Title/special slide text color | `#ffffff` |
| `--slyds-nav-bg` | Navigation button background | `var(--slyds-accent1)` |
| `--slyds-nav-fg` | Navigation button text color | `#ffffff` |
| `--slyds-nav-hover` | Navigation button hover background | `var(--slyds-accent2)` |
| `--slyds-chrome-bg` | Toolbar/chrome background | `rgba(0,0,0,0.03)` |
| `--slyds-chrome-border` | Toolbar border color | `rgba(0,0,0,0.06)` |
| `--slyds-line-height` | Default body line-height | `1.6` |

### Private (namespaced per theme/layout — no cross-package reliance)

Themes or layouts that need extra variables use a namespaced prefix:

```css
[data-theme="hacker"] {
  --slyds-hacker-scanline-opacity: 0.03;
  --slyds-hacker-glow-color: rgba(123, 147, 255, 0.4);
}

.layout-timeline {
  --slyds-timeline-marker-size: 12px;
  --slyds-timeline-line-width: 2px;
}
```

Private variables are not part of the contract. Other themes/layouts
MUST NOT rely on them. They exist for internal use only.

## Theme Mapping

How each existing theme maps to the contract:

| Variable | Default | Dark | Hacker | Corporate | Minimal |
|----------|---------|------|--------|-----------|---------|
| `--slyds-bg` | `#ffffff` | `#1e1e2e` | `#0f1219` | `#ffffff` | `#ffffff` |
| `--slyds-fg` | `#2c3e50` | `#e0e0e0` | `#e2e4f0` | `#1a1a1a` | `#222222` |
| `--slyds-accent1` | `#0d4f4f` | `#00d4ff` | `#7b93ff` | `#0066cc` | `#111111` |
| `--slyds-accent2` | `#1a3a4a` | `#00b4d8` | `#4f6eff` | `#004488` | `#333333` |
| `--slyds-accent3` | `#f0a500` | `#e040fb` | `#73d1ff` | `#003366` | `#555555` |
| `--slyds-fg-muted` | `#6c757d` | `#666688` | `#3a3c4e` | `#6c757d` | `#666666` |
| `--slyds-bg-secondary` | `#f8f9fa` | `rgba(255,255,255,0.02)` | `rgba(123,147,255,0.02)` | `#f8f9fb` | `#fafafa` |
| `--slyds-code-bg` | `#f5f5f5` | `#0d1117` | `#0a0d14` | `#f5f5f5` | `#f5f5f5` |
| `--slyds-code-fg` | `#2c3e50` | `#c9d1d9` | `#73d1ff` | `#333333` | `#222222` |
| `--slyds-heading-font` | `'Segoe UI', sans-serif` | `'Segoe UI', sans-serif` | `'Inter', system-ui` | `'Segoe UI', sans-serif` | `'Georgia', serif` |
| `--slyds-body-font` | `'Segoe UI', sans-serif` | `'Segoe UI', sans-serif` | `'Inter', system-ui` | `'Segoe UI', sans-serif` | `'Georgia', serif` |
| `--slyds-code-font` | `monospace` | `'SF Mono', 'Fira Code'` | `'JetBrains Mono'` | `monospace` | `monospace` |
| `--slyds-radius` | `8px` | `8px` | `8px` | `8px` | `0` |
| `--slyds-shadow` | `0 4px 20px rgba(0,0,0,0.08)` | `0 20px 60px rgba(0,0,0,0.4)` | `0 20px 60px rgba(0,0,0,0.6)` | `0 4px 20px rgba(0,0,0,0.08)` | `none` |

## Versioning

The variable contract is versioned. Theme packages declare which contract
version they target:

```yaml
# theme.yaml in a 3P theme package
name: nord
contract: "1"
```

- Adding new **extended** variables: minor/non-breaking (base provides default)
- Renaming or removing **core** variables: major/breaking
- Adding **private** variables: always safe (namespaced)
