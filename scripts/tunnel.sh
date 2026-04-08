#!/usr/bin/env bash
# tunnel.sh — start a localhost tunnel for slyds MCP server.
#
# Usage:
#   make tunnel                                          # ngrok, ephemeral URL
#   NGROK_DOMAIN=your-name.ngrok-free.app make tunnel    # ngrok, stable URL
#   TOOL=cf make tunnel                                  # cloudflared
#   SLYDS_MCP_TOKEN=secret make tunnel                   # with auth
#
# TOOL (default: ngrok):
#   ngrok  — requires ngrok on PATH (brew install ngrok)
#   cf     — requires cloudflared on PATH (brew install cloudflared)
#
# NGROK_DOMAIN (optional):
#   Set to your free static domain from https://dashboard.ngrok.com/domains
#   to get a stable URL that doesn't change between restarts.

set -euo pipefail

PORT="${SLYDS_MCP_PORT:-6274}"
TOKEN="${SLYDS_MCP_TOKEN:-}"
TOOL="${TOOL:-ngrok}"
NGROK_DOMAIN="${NGROK_DOMAIN:-}"

echo "Starting $TOOL tunnel to 127.0.0.1:$PORT..."

# --- Start tunnel and extract URL ---

if [ "$TOOL" = "ngrok" ]; then
    command -v ngrok &>/dev/null || { echo "Error: ngrok not found on PATH. Install: brew install ngrok"; exit 1; }
    NGROK_ARGS="http $PORT"
    if [ -n "$NGROK_DOMAIN" ]; then
        NGROK_ARGS="http --url $NGROK_DOMAIN $PORT"
        echo "Using static domain: $NGROK_DOMAIN"
    fi
    ngrok $NGROK_ARGS --log=stdout > /tmp/ngrok-slyds.log 2>&1 &
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
elif [ "$TOOL" = "cf" ]; then
    command -v cloudflared &>/dev/null || { echo "Error: cloudflared not found on PATH. Install: brew install cloudflared"; exit 1; }
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
else
    echo "Error: unknown TOOL=$TOOL. Use 'ngrok' or 'cf'."
    exit 1
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
