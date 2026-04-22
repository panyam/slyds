# Constraints

Read this before making structural changes. These are enforceable architectural rules.

### No Regex-Based HTML Mutation
**Rule**: Do not build CLI commands that modify slide HTML content using regex or string manipulation. All HTML content reads and writes must go through a proper DOM parser (`slyds query` uses goquery/CSS selectors).
**Why**: Regex-based HTML manipulation is fragile (breaks on nested tags, attributes, whitespace). The `slyds query` command provides a safe, format-aware interface. When structured slide formats (YAML, JSON, MD) are added, query dispatch will route to format-specific handlers.
**Verify**: `grep -rn 'regexp.*h1\|regexp.*slide\|regexp.*speaker' cmd/ | grep -v _test.go | grep -v check.go` — new HTML content parsing should use goquery, not regex. (check.go and extractFirstHeading are legacy, to be migrated incrementally.)
**Scope**: All CLI commands that read or modify slide file content. Does not apply to renaming files, rewriting index.html includes, or scaffolding new slides from templates.

### Batch query uses the same DOM path
**Rule**: `slyds query --batch` must apply operations through the same goquery/fragment pipeline as single `query`; no string-level splicing of slide HTML.
**Why**: Same correctness guarantees as single-query; atomic mode relies on consistent parse/serialize per slide.
**Verify**: Batch implementation calls shared mutation helpers with `goquery` documents, not `strings.Replace` on file bodies.

### No proto-path work until protogen graduates
**Rule**: Do not add, modify, or consolidate proto-based MCP code (`slyds mcp-proto`, `proto/`, `gen/`, `cmd/mcp_proto*.go`) until `mcpkit/ext/protogen` moves out of experimental status.
**Why**: protogen was moved to mcpkit experimental — the API is unstable and not ready for production consumers. The hand-written MCP path (`slyds mcp`) remains the production path.
**Verify**: `git diff --name-only HEAD | grep -E 'proto/|gen/|mcp_proto' | grep -v _test.go` should be empty in normal PRs.

