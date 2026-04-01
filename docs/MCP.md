# slyds MCP transports

`slyds` exposes a thin [Model Context Protocol](https://modelcontextprotocol.io/) server with one tool, `slyds`, that runs the CLI in a working directory. Two transports are supported.

## Quick start — expose MCP with minimal setup

| Goal | Command | Notes |
|------|---------|--------|
| **Local / editor** (stdio) | `slyds mcp` | Client spawns this process; JSON-RPC over stdin/stdout with Content-Length framing. |
| **Same machine, URL-based testing** | `slyds mcp serve --listen 127.0.0.1:8787` | Open `GET http://127.0.0.1:8787/mcp/sse` — read the first SSE `endpoint` event for the exact POST URL (includes `session`). |
| **Behind HTTPS / remote clients** | `slyds mcp serve --listen 127.0.0.1:8787 --public-url https://your-host/mcp` | `--public-url` must be the **external** base clients use so the `endpoint` event’s POST URL is reachable. Add reverse proxy TLS in front. |
| **Shared secret** | Add `--token SECRET` | Clients send `Authorization: Bearer SECRET` on SSE and POST. |

Fastest path for **Glean**: deploy or run `slyds mcp serve` behind your ingress, set `--public-url` to the public MCP base path, configure the client with the SSE URL (`…/mcp/sse`), and pass **`cwd`** in each `tools/call` (same as stdio).

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

### Tool arguments

Remote clients should pass **`cwd`** on each `tools/call` (same as stdio). Optionally set **`min_version`** to fail if the server binary is too old.

## Tests (maintainers)

Integration coverage lives in **`cmd/mcp_http_test.go`**:

- **`TestMCPHTTPSSEFullFlow`** — `httptest.Server`, `GET /mcp/sse` → `endpoint` event → `POST` advertised URL → `message` SSE event (validates the client-visible HTTP contract).
- Auth / **Origin**: unauthorized, forbidden origin, bearer token accepted.
- **`TestMCPMessagePOSTPushesSSE`** — POST handler + hub without full HTTP stack.

Run: `go test ./cmd/... -run MCP`

**Gotcha:** SSE requires **`http.Flusher`**; use a real server or client against **`httptest.Server`**, not `httptest.ResponseRecorder` alone for end-to-end SSE.
