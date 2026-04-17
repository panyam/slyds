# MCP Auth Testing Guide

Manual test flows for slyds MCP auth. Covers static tokens, JWT/Keycloak, scoped access, and browser OAuth from VS Code.

## Prerequisites

```bash
make build
make demo          # scaffold demo decks in /tmp/slyds-demo/
make upkcl         # start Keycloak (wait ~30s for realm import)
```

Verify Keycloak is ready:
```bash
curl -s http://localhost:8180/realms/slyds-test | python3 -c "import sys,json; print(json.load(sys.stdin)['realm'])"
# Should print: slyds-test
```

---

## Flow 1: No Auth (backward compat)

```bash
slyds mcp --deck-root /tmp/slyds-demo/
```

Test: tools work without any token.
```bash
curl -s -X POST http://127.0.0.1:8274/mcp \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"curl","version":"1"}}}' | python3 -m json.tool
```
Expected: 200 with server info.

---

## Flow 2: Static Bearer Token

```bash
slyds mcp --deck-root /tmp/slyds-demo/ --token mysecret
```

### 2a: No token → 401
```bash
curl -s -i -X POST http://127.0.0.1:8274/mcp \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"curl","version":"1"}}}'
```
Expected: `401 Unauthorized`.

### 2b: Wrong token → 401
```bash
curl -s -i -X POST http://127.0.0.1:8274/mcp \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer wrongtoken' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"curl","version":"1"}}}'
```
Expected: `401 Unauthorized`.

### 2c: Correct token → 200
```bash
curl -s -X POST http://127.0.0.1:8274/mcp \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer mysecret' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"curl","version":"1"}}}' | python3 -m json.tool
```
Expected: 200 with server info.

---

## Flow 3: JWT + Keycloak (curl)

```bash
slyds mcp --deck-root /tmp/slyds-demo/ \
  --jwks-url http://localhost:8180/realms/slyds-test/protocol/openid-connect/certs \
  --issuer http://localhost:8180/realms/slyds-test \
  --verbose
```

### 3a: No token → 401 with PRM discovery
```bash
curl -s -D - -o /dev/null -X POST http://127.0.0.1:8274/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{}'
```
Expected: `401` with `WWW-Authenticate: Bearer resource_metadata="http://127.0.0.1:8274/.well-known/oauth-protected-resource/mcp"`.

### 3b: PRM endpoint returns Keycloak as auth server
```bash
curl -s http://127.0.0.1:8274/.well-known/oauth-protected-resource/mcp | python3 -m json.tool
```
Expected:
```json
{
    "resource": "http://127.0.0.1:8274",
    "authorization_servers": ["http://localhost:8180/realms/slyds-test"]
}
```

### 3c: RFC 8414 proxy serves Keycloak's AS metadata
```bash
curl -s http://127.0.0.1:8274/.well-known/oauth-authorization-server/realms/slyds-test | python3 -m json.tool
```
Expected: JSON with `issuer`, `authorization_endpoint`, `token_endpoint`, `jwks_uri` from Keycloak.

### 3d: Get client_credentials token (read+write)
```bash
TOKEN=$(curl -s -X POST http://localhost:8180/realms/slyds-test/protocol/openid-connect/token \
  -d "grant_type=client_credentials" \
  -d "client_id=slyds-confidential" \
  -d "client_secret=slyds-test-secret" \
  -d "scope=slyds-read slyds-write" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")
echo "Token: ${TOKEN:0:20}..."
```

### 3e: Initialize + list decks with token
```bash
curl -s -X POST http://127.0.0.1:8274/mcp \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"curl","version":"1"}}}' | python3 -m json.tool

curl -s -X POST http://127.0.0.1:8274/mcp \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}'

curl -s -X POST http://127.0.0.1:8274/mcp \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_decks","arguments":{}}}' | python3 -m json.tool
```
Expected: deck list returned.

---

## Flow 4: Scoped Access (read-only vs read-write)

### 4a: Read-only token — reads work, mutations blocked
```bash
ROTOKEN=$(curl -s -X POST http://localhost:8180/realms/slyds-test/protocol/openid-connect/token \
  -d "grant_type=client_credentials" \
  -d "client_id=slyds-confidential" \
  -d "client_secret=slyds-test-secret" \
  -d "scope=slyds-read" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

# Initialize
curl -s -X POST http://127.0.0.1:8274/mcp \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $ROTOKEN" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"curl","version":"1"}}}' > /dev/null

curl -s -X POST http://127.0.0.1:8274/mcp \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $ROTOKEN" \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}'

# list_decks works (read)
curl -s -X POST http://127.0.0.1:8274/mcp \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $ROTOKEN" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_decks","arguments":{}}}' | python3 -m json.tool
```
Expected: deck list returned.

```bash
# create_deck BLOCKED (write)
curl -s -X POST http://127.0.0.1:8274/mcp \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $ROTOKEN" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"create_deck","arguments":{"name":"blocked","title":"Should Fail","theme":"default"}}}' | python3 -m json.tool
```
Expected: error about "insufficient scope".

### 4b: Read-write token — mutations work
```bash
# Use $TOKEN from Flow 3d (has slyds-read + slyds-write)
curl -s -X POST http://127.0.0.1:8274/mcp \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"create_deck","arguments":{"name":"auth-test","title":"Auth Works","theme":"default"}}}' | python3 -m json.tool
```
Expected: deck created successfully.

---

## Flow 5: User Token (password grant)

```bash
UTOKEN=$(curl -s -X POST http://localhost:8180/realms/slyds-test/protocol/openid-connect/token \
  -d "grant_type=password" \
  -d "client_id=slyds-confidential" \
  -d "client_secret=slyds-test-secret" \
  -d "username=slyds-testuser" \
  -d "password=testpassword" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

curl -s -X POST http://127.0.0.1:8274/mcp \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $UTOKEN" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"curl","version":"1"}}}' | python3 -m json.tool
```
Expected: 200 — user token accepted.

---

## Flow 6: VS Code Browser OAuth (PKCE)

1. Start slyds with Keycloak auth:
   ```bash
   slyds mcp --deck-root /tmp/slyds-demo/ \
     --jwks-url http://localhost:8180/realms/slyds-test/protocol/openid-connect/certs \
     --issuer http://localhost:8180/realms/slyds-test \
     --verbose
   ```

2. In VS Code, configure `.vscode/mcp.json`:
   ```json
   {
     "servers": {
       "slyds": {
         "type": "http",
         "url": "http://127.0.0.1:8274/mcp"
       }
     }
   }
   ```
   (No `Authorization` header — VS Code will discover auth automatically.)

3. Open Copilot Chat → ask "list my decks"

4. VS Code shows dialog: **"Dynamic Client Registration not supported"**
   - Click **"Copy URIs & Proceed"**
   - Enter client ID: **`slyds-public`**
   - Leave client secret **blank** (press Enter)

5. Browser opens to Keycloak login page
   - Username: **`slyds-testuser`**
   - Password: **`testpassword`**

6. After login, Keycloak redirects back to VS Code
   - VS Code exchanges auth code for JWT via PKCE
   - Tools start working automatically

7. Verify: ask Copilot "list my decks" — should return the demo decks.

---

## Flow 7: Proto Path (same auth)

All flows above work identically with `slyds mcp-proto`:
```bash
slyds mcp-proto --deck-root /tmp/slyds-demo/ \
  --jwks-url http://localhost:8180/realms/slyds-test/protocol/openid-connect/certs \
  --issuer http://localhost:8180/realms/slyds-test
```

---

## Cleanup

```bash
make downkcl    # stop Keycloak
```

---

## Keycloak Realm Details

| Item | Value |
|------|-------|
| Realm | `slyds-test` |
| Confidential client | `slyds-confidential` / secret: `slyds-test-secret` |
| Public client (PKCE) | `slyds-public` (no secret) |
| Test user | `slyds-testuser` / `testpassword` |
| Scopes | `slyds-read`, `slyds-write`, `offline_access` |
| Config file | `tests/keycloak/realm.json` |

## Automated Tests

```bash
make test         # unit + e2e (auth tests use mock validator, no Keycloak)
make testkcl      # Keycloak interop tests (requires make upkcl first)
make testall      # everything including Keycloak (auto-starts/stops)
```
