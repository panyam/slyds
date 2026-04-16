package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/mcpkit/client"
	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/mcpkit/testutil"
	"github.com/panyam/slyds/core"
)

// mockValidator is a test AuthValidator that accepts any Bearer token
// and returns fixed claims. Implements both AuthValidator and ClaimsProvider.
type mockValidator struct {
	claims *mcpcore.Claims
}

func (v *mockValidator) Validate(r *http.Request) error {
	auth := r.Header.Get("Authorization")
	if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
		return &mcpcore.AuthError{Code: 401, Message: "missing token"}
	}
	return nil
}

func (v *mockValidator) Claims(r *http.Request) *mcpcore.Claims {
	return v.claims
}

// staticToken implements core.TokenSource for tests.
type staticToken string

func (s staticToken) Token() (string, error) { return string(s), nil }

// newAuthMCPClient creates a TestClient with a mock auth validator that
// injects the given claims into every request's context.
func newAuthMCPClient(t *testing.T, root string, claims *mcpcore.Claims) *testutil.TestClient {
	t.Helper()
	ws, err := NewLocalWorkspace(root)
	if err != nil {
		t.Fatalf("NewLocalWorkspace: %v", err)
	}
	srv := server.NewServer(
		mcpcore.ServerInfo{Name: "slyds-auth-test", Version: "0.0.1"},
		server.WithMiddleware(workspaceMiddleware(ws)),
		server.WithAuth(&mockValidator{claims: claims}),
	)
	registerTools(srv)
	registerResources(srv)
	registerPrompts(srv)
	return testutil.NewTestClient(t, srv, client.WithTokenSource(staticToken("test-token")))
}

// TestE2E_NoAuth_ToolsWork verifies that tools work without any auth
// configured (backward compatibility).
func TestE2E_NoAuth_ToolsWork(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("No Auth", 2, "default", filepath.Join(root, "deck"), true)
	c := newSlydsMCPClient(t, root)

	result := c.ToolCall("list_decks", map[string]any{})
	if !strings.Contains(result, "deck") {
		t.Errorf("list_decks should work without auth, got: %s", result)
	}
}

// TestE2E_ScopeCheck_WriteToolAllowed verifies that mutation tools succeed
// when the token has the slyds:write scope.
func TestE2E_ScopeCheck_WriteToolAllowed(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Scope Test", 2, "default", filepath.Join(root, "deck"), true)

	claims := &mcpcore.Claims{
		Subject: "user-1",
		Scopes:  []string{"slyds:read", "slyds:write"},
	}
	c := newAuthMCPClient(t, root, claims)

	// Mutation should succeed with slyds:write
	c.ToolCall("create_deck", map[string]any{
		"name": "new-deck", "title": "Test", "theme": "default",
	})

	// Verify it was created
	result := c.ToolCall("list_decks", map[string]any{})
	if !strings.Contains(result, "new-deck") {
		t.Errorf("create_deck should succeed with slyds:write scope, got: %s", result)
	}
}

// TestE2E_ScopeCheck_WriteToolBlocked verifies that mutation tools are
// rejected when the token lacks the slyds:write scope.
func TestE2E_ScopeCheck_WriteToolBlocked(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Read Only", 2, "default", filepath.Join(root, "deck"), true)

	claims := &mcpcore.Claims{
		Subject: "user-2",
		Scopes:  []string{"slyds:read"}, // no slyds:write
	}
	c := newAuthMCPClient(t, root, claims)

	// Mutation should be blocked
	text, err := c.Client.ToolCall("create_deck", map[string]any{
		"name": "blocked", "title": "Should Fail", "theme": "default",
	})
	msg := text
	if err != nil {
		msg = err.Error()
	}
	if !strings.Contains(msg, "scope") && !strings.Contains(msg, "insufficient") {
		t.Errorf("create_deck should fail without slyds:write, got: %s", msg)
	}
}

// TestE2E_ScopeCheck_ReadToolAllowed verifies that read-only tools work
// even without the slyds:write scope.
func TestE2E_ScopeCheck_ReadToolAllowed(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Read OK", 2, "default", filepath.Join(root, "deck"), true)

	claims := &mcpcore.Claims{
		Subject: "user-3",
		Scopes:  []string{"slyds:read"}, // no slyds:write
	}
	c := newAuthMCPClient(t, root, claims)

	// Read tools should work
	result := c.ToolCall("list_decks", map[string]any{})
	if !strings.Contains(result, "deck") {
		t.Errorf("list_decks should work with slyds:read, got: %s", result)
	}

	result = c.ToolCall("describe_deck", map[string]any{"deck": "deck"})
	if !strings.Contains(result, "Read OK") {
		t.Errorf("describe_deck should work with slyds:read, got: %s", result)
	}
}

// TestE2E_ScopeCheck_QueryReadAllowed verifies that query_slide in read
// mode works without slyds:write, but mutation mode is blocked.
func TestE2E_ScopeCheck_QueryReadAllowed(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Query Scope", 2, "default", filepath.Join(root, "deck"), true)

	claims := &mcpcore.Claims{
		Subject: "user-4",
		Scopes:  []string{"slyds:read"},
	}
	c := newAuthMCPClient(t, root, claims)

	// Read query should work
	result := c.ToolCall("query_slide", map[string]any{
		"deck": "deck", "slide": "1", "selector": "h1",
	})
	if strings.Contains(result, "scope") {
		t.Errorf("query_slide read should work without write scope, got: %s", result)
	}

	// Mutation query should be blocked
	text, err := c.Client.ToolCall("query_slide", map[string]any{
		"deck": "deck", "slide": "1", "selector": "h1", "set": "Hacked",
	})
	msg := text
	if err != nil {
		msg = err.Error()
	}
	if !strings.Contains(msg, "scope") && !strings.Contains(msg, "insufficient") {
		t.Errorf("query_slide mutation should fail without write scope, got: %s", msg)
	}
}

// TestE2E_MCPAuthConfig_Disabled verifies MCPAuthConfig returns no options
// when not enabled.
func TestE2E_MCPAuthConfig_Disabled(t *testing.T) {
	cfg := MCPAuthConfig{}
	if cfg.IsEnabled() {
		t.Error("empty config should not be enabled")
	}
	opts := cfg.ServerOptions()
	if len(opts) != 0 {
		t.Errorf("disabled config should return no options, got %d", len(opts))
	}
}

// TestE2E_MCPAuthConfig_ScopeList verifies scope parsing.
func TestE2E_MCPAuthConfig_ScopeList(t *testing.T) {
	cfg := MCPAuthConfig{Scopes: "slyds:read, slyds:write, admin"}
	scopes := cfg.ScopeList()
	if len(scopes) != 3 {
		t.Fatalf("expected 3 scopes, got %d: %v", len(scopes), scopes)
	}
	if scopes[0] != "slyds:read" || scopes[1] != "slyds:write" || scopes[2] != "admin" {
		t.Errorf("unexpected scopes: %v", scopes)
	}
}

// TestE2E_PRMEndpoint verifies that the Protected Resource Metadata
// endpoint returns valid JSON when auth is configured.
func TestE2E_PRMEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	cfg := MCPAuthConfig{
		JWKSURL:  "https://auth.example.com/.well-known/jwks.json",
		Issuer:   "https://auth.example.com",
		Audience: "https://mcp.example.com",
		Scopes:   "slyds:read,slyds:write",
	}
	// Need to initialize the validator for MountPRM
	cfg.ServerOptions()
	cfg.MountPRM(mux, "https://mcp.example.com", "/mcp")

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/.well-known/oauth-protected-resource/mcp")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("PRM status = %d, want 200", resp.StatusCode)
	}
	var prm map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&prm); err != nil {
		t.Fatalf("PRM decode: %v", err)
	}
	if prm["resource"] != "https://mcp.example.com" {
		t.Errorf("PRM resource = %v", prm["resource"])
	}
}
