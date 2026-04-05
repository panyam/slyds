# slyds MCP (Model Context Protocol)

[slyds](https://github.com/panyam/slyds) exposes a full [Model Context Protocol](https://modelcontextprotocol.io/) server with **semantic tools** for deck operations and **browsable resources** for reading deck content. Powered by [mcpkit](https://github.com/panyam/mcpkit).

**Build the binary** from the repo (`make build`) and use the **absolute path** to `./slyds` in configs below.

## Tools

Agents call tools to create, read, modify, and build decks. Each tool takes a `deck` parameter (subdirectory name under `--deck-root`).

| Tool | Description |
|------|-------------|
| `create_deck` | Scaffold a new deck (name, title, theme, slides) |
| `describe_deck` | Deck metadata: title, theme, slide list with layouts/word counts |
| `list_slides` | Slide filenames, layouts, titles, word counts |
| `read_slide` | Raw HTML content by position |
| `edit_slide` | Replace slide HTML content |
| `query_slide` | CSS selector read/write (goquery) |
| `add_slide` | Insert slide at position with layout template |
| `remove_slide` | Remove slide by name or position |
| `check_deck` | Validate deck (missing files, broken includes, etc.) |
| `build_deck` | Build self-contained HTML |

## Resources

Agents browse resources to discover and read deck content without mutations.

| URI | Content |
|-----|---------|
| `slyds://server/info` | Server version, available themes and layouts |
| `slyds://decks` | List all decks under deck root |
| `slyds://decks/{name}` | Deck metadata (title, theme, slides with layouts) |
| `slyds://decks/{name}/slides` | Slide list with metadata |
| `slyds://decks/{name}/slides/{n}` | Raw slide HTML by position |
| `slyds://decks/{name}/config` | .slyds.yaml content |
| `slyds://decks/{name}/agent` | AGENT.md content |

## Transports

| Transport | Command | Typical clients |
|-----------|---------|-----------------|
| **Streamable HTTP** (default) | `slyds mcp` | Remote agents, curl, any HTTP client |
| **SSE** | `slyds mcp --sse` | Legacy SSE clients |
| **stdio** | `slyds mcp --stdio` | Cursor, Claude Desktop (not yet implemented — requires mcpkit#3) |

## Server flags

```bash
slyds mcp [flags]

--deck-root string    Root directory for deck discovery (default ".")
--listen string       Listen address (default "127.0.0.1:6274")
--token string        Bearer token for authentication
--public-url string   Public URL for reverse proxy
--sse                 Use legacy HTTP+SSE transport
--stdio               Use stdio transport (not yet implemented)
```

## Example agent workflow

```
1. resources/read slyds://decks         → discover available decks
2. resources/read slyds://decks/q3      → get deck metadata (12 slides, corporate theme)
3. resources/read slyds://decks/q3/slides/2 → read slide 2 HTML
4. tools/call edit_slide {deck: "q3", position: 2, content: "..."} → update slide
5. tools/call build_deck {deck: "q3"}   → build self-contained HTML
```

## Quick start

```bash
# Start server pointing at a directory of decks
slyds mcp --deck-root ~/presentations/

# Test with curl (Streamable HTTP)
curl -s -X POST http://127.0.0.1:6274/mcp \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}'
```

## Client setup

### Cursor / Claude Desktop (stdio — when available)

```json
{
  "mcpServers": {
    "slyds": {
      "command": "/path/to/slyds",
      "args": ["mcp", "--deck-root", "/path/to/presentations"]
    }
  }
}
```

Note: stdio transport requires mcpkit#3. Until then, use Streamable HTTP:

### Cursor / Claude Desktop (Streamable HTTP)

1. Start: `slyds mcp --deck-root ~/presentations/ --listen 127.0.0.1:6274`
2. Configure client with URL `http://127.0.0.1:6274/mcp`

### Behind HTTPS / reverse proxy

```bash
slyds mcp --listen 127.0.0.1:6274 --public-url https://mcp.example.com/mcp --token SECRET
```

Clients send `Authorization: Bearer SECRET`. The `--public-url` ensures SSE endpoint events advertise a reachable URL.

## Tests

```bash
go test ./cmd/... -run 'Tool|Resource|Discover|OpenDeck'
```

15 tests covering: tool handlers (create, describe, list, read, edit, query, add, remove, check, build), resource helpers (deck discovery, open), and JSON round-trip.
