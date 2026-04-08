# slyds MCP Setup Guide

This guide explains how to set up slyds for use with AI agents. For the quick-start decision tree, see [AGENT-SETUP.md](../AGENT-SETUP.md).

## Two modes of operation

### CLI-direct

Agents run slyds commands via shell — no MCP server, no protocol overhead. Best for agents with shell access doing simple tasks (create, edit, build). Lower context usage since there's no MCP handshake or session management.

Key commands: `slyds describe --json`, `slyds ls --json`, `slyds query`, `slyds check --json`, `slyds build --json`, `slyds introspect`.

### MCP (Model Context Protocol)

Full protocol server with 10 tools and 7 browsable resources. Best for multi-turn sessions, resource browsing, and hosts that require MCP (Claude Desktop native integration).

## Transports

slyds MCP supports three transports:

| Transport | Flag | When to use |
|-----------|------|-------------|
| **Stdio** | `--stdio` | Local editors (Claude Desktop, Claude Code, Cursor). Editor spawns slyds directly. No port conflicts, no server to manage. |
| **Streamable HTTP** | (default) | Multi-client, remote access, curl testing. Runs on `:6274`. |
| **SSE** | `--sse` | Legacy clients that don't support Streamable HTTP. Same port. |

**Stdio** is recommended for local development. The editor spawns `slyds mcp --stdio` as a subprocess — no separate server to start, no port to remember.

**HTTP** is needed when the agent connects over the network (remote LLM, tunnel, production deployment).

**SSE** exists for backward compatibility with older MCP clients.

## Configurations

### Dev (localhost)

All three transports, all testable locally.

```bash
# Scaffold demo decks
make demo

# Streamable HTTP
make dev-http          # http://127.0.0.1:6274/mcp

# SSE
make dev-sse           # http://127.0.0.1:6274/sse + /message

# Stdio (pipe testing)
make dev-stdio         # reads JSON-RPC from stdin

# With bearer auth
make dev-http-auth     # --token dev-secret
make dev-sse-auth      # --token dev-secret
```

### Remote (tunnel)

Expose your local server via HTTPS for LLM hosts that can't reach localhost.

```bash
# Start server + tunnel
make dev-http &
make tunnel
# Prints public URL + config snippets
```

**Prerequisites:** Install [ngrok](https://ngrok.com) (`brew install ngrok`) or [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/) (`brew install cloudflared`).

ngrok requires a free account. cloudflared works without an account via `trycloudflare.com` (ephemeral URLs).

### Production

See issue #62 for future Docker + Fly.io deployment support (not yet implemented).

## Authentication

### `--token` flag

Static bearer token. Clients send `Authorization: Bearer <token>`.

```bash
slyds mcp --token my-secret --deck-root ~/presentations
```

### `SLYDS_MCP_TOKEN` environment variable

Fallback when `--token` is not set. Useful for containers and CI:

```bash
SLYDS_MCP_TOKEN=secret slyds mcp --deck-root ~/presentations
```

### Stdio and auth

For stdio, the `--token` flag is typically unnecessary — the process boundary is the trust boundary. But it can be set if the server also serves HTTP on a separate goroutine.

### Future: JWT/OAuth

See [issue #63](https://github.com/panyam/slyds/issues/63) for planned JWT/OAuth support via mcpkit's `ext/auth` module.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `command not found: slyds` | Run `go install github.com/panyam/slyds@latest` or `make install` |
| `Connection refused` | Ensure `slyds mcp` is running. Check port with `lsof -i :6274`. |
| Tools not showing in Claude | Restart Claude Desktop/Code after editing config. Check `slyds version` matches. |
| `401 Unauthorized` | Add `Authorization: Bearer <token>` header, or remove `--token` from server. |
| Tunnel URL not working | Ensure local server is running before starting tunnel. Check ngrok dashboard at `http://127.0.0.1:4040`. |
| `--json` not recognized | Update slyds: `go install github.com/panyam/slyds@latest` |
| Decks not found | Check `--deck-root` points to a directory containing deck subdirectories with `index.html`. |
