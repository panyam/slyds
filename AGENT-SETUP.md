# slyds Setup

## 1. Install

```bash
go install github.com/panyam/slyds@latest
slyds version   # verify
```

From source: `git clone https://github.com/panyam/slyds.git && cd slyds && make install`

## 2. Create decks

```bash
slyds init "My Talk" --theme dark -n 5
# Creates my-talk/ with 5 slides
```

Or scaffold demo decks: `make demo` (creates 3 decks in `/tmp/slyds-demo/`).

## 3. Pick mode

### CLI-direct (no MCP, lower context)

Use when: agent has shell access, simple tasks, tight context budget.

```bash
slyds describe my-talk --json     # deck metadata
slyds ls my-talk --json           # slide list
slyds query my-talk 1 "h1"       # read content via CSS selector
slyds query my-talk 1 "h1" --set "New Title"  # write content
slyds add my-talk 4 --name extra --layout content  # insert slide
slyds check my-talk --json        # validate deck
slyds build my-talk --json        # build self-contained HTML (stdout)
slyds introspect                  # full command/theme/layout catalog (JSON)
```

Discovery: run `slyds introspect` or read `AGENT.md` in the deck directory.

Skip to **step 6** (verify).

### MCP (protocol-based)

Use when: need resource browsing, multi-turn sessions, remote access, or host requires MCP.

Continue to **step 4**.

## 4. Pick transport (MCP mode)

### Stdio (recommended for local editors)

No server to start. Editor spawns slyds directly.

**Claude Desktop** (`~/Library/Application Support/Claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "slyds": {
      "command": "slyds",
      "args": ["mcp", "--stdio", "--deck-root", "<PATH_TO_DECKS>"]
    }
  }
}
```

**Claude Code** (`.mcp.json`):
```json
{
  "mcpServers": {
    "slyds": {
      "command": "slyds",
      "args": ["mcp", "--stdio", "--deck-root", "<PATH_TO_DECKS>"]
    }
  }
}
```

**Cursor** (`.cursor/mcp.json`):
```json
{
  "mcpServers": {
    "slyds": {
      "command": "slyds",
      "args": ["mcp", "--stdio", "--deck-root", "<PATH_TO_DECKS>"]
    }
  }
}
```

### HTTP (for localhost or remote access)

Start server separately:
```bash
slyds mcp --deck-root <PATH_TO_DECKS>
# Streamable HTTP on 127.0.0.1:6274
```

Config for all editors:
```json
{
  "mcpServers": {
    "slyds": {
      "url": "http://127.0.0.1:6274/mcp"
    }
  }
}
```

### Remote (tunnel)

```bash
slyds mcp --deck-root <PATH_TO_DECKS> &
make tunnel   # or: bash scripts/tunnel.sh
# Prints public HTTPS URL + config snippet
```

## 5. Auth (if needed)

**Stdio with token:**
```json
{
  "mcpServers": {
    "slyds": {
      "command": "slyds",
      "args": ["mcp", "--stdio", "--deck-root", "<PATH>", "--token", "<SECRET>"]
    }
  }
}
```

**HTTP with token:**
```json
{
  "mcpServers": {
    "slyds": {
      "url": "http://127.0.0.1:6274/mcp",
      "headers": {
        "Authorization": "Bearer <SECRET>"
      }
    }
  }
}
```

**Environment variable** (for containers/CI): `SLYDS_MCP_TOKEN=secret slyds mcp ...`

## 6. Verify

**CLI-direct:** `slyds describe <deck> --json` — should print deck metadata as JSON.

**MCP:** Ask your agent "List my presentations" — should return deck names.

**curl (HTTP):**
```bash
curl -s -X POST http://127.0.0.1:6274/mcp \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"curl","version":"1"}}}'
```

## CLI command reference

| Command | Description | `--json` |
|---------|-------------|----------|
| `slyds init "Title" [-n N] [--theme T]` | Scaffold new deck | N/A |
| `slyds describe <deck>` | Deck metadata | Yes |
| `slyds ls <deck>` | List slides | Yes |
| `slyds query <deck> <slide> <sel>` | CSS selector read/write | `--batch` |
| `slyds check <deck>` | Validate deck | Yes |
| `slyds build <deck>` | Build self-contained HTML | Yes |
| `slyds add <deck> <pos> --name N` | Insert slide | N/A |
| `slyds introspect` | Full catalog (JSON) | Always JSON |

## MCP tools (10)

`create_deck`, `describe_deck`, `list_slides`, `read_slide`, `edit_slide`, `query_slide`, `add_slide`, `remove_slide`, `check_deck`, `build_deck`

## MCP resources (7)

`slyds://server/info`, `slyds://decks`, `slyds://decks/{name}`, `slyds://decks/{name}/slides`, `slyds://decks/{name}/slides/{n}`, `slyds://decks/{name}/config`, `slyds://decks/{name}/agent`
