---
name: no-hardcoded-html
description: Don't hardcode HTML in Go code — use embedded template files for extensibility (themes, customization)
type: feedback
---

Don't hardcode HTML strings in Go source files for scaffold/template generation.
Use embedded template files under core/templates/<theme>/ instead.

**Why:** The user wants themes to be extensible — adding a new theme should be adding template files, not modifying Go code. Hardcoded HTML prevents this.

**How to apply:** When generating HTML output (slide scaffolding, index.html, etc.), always render from embedded `.tmpl` files loaded via the core package. New slide types or themes = new template files, not new Go code.
