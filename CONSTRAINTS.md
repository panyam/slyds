# Constraints

Read this before making structural changes. These are enforceable architectural rules.

### No Programmatic HTML Mutation
**Rule**: Do not build CLI commands that parse or modify slide HTML content (e.g., setting titles, injecting media elements, changing CSS classes).
**Why**: Slides are currently raw HTML with no structured schema. Regex/string-based HTML manipulation is fragile and will be thrown away when we add structured slide formats (YAML, JSON, MD). Template rendering (templar) is the path from structured format → HTML, not the reverse.
**Verify**: `grep -r "WriteFile.*slides/" cmd/ | grep -v "_test.go"` — new slide files are OK (scaffold/insert), but modifying existing slide content is not.
**Scope**: All CLI commands that touch slide file content. Does not apply to renaming files or rewriting index.html includes.
