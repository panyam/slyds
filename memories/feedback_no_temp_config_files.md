---
name: no-temp-config-files
description: Configure templar programmatically instead of generating temporary .templar.yaml files
type: feedback
---

Don't generate temporary config files (like .templar.yaml) when using templar as a library.
Configure it programmatically by setting struct fields directly.

**Why:** slyds uses templar as a Go library, not as a standalone CLI. Creating config files adds unnecessary disk artifacts.

**How to apply:** Use templar.NewTemplateGroup(), FileSystemLoader, etc. directly in Go code instead of writing YAML configs.
