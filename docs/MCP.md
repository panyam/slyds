# slyds MCP Server

[slyds](https://github.com/panyam/slyds) includes a [Model Context Protocol](https://modelcontextprotocol.io/) server that lets AI agents create, browse, edit, and build presentations. Powered by [mcpkit](https://github.com/panyam/mcpkit).

---

## Getting Started

### 1. Install slyds

```bash
# Via go install (recommended — fetches the latest release)
go install github.com/panyam/slyds@latest

# Or from source
git clone https://github.com/panyam/slyds.git
cd slyds && make install

# Verify
slyds version
```

### 2. Create a presentations directory

```bash
mkdir ~/presentations
cd ~/presentations
slyds init "My First Talk" --theme dark -n 5
slyds init "Q3 Review" --theme corporate -n 8
```

### 3. Start the MCP server

```bash
slyds mcp --deck-root ~/presentations/
# MCP server (Streamable HTTP) on 127.0.0.1:6274 — deck root: /Users/you/presentations
```

### 4. Connect your agent

See setup for [Claude](#agent-setup-claude), [Cursor](#agent-setup-cursor), or [GitHub Copilot](#agent-setup-github-copilot--vs-code).

---

## What the Server Exposes

### Tools (10)

Agents call tools to create, read, modify, and build decks. Each tool takes a `deck` parameter — the subdirectory name under `--deck-root`.

| Tool | Parameters | Description |
|------|-----------|-------------|
| `create_deck` | `name`, `title`, `theme?`, `slides?` | Scaffold a new presentation |
| `describe_deck` | `deck` | Deck metadata: title, theme, slide list with layouts and word counts |
| `list_slides` | `deck` | Slide filenames, layouts, titles, word counts |
| `read_slide` | `deck`, `position` | Raw HTML content of a slide (1-based) |
| `edit_slide` | `deck`, `position`, `content` | Replace a slide's HTML content |
| `query_slide` | `deck`, `slide`, `selector`, ... | CSS selector read/write (goquery) — text, HTML, attrs, mutations |
| `add_slide` | `deck`, `position`, `name`, `layout?`, `title?` | Insert slide at position using layout template |
| `remove_slide` | `deck`, `slide` | Remove slide by filename or position |
| `check_deck` | `deck` | Validate deck: missing files, broken includes, missing notes |
| `build_deck` | `deck` | Build self-contained HTML (resolves includes, inlines CSS/JS/images) |

### Resources (7)

Agents browse resources to discover and read deck content — no mutations.

| URI | MIME Type | Content |
|-----|-----------|---------|
| `slyds://server/info` | `application/json` | Server version, available themes, available layouts |
| `slyds://decks` | `application/json` | List all decks (name, title, theme, slide count) |
| `slyds://decks/{name}` | `application/json` | Full deck metadata via `d.Describe()` |
| `slyds://decks/{name}/slides` | `application/json` | Slide list with position, filename, layout, title, words, notes |
| `slyds://decks/{name}/slides/{n}` | `text/html` | Raw slide HTML by position (1-based) |
| `slyds://decks/{name}/config` | `text/yaml` | `.slyds.yaml` manifest content |
| `slyds://decks/{name}/agent` | `text/markdown` | AGENT.md content (commands, layouts, hooks) |

### Available Themes & Layouts

**Themes**: default, dark, minimal, corporate, hacker

**Layouts**: title, content, two-col, section, blank, closing

---

## Server Flags

```
slyds mcp [flags]

--deck-root string    Root directory for deck discovery (default ".")
--listen string       Listen address (default "127.0.0.1:6274")
--token string        Bearer token for authentication
--public-url string   Public URL for reverse proxy deployments
--sse                 Use legacy HTTP+SSE transport instead of Streamable HTTP
--stdio               Use stdio transport (Content-Length framed JSON-RPC on stdin/stdout)
```

## Transports

| Transport | Flag | Port | Clients |
|-----------|------|------|---------|
| **Streamable HTTP** | (default) | 6274 | Any HTTP client, remote agents |
| **SSE** | `--sse` | 6274 | Legacy SSE clients |
| **stdio** | `--stdio` | — | Local editors (Cursor, Claude Desktop, VS Code) |

---

## Agent Setup: Claude

### Claude Desktop (macOS)

**Option A: stdio (recommended)** — Claude Desktop spawns slyds directly, no separate server:

1. Edit `~/Library/Application Support/Claude/claude_desktop_config.json`:
   ```json
   {
     "mcpServers": {
       "slyds": {
         "command": "slyds",
         "args": ["mcp", "--stdio", "--deck-root", "/Users/you/presentations"]
       }
     }
   }
   ```

2. Restart Claude Desktop. You should see "slyds" in Settings > MCP.

**Option B: HTTP** — run the server separately:

1. Start the server: `slyds mcp --deck-root ~/presentations/`
2. Edit `~/Library/Application Support/Claude/claude_desktop_config.json`:
   ```json
   {
     "mcpServers": {
       "slyds": {
         "url": "http://127.0.0.1:6274/mcp"
       }
     }
   }
   ```

3. Restart Claude Desktop.

Try it: *"List my presentations"* → agent calls `resources/read slyds://decks`

### Claude Code

**Option A: stdio (recommended)**:

```json
{
  "mcpServers": {
    "slyds": {
      "command": "slyds",
      "args": ["mcp", "--stdio", "--deck-root", "/path/to/presentations"]
    }
  }
}
```

**Option B: HTTP** — start the server separately (`slyds mcp --deck-root ~/presentations/`):

```json
{
  "mcpServers": {
    "slyds": {
      "url": "http://127.0.0.1:6274/mcp"
    }
  }
}
```

---

## Agent Setup: Cursor

**Option A: stdio (recommended)** — add to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "slyds": {
      "command": "slyds",
      "args": ["mcp", "--stdio", "--deck-root", "/path/to/presentations"]
    }
  }
}
```

**Option B: HTTP** — with the server running, add to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "slyds": {
      "url": "http://127.0.0.1:6274/mcp"
    }
  }
}
```

Restart Cursor. Check Settings > MCP for the "slyds" server.

Try it: *"Create a new presentation about AI agents with 5 slides using the dark theme"*

### Troubleshooting (Cursor)

| Symptom | Fix |
|---------|-----|
| Server not showing | Restart Cursor after editing mcp.json |
| Connection refused | Ensure `slyds mcp` is running in a terminal |
| Tools not listed | Check `--deck-root` points to a directory with decks |

---

## Agent Setup: GitHub Copilot / VS Code

MCP support in VS Code varies by version. If your build supports MCP with URL-based servers:

```json
{
  "mcpServers": {
    "slyds": {
      "url": "http://127.0.0.1:6274/mcp"
    }
  }
}
```

Check [VS Code release notes](https://code.visualstudio.com/updates) for current MCP support.

---

## Example Agent Workflow

Here's what happens when an agent helps you with a presentation:

```
Agent: "Show me my presentations"
→ resources/read slyds://decks
← [{"name":"q3-review","title":"Q3 Review","theme":"corporate","slides":8}, ...]

Agent: "What's in the Q3 Review?"
→ resources/read slyds://decks/q3-review
← {"title":"Q3 Review","theme":"corporate","slide_count":8,"slides":[...]}

Agent: "Show me slide 3"
→ resources/read slyds://decks/q3-review/slides/3
← <div class="slide" data-layout="content"><h1>Revenue</h1>...</div>

Agent: "Update the heading to 'Q3 Revenue Results'"
→ tools/call query_slide {deck:"q3-review", slide:"3", selector:"h1", set:"Q3 Revenue Results"}
← ["Q3 Revenue Results"]

Agent: "Add a new slide about projections after slide 3"
→ tools/call add_slide {deck:"q3-review", position:4, name:"projections", layout:"content", title:"Q4 Projections"}
← "Slide 'projections' inserted at position 4."

Agent: "Build it"
→ tools/call build_deck {deck:"q3-review"}
← <html>...(self-contained HTML)...</html>
```

---

## Deployment

### Local (development)

```bash
slyds mcp --deck-root ~/presentations/
```

### Behind HTTPS / reverse proxy

```bash
slyds mcp --listen 127.0.0.1:6274 \
  --public-url https://mcp.example.com/mcp \
  --token YOUR_SECRET
```

Clients send `Authorization: Bearer YOUR_SECRET`. The `--public-url` ensures the server advertises reachable URLs for SSE clients.

### Security

- **`--token`**: require Bearer token on all requests
- **Origin checks**: non-localhost origins are rejected by default (DNS rebinding protection)
- Always use TLS when exposing beyond localhost

---

## Testing

```bash
make test      # All tests (includes MCP unit + e2e)
make e2e       # MCP e2e tests only (full agent workflow via httptest)
```

### Manual testing with curl

```bash
# Start server
slyds mcp --deck-root examples/

# Initialize session
RESP=$(curl -si -X POST http://127.0.0.1:6274/mcp \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"curl","version":"1"}}}')
SESSION=$(echo "$RESP" | grep -i mcp-session-id | awk '{print $2}' | tr -d '\r')

# Send initialized notification
curl -s -X POST http://127.0.0.1:6274/mcp \
  -H "Mcp-Session-Id: $SESSION" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}'

# List decks
curl -s -X POST http://127.0.0.1:6274/mcp \
  -H "Mcp-Session-Id: $SESSION" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"slyds://decks"}}' | python3 -m json.tool

# Call a tool
curl -s -X POST http://127.0.0.1:6274/mcp \
  -H "Mcp-Session-Id: $SESSION" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"describe_deck","arguments":{"deck":"slyds-intro"}}}' | python3 -m json.tool
```
