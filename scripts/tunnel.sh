#!/usr/bin/env bash
# tunnel.sh — start a localhost tunnel for slyds MCP server.
# Detects ngrok or cloudflared, prints the public URL and ready-to-paste
# MCP config snippets for Claude Desktop, Claude Code, and Cursor.
#
# Usage:
#   bash scripts/tunnel.sh                     # no auth
#   SLYDS_MCP_TOKEN=secret bash scripts/tunnel.sh  # with auth
#
# Prerequisites:
#   brew install ngrok     # or https://ngrok.com/download
#   brew install cloudflared  # or https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/

set -euo pipefail

PORT="${SLYDS_MCP_PORT:-6274}"
TOKEN="${SLYDS_MCP_TOKEN:-}"

# --- Detect tunnel tool ---

if command -v ngrok &>/dev/null; then
    TOOL="ngrok"
elif command -v cloudflared &>/dev/null; then
    TOOL="cloudflared"
else
    echo "Error: neither ngrok nor cloudflared found on PATH."
    echo ""
    echo "Install one:"
    echo "  brew install ngrok        # https://ngrok.com"
    echo "  brew install cloudflared  # https://developers.cloudflare.com"
    exit 1
fi

echo "Starting $TOOL tunnel to 127.0.0.1:$PORT..."

# --- Start tunnel and extract URL ---

if [ "$TOOL" = "ngrok" ]; then
    ngrok http "$PORT" --log=stdout > /tmp/ngrok-slyds.log 2>&1 &
    TUNNEL_PID=$!
    # Wait for ngrok API to be ready.
    for i in $(seq 1 30); do
        URL=$(curl -s http://127.0.0.1:4040/api/tunnels 2>/dev/null \
            | python3 -c "import sys,json; tunnels=json.load(sys.stdin).get('tunnels',[]); print(tunnels[0]['public_url'] if tunnels else '')" 2>/dev/null || true)
        if [ -n "$URL" ]; then break; fi
        sleep 1
    done
    if [ -z "$URL" ]; then
        echo "Error: ngrok did not start within 30 seconds."
        kill $TUNNEL_PID 2>/dev/null || true
        exit 1
    fi
else
    # cloudflared prints the URL to stderr.
    cloudflared tunnel --url "http://127.0.0.1:$PORT" 2>&1 | tee /tmp/cloudflared-slyds.log &
    TUNNEL_PID=$!
    for i in $(seq 1 30); do
        URL=$(grep -o 'https://[^ ]*\.trycloudflare.com' /tmp/cloudflared-slyds.log 2>/dev/null | head -1 || true)
        if [ -n "$URL" ]; then break; fi
        sleep 1
    done
    if [ -z "$URL" ]; then
        echo "Error: cloudflared did not start within 30 seconds."
        kill $TUNNEL_PID 2>/dev/null || true
        exit 1
    fi
fi

# --- Print config snippets ---

echo ""
echo "============================================"
echo "Tunnel URL: $URL"
echo "============================================"
echo ""

if [ -n "$TOKEN" ]; then
    echo "Claude Desktop config (with auth):"
    cat <<SNIPPET
{
  "mcpServers": {
    "slyds": {
      "url": "$URL/mcp",
      "headers": {
        "Authorization": "Bearer $TOKEN"
      }
    }
  }
}
SNIPPET
else
    echo "Claude Desktop / Claude Code / Cursor config:"
    cat <<SNIPPET
{
  "mcpServers": {
    "slyds": {
      "url": "$URL/mcp"
    }
  }
}
SNIPPET
fi

echo ""
echo "Tunnel PID: $TUNNEL_PID (kill $TUNNEL_PID to stop)"
echo "Press Ctrl+C to stop."
wait $TUNNEL_PID
