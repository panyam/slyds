package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/slyds/core"
)

// NOTE: TestDiscoverDecks, TestDiscoverDecksEmpty, TestDiscoverDecksRootIsDeck,
// and TestOpenDeck previously exercised package-private helpers (`discoverDecks`,
// `openDeck`) that the Workspace refactor removed. Equivalent coverage now lives
// in cmd/workspace_test.go against LocalWorkspace.ListDecks and .OpenDeck with
// stricter assertions (matches by name instead of count only).

// TestResourceRegistration verifies that registerResources succeeds with
// the workspace middleware installed and the registered deck is readable
// through the Workspace API. The middleware path is the production wiring,
// so this test doubles as a smoke check for the wiring.
func TestResourceRegistration(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Deck", 2, "default", filepath.Join(root, "test-deck"), true)

	ws, err := NewLocalWorkspace(root)
	if err != nil {
		t.Fatalf("NewLocalWorkspace: %v", err)
	}

	srv := server.NewServer(
		mcpcore.ServerInfo{Name: "test", Version: "0.0.1"},
		server.WithMiddleware(workspaceMiddleware(ws)),
	)
	registerResources(srv)

	// Sanity check: resolve the deck via the workspace directly (what the
	// resource handlers do when the middleware is active).
	d, err := ws.OpenDeck("test-deck")
	if err != nil {
		t.Fatalf("ws.OpenDeck: %v", err)
	}
	content, err := d.GetSlideContent(1)
	if err != nil {
		t.Fatalf("GetSlideContent: %v", err)
	}
	if !strings.Contains(content, `class="slide`) {
		t.Error("slide 1 missing slide class")
	}
}

// TestJsonResultRoundTrip verifies that jsonResult produces valid JSON
// that round-trips correctly through marshal/unmarshal.
func TestJsonResultRoundTrip(t *testing.T) {
	result, _ := jsonResult(map[string]any{"key": "value"})
	text := toolText(result)
	var m map[string]string
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatalf("jsonResult not valid JSON: %v", err)
	}
	if m["key"] != "value" {
		t.Errorf("key = %q, want value", m["key"])
	}
}
