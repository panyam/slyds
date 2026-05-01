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
- **External themes via `{deck-root}/themes/`** — subdirectories containing `theme.yaml` are auto-discovered as external themes. `Workspace.AvailableThemes()` merges built-in + external. `Workspace.ExternalThemeFS(name)` returns the `fs.FS` for scaffolding. `CreateDeck` uses `core.CreateInDirWithThemeFS` for external themes, `core.CreateInDir` for built-in. All theme-listing call sites in cmd/ use `ws.AvailableThemes()` instead of `core.AvailableThemeNames()`.
- **Three-layer slide identity** — slides have position (mutable on insert), slug (stable across inserts, not across renames), and `slide_id` (stable across everything including renames). `slide_id` is a `sl_`-prefixed opaque ID stored per-slide in `.slyds.yaml` and returned by `describe_deck`/`list_slides`. Agents should use slug for human-readable references and slide_id for rename-safe caching. `ResolveSlide` accepts all three forms plus filenames/substrings.
- **No hardcoded HTML** — use embedded `.tmpl` files under `assets/templates/`
- **No regex HTML mutation** — use `d.Query()` (goquery/CSS selectors). See [CONSTRAINTS.md](CONSTRAINTS.md)
- **Index.html is source of truth** for slide ordering
- **Local deps via locallinks/** — `replace => ./locallinks/...` in go.mod; comment out before push

## Gotchas

- **macOS /private symlinks**: temp dirs resolve `/var/...` vs `/private/var/...`. Don't compare paths in tests.
- **`go:embed` paths relative to Go file** — `assets/embed.go` lives alongside the embedded files; `core/embed.go` re-exports.
- **Theme render fallback** — `InsertSlide` uses layout system first; falls back to theme templates.
- **MCP** — Two server paths: `slyds mcp` (hand-written, production) and `slyds mcp-proto` (proto-generated, experimental — see CONSTRAINTS.md). Both register 14 tools + 9 resources + 3 prompts + completions via mcpkit v0.2.38. Default port: `8274`. Typed handler contexts. MCP Apps with bridge (`--app-bridge`, default on) for host theme adaptation and interactive navigation. Sampling (`improve_slide` uses server→client LLM calls). Elicitation (`remove_slide` confirms, `create_deck` elicits theme — description hints LLM to omit theme for interactive selection). Optimistic versioning (`expected_version` / `expected_deck_version`). `--allow-origin '*'` for tunnel/remote. See [docs/MCP.md](docs/MCP.md). Demo walkthrough: [docs/DEMO-MCP-FEATURES.md](docs/DEMO-MCP-FEATURES.md).
- **Proto MCP (experimental)** — `proto/slyds/v1/` defines the API as annotated proto RPCs. `protoc-gen-go-mcp` generates typed registrations, sampling helpers, elicitation helpers, and prompt registrations. **Protogen is in mcpkit experimental** — no proto-path work until it graduates (see CONSTRAINTS.md). `proto/mcp` symlinks to `~/newstack/mcpkit/main/experimental/ext/protogen/proto/mcp`. Buf config: dev uses local symlink + `buf.build/mcpkit/protogen` BSR dep; prod uses BSR dep only. Dev setup: `cd proto && make setupdev && make buf`.
- **Proto elicitation schema messages** must live in `service.proto` (same file as the RPC), not `models.proto` — `protoc-gen-go-mcp` resolves `schema_message` within `file.Messages` only.
- **CLI-direct agent mode** — `describe --json`, `ls --json`, `check --json`, `build --json`, `ws info --json`, `ws list --json` for agents using shell commands instead of MCP. See [AGENT-SETUP.md](AGENT-SETUP.md).
- **`go:embed` means rebuild** — `slyds.js`, `mcp-embed.css`, `slyds-app.js` are embedded via `go:embed` in `assets/embed.go`. After changing any asset file, you must `make build` and restart the server. Existing decks also need `slyds update` or `make demo` to get updated engine files (the deck's local copies are separate from the binary's embedded copies).
- **`slyds update` hangs** — `core.FetchAll()` tries to download module dependencies over the network. If the module URL is unreachable, it hangs. Use `make demo` to re-scaffold demo decks instead.
- **MCP Apps sandbox** — VS Code and other hosts render MCP App previews in sandboxed iframes. `window.open()` is blocked (no `allow-popups`). Speaker notes use an inline panel in sandboxed contexts (detected via `window.parent !== window`). Export button also blocked in sandbox.
- **Elicitation UX** — if a tool's schema lists options in the description, the LLM will proactively ask the user and pass a value, preventing elicitation from triggering. Use descriptions like "omit to let the user choose interactively" to nudge the LLM to skip the parameter.
- **MCP Auth** — JWT validation via `--jwks-url`, `--issuer`, `--audience`. PRM at `/.well-known/oauth-protected-resource/mcp` + RFC 8414 AS metadata proxy for OIDC-only providers (Keycloak). Scoped access: mutation tools require `slyds-write` scope (not `slyds:write` — Keycloak uses hyphens). `MCPAuthConfig` in `cmd/mcp_auth.go` encapsulates auth setup. `--verbose` enables request logging. Falls back to `--token` when `--jwks-url` not set. VS Code browser OAuth works via PKCE with `slyds-public` client. See [docs/AUTH-TESTING.md](docs/AUTH-TESTING.md) for 7 manual test flows.
- **`SLYDS_MCP_TOKEN`** env var — fallback for `--token` flag in container/CI deployments.
- **`SLYDS_DECK_ROOT`** env var — fallback for `--deck-root` flag on both `slyds mcp` and `slyds ws`. Precedence: explicit flag > env var > `.` (cwd).

## Stack

| Component | Version | Notes |
|-----------|---------|-------|
| templar | v0.1.0 | WritableFS, FSFolder, MemFS, module system |
| mcpkit | v0.2.40 | TypedTool, typed handler contexts, schema validation, sampling, elicitation |
| mcpkit/ext/auth | v0.2.40 | JWT validation, PRM, RFC 8414 proxy, scope enforcement, OAuth discovery |
| mcpkit/ext/ui | v0.2.40 | MCP Apps — display modes, App Bridge, theme adaptation, bidirectional tools |
| mcpkit/experimental/ext/protogen | v0.2.41 | Proto→MCP codegen (experimental), completable_fields, mcp_sampling, mcp_elicit, mcp_prompt |

See [Stackfile.md](Stackfile.md) for full dependency list.

<!-- stack-brain:start -->
## Constraints

### No Regex-Based HTML Mutation
All HTML reads/writes must use `d.Query()` (goquery/CSS selectors), not regex.
*Why: Regex HTML manipulation breaks on nested tags, attributes, whitespace.*
<!-- stack-brain:end -->
