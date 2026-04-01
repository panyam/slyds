# slyds MCP (Model Context Protocol)

[slyds](https://github.com/panyam/slyds) exposes a thin [Model Context Protocol](https://modelcontextprotocol.io/) server with **one tool**, **`slyds`**, that runs the real CLI (`os.Executable()`) in a working directory you supply. No slide logic lives in the MCP layer.

**Build the binary** from the repo (`make build`) and use the **absolute path** to `./slyds` in stdio configs below.

## Tool contract (all clients)

Each `tools/call` for **`slyds`** should include:

| Field | Required | Meaning |
|-------|----------|---------|
| **`cwd`** | yes | Directory containing the deck’s `index.html`, or a subdirectory under it |
| **`args`** | yes | argv **after** `slyds`, e.g. `["introspect"]`, `["describe", "--json"]`, `["query", "1", "h1", "--count"]` |
| **`min_version`** | no | Minimum slyds version; fails if the binary is older |

Examples: `["introspect"]`, `["describe", "--json"]`, `["check"]`, `["ls"]`.

## Transports

| Transport | Command | Typical clients |
|-----------|---------|-----------------|
| **stdio** | `slyds mcp` | Cursor, Claude Desktop, subprocess |
| **HTTP + SSE** | `slyds mcp serve` | Remote URLs, hosted agents (e.g. Glean-style) |

---

## Coding agents: Cursor

Cursor integrates MCP by **spawning a process** over **stdio** — use **`slyds mcp`**. **`slyds mcp serve`** is for **URL**-based clients (remote agents, curl tests), not the usual Cursor chat path.

### Prerequisites

```bash
cd /path/to/slyds
make build
./slyds version
```

### stdio (recommended)

**Config:** Cursor reads **`mcp.json`** from **project** `.cursor/mcp.json` or **user** `~/.cursor/mcp.json`.

```json
{
  "mcpServers": {
    "slyds": {
      "command": "/Users/you/projects/slyds/slyds",
      "args": ["mcp"]
    }
  }
}
```

- **`command`**: full path to `slyds` (not a shell alias).
- **`args`**: must include **`mcp`**.

**Reload:** restart Cursor (or MCP reload) after saving.

**Verify:** Settings → MCP → server **`slyds`**. Terminal sanity check: `/path/to/slyds mcp` (blocks on stdin; Ctrl+C to exit).

**In chat:** invoke tool **`slyds`** with **`cwd`** (deck root) and **`args`** (e.g. `["introspect"]`).

### Remote (HTTP + SSE) in Cursor

1. Start: `slyds mcp serve --listen 127.0.0.1:8787` → SSE at `http://127.0.0.1:8787/mcp/sse` (default `--path-prefix /mcp`).
2. Cursor’s primary integration is **stdio**. For HTTP/SSE, check current [Cursor MCP docs](https://cursor.com/docs) for **`url`** / SSE support and `mcp.json` shape.
3. Behind TLS: `slyds mcp serve --listen 127.0.0.1:8787 --public-url https://your-host.example/mcp` — see [HTTP + SSE](#http--sse-remote-mcp-2024-11-05) below.
4. Prefer **`--token`** and TLS when exposing beyond localhost; avoid **`--dangerous-allow-any-origin`** outside debugging.

### Troubleshooting (Cursor)

| Symptom | Check |
|---------|--------|
| Server missing after edit | Full Cursor restart; valid JSON |
| command not found | Absolute path; `make build` ok |
| Tool always errors | **`cwd`** must contain **`index.html`** |
| curl SSE works, Cursor doesn’t | Use **`slyds mcp`** (stdio) for Cursor chat |

---

## Coding agents: Claude (Desktop & Claude Code)

Anthropic clients usually run MCP as **stdio** — **`slyds mcp`**.

### Claude Desktop (macOS)

Typical config path: `~/Library/Application Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "slyds": {
      "command": "/Users/you/projects/slyds/slyds",
      "args": ["mcp"]
    }
  }
}
```

**Restart Claude Desktop** after editing. **Windows** paths differ — see [Claude Desktop MCP](https://modelcontextprotocol.io); **`command` / `args`** shape is the same.

### Claude Code

Same stdio pattern: absolute path to **`slyds`**, **`args`**: `["mcp"]`. Placement of `mcpServers` (project vs global) depends on your Claude Code version — follow its docs. Replace **npx**/uv examples with the **`slyds`** path and **`["mcp"]`**.

### HTTP + SSE

Use **`slyds mcp serve`** only if you need the HTTP transport (e.g. matching a remote deployment). See [HTTP + SSE](#http--sse-remote-mcp-2024-11-05) below.

---

## Coding agents: GitHub Copilot / VS Code

**GitHub Copilot** in VS Code does **not** mirror **Cursor**’s MCP wiring by default; Microsoft’s MCP story varies by channel/version.

**Practical recommendation:** use **Cursor** or **Claude Desktop** for the simplest stdio setup (sections above).

If your **VS Code** build documents MCP with **`command`** / **`args`**:

```json
{
  "mcpServers": {
    "slyds": {
      "command": "/absolute/path/to/slyds",
      "args": ["mcp"]
    }
  }
}
```

Put the file where **your** VS Code / Copilot docs specify. For **URL-only** clients, run **`mcp serve`** and point at the SSE URL; see below.

**References:** [Model Context Protocol — transports](https://modelcontextprotocol.io/specification/2024-11-05/basic/transports); VS Code / Copilot release notes for “MCP”.

---

## Quick start — expose MCP with minimal setup

| Goal | Command | Notes |
|------|---------|--------|
| **Local / editor** (stdio) | `slyds mcp` | Client spawns this process; JSON-RPC over stdin/stdout with Content-Length framing. |
| **Same machine, URL-based testing** | `slyds mcp serve --listen 127.0.0.1:8787` | Open `GET http://127.0.0.1:8787/mcp/sse` — read the first SSE `endpoint` event for the exact POST URL (includes `session`). |
| **Behind HTTPS / remote clients** | `slyds mcp serve --listen 127.0.0.1:8787 --public-url https://your-host/mcp` | `--public-url` must be the **external** base clients use so the `endpoint` event’s POST URL is reachable. Add reverse proxy TLS in front. |
| **Shared secret** | Add `--token SECRET` | Clients send `Authorization: Bearer SECRET` on SSE and POST. |

Fastest path for **Glean**: deploy `slyds mcp serve` behind your ingress, set `--public-url`, configure the client with the SSE URL (`…/mcp/sse`), pass **`cwd`** in each `tools/call` (same as stdio).

## stdio (local)

```bash
slyds mcp
```

- JSON-RPC over stdin/stdout with **Content-Length** framing (common for editor-integrated clients).
- The client **spawns** this process; no HTTP.

## HTTP + SSE (remote, MCP 2024-11-05)

```bash
slyds mcp serve --listen 127.0.0.1:8787
```

- **GET** `<path-prefix>/sse` — `text/event-stream`. The server sends an **`endpoint`** SSE event whose JSON payload includes a **`url`** for POSTing JSON-RPC requests.
- **POST** that URL with query **`session=<id>`** — body is a single JSON-RPC object. Replies are delivered as SSE **`message`** events on the same GET connection (HTTP **202** on POST).

Defaults:

- `--path-prefix` `/mcp` → routes are `/mcp/sse` and `/mcp/message`.
- **Bind** `127.0.0.1:8787` — listen only on loopback unless you change `--listen`.

### Behind a reverse proxy (e.g. Glean)

Set **`--public-url`** to the **external** base the client must use (scheme + host + path prefix), so the `endpoint` event advertises a reachable POST URL:

```bash
slyds mcp serve --listen 127.0.0.1:8787 --public-url https://mcp.example.com/mcp
```

### Security

- **`--token`** — require `Authorization: Bearer <token>` on SSE and POST.
- **Origin** — by default, non-empty `Origin` must be `localhost` / `127.0.0.1` / loopback. Use **`--dangerous-allow-any-origin`** only for controlled tests (not production).
- Prefer TLS and authentication in front of the process when exposed beyond localhost.

### Tool arguments (remote)

Remote clients should pass **`cwd`** on each `tools/call` (same as stdio). Optionally set **`min_version`** to fail if the server binary is too old.

## Tests (maintainers)

Integration coverage lives in **`cmd/mcp_http_test.go`**:

- **`TestMCPHTTPSSEFullFlow`** — `httptest.Server`, `GET /mcp/sse` → `endpoint` event → `POST` advertised URL → `message` SSE event (validates the client-visible HTTP contract).
- Auth / **Origin**: unauthorized, forbidden origin, bearer token accepted.
- **`TestMCPMessagePOSTPushesSSE`** — POST handler + hub without full HTTP stack.

Run: `go test ./cmd/... -run MCP`

**Gotcha:** SSE requires **`http.Flusher`**; use a real server or client against **`httptest.Server`**, not `httptest.ResponseRecorder` alone for end-to-end SSE.
