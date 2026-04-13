package cmd

import (
	"path/filepath"
	"testing"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/mcpkit/testutil"
	"github.com/panyam/slyds/core"
)

// newSlydsMCPClientForCompletions creates a TestClient with resources,
// tools, and completions registered. Uses the standard workspace middleware.
func newSlydsMCPClientForCompletions(t *testing.T, root string) *testutil.TestClient {
	t.Helper()
	ws, err := NewLocalWorkspace(root)
	if err != nil {
		t.Fatalf("NewLocalWorkspace: %v", err)
	}
	srv := server.NewServer(
		mcpcore.ServerInfo{Name: "slyds-test", Version: "0.0.1"},
		server.WithMiddleware(workspaceMiddleware(ws)),
	)
	registerResources(srv)
	registerTools(srv)
	registerCompletions(srv)
	return testutil.NewTestClient(t, srv)
}

// completeResource sends a completion/complete request for a resource
// template URI and returns the completion values.
func completeResource(t *testing.T, c *testutil.TestClient, uri, argName, argValue string) []string {
	t.Helper()
	params := map[string]any{
		"ref": map[string]any{
			"type": "ref/resource",
			"uri":  uri,
		},
		"argument": map[string]any{
			"name":  argName,
			"value": argValue,
		},
	}
	result := c.Call("completion/complete", params)
	var parsed mcpcore.CompletionCompleteResult
	if err := result.Unmarshal(&parsed); err != nil {
		t.Fatalf("unmarshal completion result: %v", err)
	}
	return parsed.Completion.Values
}

// TestE2E_CompleteDeckName verifies that completing the {name} param
// on a deck resource template returns all matching deck names.
func TestE2E_CompleteDeckName(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Alpha Deck", 2, "default", filepath.Join(root, "alpha"), true)
	core.CreateInDir("Beta Deck", 2, "dark", filepath.Join(root, "beta"), true)

	c := newSlydsMCPClientForCompletions(t, root)

	values := completeResource(t, c, "slyds://decks/{name}", "name", "")
	if len(values) < 2 {
		t.Fatalf("expected at least 2 deck completions, got %d: %v", len(values), values)
	}
	has := func(name string) bool {
		for _, v := range values {
			if v == name {
				return true
			}
		}
		return false
	}
	if !has("alpha") {
		t.Error("completion missing 'alpha'")
	}
	if !has("beta") {
		t.Error("completion missing 'beta'")
	}
}

// TestE2E_CompleteDeckNamePrefix verifies prefix filtering — only deck
// names starting with the partial input are returned.
func TestE2E_CompleteDeckNamePrefix(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Alpha", 2, "default", filepath.Join(root, "alpha"), true)
	core.CreateInDir("Beta", 2, "dark", filepath.Join(root, "beta"), true)
	core.CreateInDir("Gamma", 2, "minimal", filepath.Join(root, "gamma"), true)

	c := newSlydsMCPClientForCompletions(t, root)

	values := completeResource(t, c, "slyds://decks/{name}", "name", "al")
	if len(values) != 1 || values[0] != "alpha" {
		t.Errorf("expected [alpha], got %v", values)
	}

	values = completeResource(t, c, "slyds://decks/{name}", "name", "b")
	if len(values) != 1 || values[0] != "beta" {
		t.Errorf("expected [beta], got %v", values)
	}
}

// TestE2E_CompleteDeckNameOnPreviewTemplate verifies that deck name
// completion works on the ui:// preview template URIs too.
func TestE2E_CompleteDeckNameOnPreviewTemplate(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("My Deck", 3, "default", filepath.Join(root, "mydeck"), true)

	c := newSlydsMCPClientForCompletions(t, root)

	values := completeResource(t, c, "ui://slyds/decks/{deck}/preview", "deck", "my")
	if len(values) != 1 || values[0] != "mydeck" {
		t.Errorf("expected [mydeck], got %v", values)
	}
}

// TestE2E_CompleteSlidePosition verifies that completing {n} on a slide
// resource template returns valid position numbers.
func TestE2E_CompleteSlidePosition(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Five Slides", 5, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClientForCompletions(t, root)

	values := completeResource(t, c, "slyds://decks/{name}/slides/{n}", "n", "")
	if len(values) != 5 {
		t.Fatalf("expected 5 slide positions, got %d: %v", len(values), values)
	}
	for i, want := range []string{"1", "2", "3", "4", "5"} {
		if values[i] != want {
			t.Errorf("position[%d] = %q, want %q", i, values[i], want)
		}
	}
}

// TestE2E_CompleteSlidePositionPrefix verifies prefix filtering on slide
// positions — "1" on a 12-slide deck returns "1", "10", "11", "12".
func TestE2E_CompleteSlidePositionPrefix(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Many Slides", 12, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClientForCompletions(t, root)

	values := completeResource(t, c, "slyds://decks/{name}/slides/{n}", "n", "1")
	expected := []string{"1", "10", "11", "12"}
	if len(values) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, values)
	}
	for i, want := range expected {
		if values[i] != want {
			t.Errorf("position[%d] = %q, want %q", i, values[i], want)
		}
	}
}

// TestE2E_CompleteUnknownParam verifies that completing an unregistered
// resource URI returns an empty result (graceful fallback, no error).
func TestE2E_CompleteUnknownParam(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Deck", 2, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClientForCompletions(t, root)

	values := completeResource(t, c, "slyds://unknown/{foo}", "foo", "")
	if len(values) != 0 {
		t.Errorf("expected empty completions for unknown URI, got %v", values)
	}
}
