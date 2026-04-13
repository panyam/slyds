# CLAUDE.md — slyds

Go CLI + library for multi-file HTML presentations. See [ARCHITECTURE.md](ARCHITECTURE.md) for design, [ROADMAP.md](ROADMAP.md) for phases, [NEXTSTEPS.md](NEXTSTEPS.md) for TODO.

## Build & Test

```bash
make resymlink   # Set up locallinks/ for local templar dependency (first time only)
make build       # Build the slyds binary (injects version from git tags)
make test        # Run all tests
make install     # Install to $GOBIN
make audit       # govulncheck + gosec + gitleaks
```

## Project Layout

- `core/` — all business logic as methods on `*Deck`. Zero `os.*` except `osfs.go` (OS boundary).
- `assets/` — embedded static files (themes, layouts, templates, engine CSS/JS). Separate from core/ so module system can vendor assets without Go code.
- `cmd/` — CLI wiring + MCP server (tools, resources). Never touches FS directly.
- `examples/` — demo presentations with tests.
- `docs/` — MCP setup, agent themes, CSS contract, design docs.

Key files: `core/deck.go` (Deck API), `core/osfs.go` (OS boundary), `cmd/workspace.go` (Workspace abstraction), `cmd/mcp.go` (MCP server), `cmd/mcp_tools.go` (11 semantic tools), `cmd/mcp_resources.go` (7 browsable resources), `cmd/mcp_apps.go` (MCP Apps preview tools), `cmd/ws.go` (`slyds ws` debug CLI).

## FS Abstraction

All Deck I/O goes through `templar.WritableFS` (v0.1.0). No `os.*`/`filepath.*` in core/ production code except `osfs.go`.

- `templar.NewLocalFS(dir)` — local disk (CLI)
- `templar.NewMemFS()` — in-memory (tests, WASM)
- `osfs.go` — the **only** file with `os.*`; contains `Create`, `CreateInDir`, `CreateFromDir`, `WriteAgentMD`, `FindDeckRoot`

## Conventions

- **Deck is the single API** — cmd/ calls Deck methods, never touches FS internals
- **Workspace is the MCP boundary** — MCP tool and resource handlers resolve decks via `workspaceFromContext(ctx).OpenDeck(name)`, never from raw paths. The workspace is installed on every request via `workspaceMiddleware`. See [cmd/workspace.go](cmd/workspace.go).
- **Three-layer slide identity** — slides have position (mutable on insert), slug (stable across inserts, not across renames), and `slide_id` (stable across everything including renames). `slide_id` is a `sl_`-prefixed opaque ID stored per-slide in `.slyds.yaml` and returned by `describe_deck`/`list_slides`. Agents should use slug for human-readable references and slide_id for rename-safe caching. `ResolveSlide` accepts all three forms plus filenames/substrings.
- **No hardcoded HTML** — use embedded `.tmpl` files under `assets/templates/`
- **No regex HTML mutation** — use `d.Query()` (goquery/CSS selectors). See [CONSTRAINTS.md](CONSTRAINTS.md)
- **Index.html is source of truth** for slide ordering
- **Local deps via locallinks/** — `replace => ./locallinks/...` in go.mod; comment out before push

## Gotchas

- **macOS /private symlinks**: temp dirs resolve `/var/...` vs `/private/var/...`. Don't compare paths in tests.
- **`go:embed` paths relative to Go file** — `assets/embed.go` lives alongside the embedded files; `core/embed.go` re-exports.
- **Theme render fallback** — `InsertSlide` uses layout system first; falls back to theme templates.
- **MCP** — 13 tools (11 core + 2 preview) + 9 resources (7 data + 2 preview) via mcpkit v0.2.2. Typed handler contexts (`ToolContext`/`ResourceContext`). Single-struct registration (`srv.Register`). Workspace middleware. Per-tool timeouts. MCP Apps extension with display modes (inline/fullscreen), concrete + template resource URIs, `NotifyResourceUpdated`. `--allow-origin '*'` for tunnel/remote deployments. See [docs/MCP.md](docs/MCP.md). `--deck-root` sets the local workspace root.
- **CLI-direct agent mode** — `describe --json`, `ls --json`, `check --json`, `build --json`, `ws info --json`, `ws list --json` for agents using shell commands instead of MCP. See [AGENT-SETUP.md](AGENT-SETUP.md).
- **`SLYDS_MCP_TOKEN`** env var — fallback for `--token` flag in container/CI deployments.
- **`SLYDS_DECK_ROOT`** env var — fallback for `--deck-root` flag on both `slyds mcp` and `slyds ws`. Precedence: explicit flag > env var > `.` (cwd).

## Stack

| Component | Version | Notes |
|-----------|---------|-------|
| templar | v0.1.0 | WritableFS, FSFolder, MemFS, module system |
| mcpkit | v0.2.2 | Typed handler contexts (ToolContext/ResourceContext), split packages, SSE + Streamable HTTP + stdio, per-tool timeout, error handler, StructuredResult, EventStore, schema validation, streaming results, NotifyResourceUpdated |
| mcpkit/ext/ui | v0.2.2 | MCP Apps extension — display modes, template resources, RequestDisplayMode, ElicitWithApp |

See [Stackfile.md](Stackfile.md) for full dependency list.

<!-- stack-brain:start -->
## Constraints

### No Regex-Based HTML Mutation
All HTML reads/writes must use `d.Query()` (goquery/CSS selectors), not regex.
*Why: Regex HTML manipulation breaks on nested tags, attributes, whitespace.*
<!-- stack-brain:end -->
