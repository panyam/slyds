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

Key files: `core/deck.go` (Deck API), `core/osfs.go` (OS boundary), `core/version.go` (optimistic versioning), `cmd/workspace.go` (Workspace abstraction), `cmd/mcp.go` (MCP server), `cmd/mcp_tools.go` (hand-written tools), `cmd/mcp_resources.go` (hand-written resources), `cmd/mcp_prompts.go` (prompt templates), `cmd/mcp_completions.go` (completions), `cmd/mcp_apps.go` (MCP Apps previews), `cmd/mcp_proto_impl.go` (proto service impl), `cmd/mcp_proto.go` (`mcp-proto` subcommand), `cmd/ws.go` (`slyds ws` debug CLI).

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
- **MCP** — Two server paths: `slyds mcp` (hand-written) and `slyds mcp-proto` (proto-generated, experimental). Both register 14 tools + 9 resources + 3 prompts + completions via mcpkit v0.2.24. Default port: `8274`. Typed handler contexts. MCP Apps with bridge (`--app-bridge`, default on) for host theme adaptation and interactive navigation. Sampling (`improve_slide` uses server→client LLM calls). Elicitation (`remove_slide` confirms, `create_deck` elicits theme — description hints LLM to omit theme for interactive selection). Optimistic versioning (`expected_version` / `expected_deck_version`). `--allow-origin '*'` for tunnel/remote. See [docs/MCP.md](docs/MCP.md). Demo walkthrough: [docs/DEMO-MCP-FEATURES.md](docs/DEMO-MCP-FEATURES.md).
- **Proto MCP** — `proto/slyds/v1/` defines the API as annotated proto RPCs. `protoc-gen-go-mcp` generates typed registrations, sampling helpers (`SampleForImproveSlide`), elicitation helpers (`ElicitThemeChoice`, `ElicitRemoveSlideConfirmation`), and prompt registrations. `SlydsServiceImpl` in `cmd/mcp_proto_impl.go` wraps Workspace. Typed handler contexts (`mcpcore.ToolContext`, `mcpcore.ResourceContext`, `mcpcore.PromptContext`). Entity-focused responses, gRPC status codes for errors. Parity tests validate both paths produce identical output. Dev setup: `cd proto && make setupdev && make buf`.
- **Proto elicitation schema messages** must live in `service.proto` (same file as the RPC), not `models.proto` — `protoc-gen-go-mcp` resolves `schema_message` within `file.Messages` only.
- **CLI-direct agent mode** — `describe --json`, `ls --json`, `check --json`, `build --json`, `ws info --json`, `ws list --json` for agents using shell commands instead of MCP. See [AGENT-SETUP.md](AGENT-SETUP.md).
- **`go:embed` means rebuild** — `slyds.js`, `mcp-embed.css`, `slyds-app.js` are embedded via `go:embed` in `assets/embed.go`. After changing any asset file, you must `make build` and restart the server. Existing decks also need `slyds update` or `make demo` to get updated engine files (the deck's local copies are separate from the binary's embedded copies).
- **`slyds update` hangs** — `core.FetchAll()` tries to download module dependencies over the network. If the module URL is unreachable, it hangs. Use `make demo` to re-scaffold demo decks instead.
- **MCP Apps sandbox** — VS Code and other hosts render MCP App previews in sandboxed iframes. `window.open()` is blocked (no `allow-popups`). Speaker notes use an inline panel in sandboxed contexts (detected via `window.parent !== window`). Export button also blocked in sandbox.
- **Elicitation UX** — if a tool's schema lists options in the description, the LLM will proactively ask the user and pass a value, preventing elicitation from triggering. Use descriptions like "omit to let the user choose interactively" to nudge the LLM to skip the parameter.
- **`SLYDS_MCP_TOKEN`** env var — fallback for `--token` flag in container/CI deployments.
- **`SLYDS_DECK_ROOT`** env var — fallback for `--deck-root` flag on both `slyds mcp` and `slyds ws`. Precedence: explicit flag > env var > `.` (cwd).

## Stack

| Component | Version | Notes |
|-----------|---------|-------|
| templar | v0.1.0 | WritableFS, FSFolder, MemFS, module system |
| mcpkit | v0.2.24 | Typed handler contexts, ToolCallFull, NotifyResourceUpdated, schema validation, streaming, completions, sampling, elicitation |
| mcpkit/ext/ui | v0.2.24 | MCP Apps — display modes, auto-fallback template URIs, RequestDisplayMode, App Bridge (postMessage transport, theme adaptation, bidirectional tools) |
| mcpkit/ext/protogen | v0.2.24 | Proto→MCP codegen, completable_fields, raw content for non-JSON resources |

See [Stackfile.md](Stackfile.md) for full dependency list.

<!-- stack-brain:start -->
## Constraints

### No Regex-Based HTML Mutation
All HTML reads/writes must use `d.Query()` (goquery/CSS selectors), not regex.
*Why: Regex HTML manipulation breaks on nested tags, attributes, whitespace.*
<!-- stack-brain:end -->
