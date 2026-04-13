# slyds MCP Server

[slyds](https://github.com/panyam/slyds) includes a [Model Context Protocol](https://modelcontextprotocol.io/) server that lets AI agents create, browse, edit, and build presentations. Powered by [mcpkit](https://github.com/panyam/mcpkit).

> **Quick setup:** See [AGENT-SETUP.md](../AGENT-SETUP.md) (agent-readable decision tree) or [docs/SETUP.md](SETUP.md) (human-readable guide with explanations and troubleshooting).

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

### Tools (11 + 2 preview)

Agents call tools to create, read, modify, and build decks. Each tool takes a `deck` parameter — the subdirectory name under `--deck-root`.

| Tool | Parameters | Description |
|------|-----------|-------------|
| `list_decks` | — | List all decks with name, title, theme, slide count |
| `create_deck` | `name`, `title`, `theme?`, `slides?` | Scaffold a new presentation |
| `describe_deck` | `deck` | Deck metadata: title, theme, slide list with layouts and word counts |
| `list_slides` | `deck` | Slide filenames, slugs, layouts, titles, word counts |
| `read_slide` | `deck`, `slide` or `position` | Raw HTML content of a slide. `slide` (preferred) accepts slug, filename, or position as string. |
| `edit_slide` | `deck`, `slide` or `position`, `content` | Replace a slide's HTML content. `slide` is stable across inserts; `position` is legacy. |
| `query_slide` | `deck`, `slide`, `selector`, ... | CSS selector read/write (goquery) — `slide` accepts slug, filename, or position |
| `add_slide` | `deck`, `position`, `name`, `layout?`, `title?` | Insert slide at position using layout template. Slug auto-suffixes on collision (`intro` → `intro-2`). |
| `remove_slide` | `deck`, `slide` | Remove slide by filename or position |
| `check_deck` | `deck` | Validate deck: missing files, broken includes, missing notes |
| `build_deck` | `deck` | Build self-contained HTML (resolves includes, inlines CSS/JS/images) |
| `preview_deck` | `deck`, `display_mode?` | Preview deck as MCP App iframe. `display_mode: "fullscreen"` for presentation mode. |
| `preview_slide` | `deck`, `position` | Preview deck opened on a specific slide |

### Resources (7 data + 2 preview)

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
| `ui://slyds/decks/{deck}/preview` | `text/html;profile=mcp-app` | Full navigable deck preview (MCP Apps) |
| `ui://slyds/decks/{deck}/slides/{position}/preview` | `text/html;profile=mcp-app` | Deck preview opened on a specific slide (MCP Apps) |

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

## Slide Identity

Slides have three overlapping identifiers in the MCP API:

| Reference | Stable across... | Use when |
|-----------|-----------------|----------|
| **Position** (`2`) | same session only | legacy/simple access |
| **Filename** (`02-metrics.html`) | content edits | you already have it from a previous response |
| **Slug** (`metrics`) | inserts, removes, moves | you want to re-reference after structural changes |
| **Slide ID** (`sl_a1b2c3d4`) | everything incl. renames | you want to cache the ID across tool calls |

Agents should prefer **slug** for references that survive insert/remove operations. The `slide` parameter on `read_slide`, `edit_slide`, `query_slide`, and `remove_slide` accepts any of the three forms — the server tries numeric → exact filename → exact slug → substring match.

**Slug uniqueness** is enforced within a deck. `add_slide` auto-suffixes colliding slugs with `-2`, `-3`, ... (e.g. inserting a second slide named `intro` produces `intro-2`). The `add_slide` response text reports the final slug when auto-suffixing occurs.

**Ambiguous references** return a clear error instead of silently picking the first match. If a substring or slug matches more than one slide, the response lists the candidates so the agent can retry with a specific filename.

Slug is **not rename-safe**: `slyds slides slugify` changes slugs based on `<h1>` headings. For rename-safe references, use **slide_id** — an opaque `sl_`-prefixed identifier assigned by slyds on slide creation, stored in `.slyds.yaml`, and returned in `describe_deck` / `list_slides` output. Slide IDs survive every mutation including renames.

**Legacy decks** (scaffolded before #83) start without slide IDs and get them auto-assigned on the first mutation. Until then, `describe_deck` returns `""` for `slide_id` — use slug or position instead.

## Server Configuration (mcpkit v0.1.31)

### Streaming Progress

`build_deck`, `check_deck`, `preview_deck`, and `preview_slide` emit progress chunks via `EmitContent` before the main operation runs (e.g., "Building deck 'q3-review'..."). Clients connected via SSE or Streamable HTTP with a content chunk handler see real-time progress; clients without chunk support see only the final result. The final result is authoritative — chunks are purely informational.

### Schema Validation

Server-side JSON Schema validation is active by default. Tool arguments are validated against the declared `InputSchema` before the handler runs. Malformed arguments (wrong type, missing required fields) produce a `-32602 Invalid Params` JSON-RPC error with structured error data. Agents can parse the error to identify which field needs correction without a handler round-trip.

### Per-Tool Timeouts

`build_deck` (30s) and `check_deck` (10s) have per-tool timeouts via `ToolDef.Timeout`. Other tools use the server default. This prevents long-running builds from being killed by a short global timeout while keeping fast tools responsive.

### Structured Results

Tools that return JSON (`list_decks`, `describe_deck`, `list_slides`, `check_deck`, `query_slide`, `create_deck`) use `StructuredResult` — the response carries both a human-readable text representation and a typed `structuredContent` field for programmatic access via `ToolCallTyped`.

### Error Handler

The server logs session lifecycle events (session expiry, keepalive failures) to stderr via `WithErrorHandler`. This provides visibility into agent disconnects and network issues in production deployments.

### EventStore (Streamable HTTP)

Streamable HTTP transport uses an in-memory EventStore (1000 events per stream) for reconnection support. If a client disconnects briefly, it can reconnect with `Last-Event-ID` and receive missed notifications.

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

Use **GitHub Copilot** in VS Code (recent stable; see [Add and manage MCP servers](https://code.visualstudio.com/docs/copilot/customization/mcp-servers)). Configuration lives in **`.vscode/mcp.json`** with a top-level **`servers`** object (not `mcpServers`).

**stdio (recommended)** — Copilot spawns `slyds` and talks over stdin/stdout:

```json
{
  "servers": {
    "slyds": {
      "type": "stdio",
      "command": "slyds",
      "args": ["mcp", "--stdio", "--deck-root", "/path/to/presentations"]
    }
  }
}
```

You can use `${workspaceFolder}` in `--deck-root` to point at a repo folder.

**MCP Apps** (inline `preview_deck` / `preview_slide` iframes): enable the experimental setting **`chat.mcp.apps.enabled`** (user or workspace `settings.json`). See [MCP Apps support in VS Code](https://code.visualstudio.com/blogs/2026/01/26/mcp-apps-support).

**HTTP (Streamable HTTP or SSE)** — run `slyds mcp` in a terminal yourself, then point VS Code at the MCP endpoint. You must pass **`--deck-root`** on that process (the editor no longer spawns the binary). Default transport is **Streamable HTTP**; use **`--sse`** for legacy SSE only. VS Code’s `type: "http"` tries Streamable HTTP first and falls back to SSE.

```json
{
  "servers": {
    "slyds": {
      "type": "http",
      "url": "http://127.0.0.1:6274/mcp"
    }
  }
}
```

Example (this repo’s `examples/` decks):

```bash
slyds mcp --listen 127.0.0.1:6274 --deck-root /path/to/slyds/examples
```

Use **MCP: Open Workspace Folder MCP Configuration** or **MCP: Add Server** from the Command Palette. Trust the server when prompted. Check [VS Code release notes](https://code.visualstudio.com/updates) if anything is missing.

---

## MCP Apps: Inline Slide Previews

Hosts that support the [MCP Apps extension](https://modelcontextprotocol.io/specification/2025-03-26/server/utilities/mcp-apps) render slyds previews as interactive iframes directly in the chat — no external browser needed.

### What you get

| Feature | How it works |
|---------|-------------|
| **Inline preview** | Agent calls `preview_deck` → host renders a navigable slide deck in an iframe |
| **Slide-specific preview** | Agent calls `preview_slide` with a position → deck opens on that slide |
| **Fullscreen mode** | `preview_deck` with `display_mode: "fullscreen"` → host switches to a fullscreen overlay (fixes the iframe sizing issue in constrained panels) |
| **Live updates** | Edit a slide → re-read the resource → preview reflects the change immediately (no tool re-call needed) |

### Host support

| Host | MCP Apps? | How to enable |
|------|-----------|---------------|
| **VS Code** (Copilot) | Yes | Enable `chat.mcp.apps.enabled` in settings.json |
| **Claude Desktop** | Yes | Built-in (no extra config) |
| **Claude Code** (CLI) | No | Text summary only — preview HTML not rendered |
| **Cursor** | Not yet | Text summary fallback |

### Try it

1. Set up slyds MCP in your host (see setup sections above)
2. Create a deck: *"Create a presentation about AI agents with 5 slides using the dark theme"*
3. Preview it: *"Preview the deck"* → agent calls `preview_deck` → slide deck appears inline
4. Navigate: use Prev/Next buttons in the iframe, or ask *"Show me slide 3"*
5. Go fullscreen: *"Preview the deck in fullscreen"* → agent calls `preview_deck` with `display_mode: "fullscreen"`

### Template resource URIs

Preview resources use parameterized URIs — each deck has its own resource:

| URI | Content |
|-----|---------|
| `ui://slyds/decks/{deck}/preview` | Full navigable presentation |
| `ui://slyds/decks/{deck}/slides/{position}/preview` | Presentation opened on a specific slide |

These are registered as MCP resource templates. Hosts that support `resources/read` with template matching can read them directly. The `TemplateHandler` extracts the deck name and slide position from the URI — no mutable server state involved.

### Display modes

Both preview tools declare `supportedDisplayModes: ["inline", "fullscreen"]` in their `_meta.ui` metadata. Hosts use this to:
- Render the deck inline in the chat panel (default)
- Offer a "fullscreen" button or honor `RequestDisplayMode` notifications
- Show picture-in-picture if supported (future)

The `display_mode` parameter on `preview_deck` triggers a `notifications/ui/displayMode` notification asking the host to switch. This is fire-and-forget — the host may ignore it if fullscreen is not supported.

### Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Preview shows text instead of iframe | Host doesn't support MCP Apps | Use a supported host, or `slyds build <deck>` for HTML |
| Iframe too small / slides clipped | Host panel is narrow | Use `display_mode: "fullscreen"`, or resize the panel. For VS Code, see iframe sizing below. |
| "No preview available" error | Resource read before tool call on old server | Update to latest slyds — template resources don't need prior tool calls |
| Slides not updating after edit | Stale resource cache in host | The server always builds fresh; if the host caches, close and re-read the resource |
| Agent sends local file paths as deck names | Agent guesses from workspace files instead of MCP discovery | Error message guides it to call `list_decks`. Start `--deck-root` at the directory containing your decks. |

### VS Code: iframe sizing

VS Code's MCP Apps iframe uses a fixed height (~300px) that's too short for presentations. To increase it:

1. Install the **[Custom CSS and JS Loader](https://marketplace.visualstudio.com/items?itemName=be5invis.vscode-custom-css)** extension
2. Add to your VS Code **user** settings.json (**not** workspace settings — Custom CSS patches VS Code's core UI, which requires user-level config):
   ```
   Cmd+Shift+P → "Preferences: Open User Settings (JSON)"
   ```
   ```json
   "vscode_custom_css.imports": ["file:///path/to/slyds/.vscode/mcp-apps.css"]
   ```
   (A ready-made CSS file is included at `.vscode/mcp-apps.css` in this repo — adjust the path to your clone)
3. Cmd+Shift+P → **"Enable Custom CSS and JS"** → restart VS Code when prompted

**Important**: Adding this to `.vscode/settings.json` (workspace settings) will **not** work — the Custom CSS extension only reads from user-level settings.

The CSS sets `.mcp-app-webview` to 600px — edit the file to adjust. This is a VS Code limitation; the MCP Apps spec doesn't yet support server-declared preferred sizes.

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

### With MCP Apps (inline previews)

On hosts that support MCP Apps (VS Code with Copilot, Claude Desktop):

```
Agent: "Preview the Q3 Review"
→ tools/call preview_deck {deck:"q3-review"}
← "Built deck 'Q3 Review' (8 slides, theme: corporate). Preview available at ui://slyds/decks/q3-review/preview"
→ [Host renders ui://slyds/decks/q3-review/preview as an iframe]
← [User sees a navigable slide deck inline in the chat]

Agent: "Show me just slide 5"
→ tools/call preview_slide {deck:"q3-review", position:5}
← "Preview of 'Q3 Review' opened at slide 5/8 (Projections)."
→ [Host renders ui://slyds/decks/q3-review/slides/5/preview]
← [Deck opens on slide 5; user can navigate forward/backward]

Agent: "Present it fullscreen"
→ tools/call preview_deck {deck:"q3-review", display_mode:"fullscreen"}
→ [Host receives notifications/ui/displayMode → switches to fullscreen]
← [Full-viewport slide deck with navigation]

User: [Edits slide 5 via agent]
→ tools/call edit_slide {deck:"q3-review", slide:"projections", content:"<div class='slide'>...new content...</div>"}
→ [Server sends notifications/resources/updated for ui://slyds/decks/q3-review/preview]
← [Host re-reads the resource → preview updates immediately]
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
