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

Key files: `core/deck.go` (Deck API), `core/osfs.go` (OS boundary), `core/version.go` (optimistic versioning), `cmd/workspace.go` (Workspace abstraction), `cmd/mcp.go` (MCP server), `cmd/mcp_tools.go` (hand-written tools), `cmd/mcp_resources.go` (hand-written resources), `cmd/mcp_apps.go` (MCP Apps previews), `cmd/mcp_proto_impl.go` (proto service impl), `cmd/mcp_proto.go` (`mcp-proto` subcommand), `cmd/ws.go` (`slyds ws` debug CLI).

- `proto/` — Proto definitions + buf config. `proto/slyds/v1/service.proto` (tools + resources), `proto/slyds/v1/models.proto` (messages).
- `gen/` — Generated code from protos. `gen/go/slyds/v1/service.pb.mcp.go` (MCP registrations).

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
- **MCP** — Two server paths: `slyds mcp` (hand-written) and `slyds mcp-proto` (proto-generated, experimental). Both register 13 tools + 9 resources + completions via mcpkit v0.2.15+. Typed handler contexts. MCP Apps with display modes + auto-fallback template URIs. Optimistic versioning (`expected_version` / `expected_deck_version`). `--allow-origin '*'` for tunnel/remote. See [docs/MCP.md](docs/MCP.md).
- **Proto MCP** — `proto/slyds/v1/` defines the API as annotated proto RPCs. `protoc-gen-go-mcp` generates typed registrations. `SlydsServiceImpl` in `cmd/mcp_proto_impl.go` wraps Workspace. Entity-focused responses, gRPC status codes for errors. Parity tests validate both paths produce identical output. Dev setup: `cd proto && make setupdev && make buf`.
- **CLI-direct agent mode** — `describe --json`, `ls --json`, `check --json`, `build --json`, `ws info --json`, `ws list --json` for agents using shell commands instead of MCP. See [AGENT-SETUP.md](AGENT-SETUP.md).
- **`SLYDS_MCP_TOKEN`** env var — fallback for `--token` flag in container/CI deployments.
- **`SLYDS_DECK_ROOT`** env var — fallback for `--deck-root` flag on both `slyds mcp` and `slyds ws`. Precedence: explicit flag > env var > `.` (cwd).

## Stack

| Component | Version | Notes |
|-----------|---------|-------|
| templar | v0.1.0 | WritableFS, FSFolder, MemFS, module system |
| mcpkit | v0.2.15 | Typed handler contexts, ToolCallFull, NotifyResourceUpdated, schema validation, streaming, completions |
| mcpkit/ext/ui | v0.2.15 | MCP Apps — display modes, auto-fallback template URIs, RequestDisplayMode |
| mcpkit/ext/protogen | v0.2.16 | Proto→MCP codegen, completable_fields, raw content for non-JSON resources |

See [Stackfile.md](Stackfile.md) for full dependency list.

<!-- stack-brain:start -->
## Constraints

### No Regex-Based HTML Mutation
All HTML reads/writes must use `d.Query()` (goquery/CSS selectors), not regex.
*Why: Regex HTML manipulation breaks on nested tags, attributes, whitespace.*
<!-- stack-brain:end -->
