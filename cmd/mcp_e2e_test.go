package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/panyam/mcpkit"
	"github.com/panyam/mcpkit/testutil"
	"github.com/panyam/slyds/core"
)

// newSlydsMCPClient creates a TestClient connected to a slyds MCP server
// with the given deck root. Uses mcpkit/testutil for automatic httptest
// server lifecycle, session management, and t.Fatal on errors.
func newSlydsMCPClient(t *testing.T, root string) *testutil.TestClient {
	t.Helper()
	srv := mcpkit.NewServer(mcpkit.ServerInfo{Name: "slyds-test", Version: "0.0.1"})
	registerResources(srv, root)
	registerTools(srv, root)
	return testutil.NewTestClient(t, srv)
}

// TestE2E_FullAgentWorkflow exercises the complete MCP agent lifecycle:
// discover decks → read metadata → read slide → create deck → query →
// edit → verify edit → add slide → check → build. All via HTTP against
// a real mcpkit server with slyds tools and resources registered.
func TestE2E_FullAgentWorkflow(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Existing Deck", 3, "default", fmt.Sprintf("%s/existing", root), true)

	c := newSlydsMCPClient(t, root)

	// 1. Discover decks via resource
	decksJSON := c.ReadResource("slyds://decks")
	var decks []map[string]any
	json.Unmarshal([]byte(decksJSON), &decks)
	if len(decks) != 1 || decks[0]["name"] != "existing" {
		t.Fatalf("expected 1 deck 'existing', got: %v", decks)
	}

	// 2. Read deck metadata via resource
	metaJSON := c.ReadResource("slyds://decks/existing")
	var meta map[string]any
	json.Unmarshal([]byte(metaJSON), &meta)
	if meta["title"] != "Existing Deck" {
		t.Errorf("title = %v, want 'Existing Deck'", meta["title"])
	}

	// 3. Read slide content via resource
	slideHTML := c.ReadResource("slyds://decks/existing/slides/1")
	if !strings.Contains(slideHTML, `class="slide`) {
		t.Error("slide 1 missing slide class")
	}

	// 4. Create a new deck via tool
	createResult := c.ToolCall("create_deck", map[string]any{
		"name": "new-deck", "title": "Agent Created", "theme": "dark", "slides": 2,
	})
	if !strings.Contains(createResult, "Agent Created") {
		t.Error("create_deck didn't return title")
	}

	// 5. Verify new deck appears in resource list
	decksJSON = c.ReadResource("slyds://decks")
	json.Unmarshal([]byte(decksJSON), &decks)
	if len(decks) != 2 {
		t.Errorf("expected 2 decks after create, got %d", len(decks))
	}

	// 6. Query slide h1 via tool
	queryResult := c.ToolCall("query_slide", map[string]any{
		"deck": "new-deck", "slide": "1", "selector": "h1",
	})
	if !strings.Contains(queryResult, "Agent Created") {
		t.Errorf("query h1 = %s, want 'Agent Created'", queryResult)
	}

	// 7. Edit slide via tool
	c.ToolCall("edit_slide", map[string]any{
		"deck": "new-deck", "position": 1,
		"content": `<div class="slide"><h1>Modified</h1></div>`,
	})

	// 8. Verify edit via read_slide tool
	readResult := c.ToolCall("read_slide", map[string]any{
		"deck": "new-deck", "position": 1,
	})
	if !strings.Contains(readResult, "Modified") {
		t.Error("edit not persisted")
	}

	// 9. Add slide via tool
	c.ToolCall("add_slide", map[string]any{
		"deck": "new-deck", "position": 2, "name": "extra", "layout": "content", "title": "Extra",
	})
	listResult := c.ToolCall("list_slides", map[string]any{"deck": "new-deck"})
	var slides []map[string]any
	json.Unmarshal([]byte(listResult), &slides)
	if len(slides) != 3 {
		t.Errorf("after add: expected 3 slides, got %d", len(slides))
	}

	// 10. Check deck via tool
	checkResult := c.ToolCall("check_deck", map[string]any{"deck": "new-deck"})
	if !strings.Contains(checkResult, "InSync") {
		t.Error("check_deck missing InSync field")
	}

	// 11. Build deck via tool
	buildResult := c.ToolCall("build_deck", map[string]any{"deck": "new-deck"})
	if !strings.Contains(buildResult, "<style>") {
		t.Error("build missing inlined CSS")
	}
	if strings.Contains(buildResult, "{{#") {
		t.Error("build has unresolved includes")
	}
}

// TestE2E_ResourceTemplatesList verifies that all 6 resource templates
// are registered and discoverable via resources/templates/list.
func TestE2E_ResourceTemplatesList(t *testing.T) {
	c := newSlydsMCPClient(t, t.TempDir())

	templates := c.ListResourceTemplates()
	names := make(map[string]bool)
	for _, tmpl := range templates {
		names[tmpl.URITemplate] = true
	}

	expected := []string{
		"slyds://decks",
		"slyds://decks/{name}",
		"slyds://decks/{name}/slides",
		"slyds://decks/{name}/slides/{n}",
		"slyds://decks/{name}/config",
		"slyds://decks/{name}/agent",
	}
	for _, uri := range expected {
		if !names[uri] {
			t.Errorf("missing resource template: %s", uri)
		}
	}
}

// TestE2E_ToolsList verifies that all 10 semantic tools are registered
// and discoverable via tools/list.
func TestE2E_ToolsList(t *testing.T) {
	c := newSlydsMCPClient(t, t.TempDir())

	tools := c.ListTools()
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}

	expected := []string{
		"create_deck", "describe_deck", "list_slides", "read_slide",
		"edit_slide", "query_slide", "add_slide", "remove_slide",
		"check_deck", "build_deck",
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing tool: %s", name)
		}
	}
}

// TestE2E_ServerInfo verifies the slyds://server/info static resource
// returns server version, available themes, and available layouts.
func TestE2E_ServerInfo(t *testing.T) {
	c := newSlydsMCPClient(t, t.TempDir())

	text := c.ReadResource("slyds://server/info")
	var info map[string]any
	json.Unmarshal([]byte(text), &info)

	if info["name"] != "slyds" {
		t.Errorf("server name = %v", info["name"])
	}
	themes, _ := info["themes"].([]any)
	if len(themes) < 3 {
		t.Errorf("expected >=3 themes, got %d", len(themes))
	}
}
