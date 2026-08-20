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

### No protogen dependency until it graduates
**Rule**: Do not add `github.com/panyam/mcpkit/experimental/ext/protogen` (or a generated `gen/` tree that imports it) back to `go.mod` until protogen moves out of mcpkit experimental. The `proto/` definitions stay in the tree as design source, but nothing in the Go build may depend on them.
**Why**: protogen is pinned to the mcpkit v0.2.x API while mcpkit ships v0.5.x, so depending on it strands the whole module on an old, unsupported mcpkit. That is exactly what blocked the v0.5.1 upgrade until the proto path was cut. The hand-written MCP path (`slyds mcp`) is the only production path.
**Verify**: `grep -q protogen go.mod` should find nothing, and `ls gen/ cmd/mcp_proto*.go` should not exist.
**When it graduates**: regenerate from `proto/slyds/v1/` (see [proto/README.md](proto/README.md)), reinstate `cmd/mcp_proto*.go` from git history, and rewrite this constraint.

