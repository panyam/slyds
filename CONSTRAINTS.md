# Constraints

Read this before making structural changes. These are enforceable architectural rules.

### No Regex-Based HTML Mutation
**Rule**: Do not build CLI commands that modify slide HTML content using regex or string manipulation. All HTML content reads and writes must go through a proper DOM parser (`slyds query` uses goquery/CSS selectors).
**Why**: Regex-based HTML manipulation is fragile (breaks on nested tags, attributes, whitespace). The `slyds query` command provides a safe, format-aware interface. When structured slide formats (YAML, JSON, MD) are added, query dispatch will route to format-specific handlers.
**Verify**: `grep -rn 'regexp.*h1\|regexp.*slide\|regexp.*speaker' cmd/ | grep -v _test.go | grep -v check.go` — new HTML content parsing should use goquery, not regex. (check.go and extractFirstHeading are legacy, to be migrated incrementally.)
**Scope**: All CLI commands that read or modify slide file content. Does not apply to renaming files, rewriting index.html includes, or scaffolding new slides from templates.
