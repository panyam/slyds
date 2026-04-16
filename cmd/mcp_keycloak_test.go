package cmd

// Keycloak interop tests for slyds MCP auth. Validates that slyds correctly
// validates Keycloak-issued JWTs, enforces scopes on mutation tools, and
// serves the PRM endpoint with the correct authorization server.
//
// Prerequisites:
//   - Keycloak running at localhost:8180 (or KEYCLOAK_URL env var)
//   - Realm "slyds-test" imported from tests/keycloak/realm.json
//   - Run: make upkcl  (starts Keycloak with realm auto-import)
//   - Run: make testkcl (runs these tests)
//
// Tests skip gracefully when Keycloak is not reachable.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panyam/mcpkit/client"
	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/ext/auth"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/slyds/core"
)

const (
	defaultKCURL  = "http://localhost:8180"
	kcRealm       = "slyds-test"
	kcClientID    = "slyds-confidential"
	kcClientSecret = "slyds-test-secret"
	kcTestUser    = "slyds-testuser"
	kcTestPass    = "testpassword"
	kcScopeRead   = "slyds-read"
	kcScopeWrite  = "slyds-write"
)

func kcURL() string {
	if u := os.Getenv("KEYCLOAK_URL"); u != "" {
		return u
	}
	return defaultKCURL
}

func kcRealmURL() string { return kcURL() + "/realms/" + kcRealm }

func skipIfKCDown(t *testing.T) {
	t.Helper()
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(kcRealmURL())
	if err != nil {
		t.Skipf("Keycloak not reachable at %s (run 'make upkcl' to start)", kcURL())
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Skipf("Keycloak realm %s not ready (status %d)", kcRealm, resp.StatusCode)
	}
}

// kcOIDC holds discovered OIDC endpoints.
type kcOIDC struct {
	Issuer        string `json:"issuer"`
	TokenEndpoint string `json:"token_endpoint"`
	JWKSURI       string `json:"jwks_uri"`
}

func discoverKCOIDC(t *testing.T) kcOIDC {
	t.Helper()
	resp, err := http.Get(kcRealmURL() + "/.well-known/openid-configuration")
	if err != nil {
		t.Fatalf("OIDC discovery: %v", err)
	}
	defer resp.Body.Close()
	var cfg kcOIDC
	json.NewDecoder(resp.Body).Decode(&cfg)
	return cfg
}

func getKCToken(t *testing.T, tokenEndpoint string, scopes ...string) string {
	t.Helper()
	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {kcClientID},
		"client_secret": {kcClientSecret},
	}
	if len(scopes) > 0 {
		data.Set("scope", strings.Join(scopes, " "))
	}
	resp, err := http.PostForm(tokenEndpoint, data)
	if err != nil {
		t.Fatalf("token request: %v", err)
	}
	defer resp.Body.Close()
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	json.NewDecoder(resp.Body).Decode(&tok)
	if tok.AccessToken == "" {
		t.Fatal("empty access token from Keycloak")
	}
	return tok.AccessToken
}

func getUserToken(t *testing.T, tokenEndpoint string) string {
	t.Helper()
	data := url.Values{
		"grant_type":    {"password"},
		"client_id":     {kcClientID},
		"client_secret": {kcClientSecret},
		"username":      {kcTestUser},
		"password":      {kcTestPass},
	}
	resp, err := http.PostForm(tokenEndpoint, data)
	if err != nil {
		t.Fatalf("password token: %v", err)
	}
	defer resp.Body.Close()
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	json.NewDecoder(resp.Body).Decode(&tok)
	if tok.AccessToken == "" {
		t.Fatal("empty user token from Keycloak")
	}
	return tok.AccessToken
}

// newKCSlydsEnv creates a slyds MCP server with Keycloak JWT validation.
type kcSlydsEnv struct {
	Server *httptest.Server
	OIDC   kcOIDC
}

func newKCSlydsEnv(t *testing.T) *kcSlydsEnv {
	t.Helper()
	cfg := discoverKCOIDC(t)

	root := t.TempDir()
	core.CreateInDir("KC Test Deck", 3, "default", filepath.Join(root, "test-deck"), true)

	var handler http.Handler
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handler == nil {
			http.Error(w, "not ready", 503)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)

	validator := auth.NewJWTValidator(auth.JWTConfig{
		JWKSURL:             cfg.JWKSURI,
		Issuer:              cfg.Issuer,
		ResourceMetadataURL: ts.URL + "/.well-known/oauth-protected-resource/mcp",
		AllScopes:           []string{kcScopeRead, kcScopeWrite},
	})
	validator.Start()
	t.Cleanup(validator.Stop)

	ws, err := NewLocalWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}

	srv := server.NewServer(
		mcpcore.ServerInfo{Name: "slyds-kc-test", Version: "0.0.1"},
		server.WithAuth(validator),
		server.WithMiddleware(workspaceMiddleware(ws)),
	)
	registerTools(srv)
	registerResources(srv)
	registerPrompts(srv)

	mux := http.NewServeMux()
	mux.Handle("/mcp", srv.Handler(server.WithStreamableHTTP(true)))
	auth.MountAuth(mux, auth.AuthConfig{
		ResourceURI:          ts.URL,
		AuthorizationServers: []string{cfg.Issuer},
		ScopesSupported:      []string{kcScopeRead, kcScopeWrite},
		MCPPath:              "/mcp",
		Validator:            validator,
	})
	handler = mux

	return &kcSlydsEnv{Server: ts, OIDC: cfg}
}

// --- Keycloak interop tests ---

// TestKC_ValidToken verifies a Keycloak-issued JWT is accepted by slyds.
func TestKC_ValidToken(t *testing.T) {
	skipIfKCDown(t)
	env := newKCSlydsEnv(t)

	token := getKCToken(t, env.OIDC.TokenEndpoint, kcScopeRead)
	c := client.NewClient(
		env.Server.URL+"/mcp",
		mcpcore.ClientInfo{Name: "kc-test", Version: "0.1.0"},
		client.WithClientBearerToken(token),
	)
	if err := c.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	result, err := c.ToolCall("list_decks", map[string]any{})
	if err != nil {
		t.Fatalf("list_decks: %v", err)
	}
	if !strings.Contains(result, "test-deck") {
		t.Errorf("expected test-deck in result: %s", result)
	}
}

// TestKC_NoToken verifies unauthenticated requests get 401.
func TestKC_NoToken(t *testing.T) {
	skipIfKCDown(t)
	env := newKCSlydsEnv(t)

	req, _ := http.NewRequest("POST", env.Server.URL+"/mcp", strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
	wwwAuth := resp.Header.Get("WWW-Authenticate")
	if !strings.Contains(wwwAuth, "Bearer") {
		t.Errorf("WWW-Authenticate missing Bearer: %s", wwwAuth)
	}
}

// TestKC_ScopeWrite verifies slyds:write scope allows mutations.
func TestKC_ScopeWrite(t *testing.T) {
	skipIfKCDown(t)
	env := newKCSlydsEnv(t)

	token := getKCToken(t, env.OIDC.TokenEndpoint, kcScopeRead, kcScopeWrite)
	c := client.NewClient(
		env.Server.URL+"/mcp",
		mcpcore.ClientInfo{Name: "kc-test", Version: "0.1.0"},
		client.WithClientBearerToken(token),
	)
	if err := c.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	result, err := c.ToolCall("create_deck", map[string]any{
		"name": "kc-deck", "title": "KC Created", "theme": "default",
	})
	if err != nil {
		t.Fatalf("create_deck: %v", err)
	}
	if strings.Contains(result, "scope") {
		t.Errorf("create_deck should succeed with write scope: %s", result)
	}
}

// TestKC_ScopeReadOnly verifies mutations are blocked without slyds:write.
func TestKC_ScopeReadOnly(t *testing.T) {
	skipIfKCDown(t)
	env := newKCSlydsEnv(t)

	token := getKCToken(t, env.OIDC.TokenEndpoint, kcScopeRead)
	c := client.NewClient(
		env.Server.URL+"/mcp",
		mcpcore.ClientInfo{Name: "kc-test", Version: "0.1.0"},
		client.WithClientBearerToken(token),
	)
	if err := c.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	// Read should work
	result, err := c.ToolCall("list_decks", map[string]any{})
	if err != nil {
		t.Fatalf("list_decks: %v", err)
	}
	if !strings.Contains(result, "test-deck") {
		t.Errorf("list_decks should work: %s", result)
	}

	// Mutation should fail
	result, err = c.ToolCall("create_deck", map[string]any{
		"name": "blocked", "title": "Should Fail", "theme": "default",
	})
	if err != nil {
		// Tool error is fine — means scope check worked
		return
	}
	if !strings.Contains(result, "scope") && !strings.Contains(result, "insufficient") {
		t.Errorf("create_deck should fail without write scope: %s", result)
	}
}

// TestKC_PRM verifies the PRM endpoint includes Keycloak as auth server.
func TestKC_PRM(t *testing.T) {
	skipIfKCDown(t)
	env := newKCSlydsEnv(t)

	resp, err := http.Get(env.Server.URL + "/.well-known/oauth-protected-resource/mcp")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("PRM status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var prm map[string]any
	json.Unmarshal(body, &prm)
	servers, _ := prm["authorization_servers"].([]any)
	found := false
	for _, s := range servers {
		if fmt.Sprint(s) == env.OIDC.Issuer {
			found = true
		}
	}
	if !found {
		t.Errorf("PRM missing Keycloak issuer %s in %v", env.OIDC.Issuer, servers)
	}
}

// TestKC_UserToken verifies password-grant user tokens work.
func TestKC_UserToken(t *testing.T) {
	skipIfKCDown(t)
	env := newKCSlydsEnv(t)

	token := getUserToken(t, env.OIDC.TokenEndpoint)
	c := client.NewClient(
		env.Server.URL+"/mcp",
		mcpcore.ClientInfo{Name: "kc-test", Version: "0.1.0"},
		client.WithClientBearerToken(token),
	)
	if err := c.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	result, err := c.ToolCall("list_decks", map[string]any{})
	if err != nil {
		t.Fatalf("list_decks: %v", err)
	}
	if !strings.Contains(result, "test-deck") {
		t.Errorf("user token should work: %s", result)
	}
}
