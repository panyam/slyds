package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/panyam/mcpkit"
	"github.com/panyam/slyds/core"
)

// mcpClient wraps an httptest.Server for MCP e2e tests.
type mcpClient struct {
	t         *testing.T
	server    *httptest.Server
	sessionID string
	nextID    int
}

func newMCPClient(t *testing.T, root string) *mcpClient {
	t.Helper()

	srv := mcpkit.NewServer(mcpkit.ServerInfo{Name: "slyds-test", Version: "0.0.1"})
	registerResources(srv, root)
	registerTools(srv, root)

	handler := srv.Handler(mcpkit.WithStreamableHTTP(true))
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	c := &mcpClient{t: t, server: ts, nextID: 1}
	c.initialize()
	return c
}

func (c *mcpClient) initialize() {
	resp := c.rawPost(`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	c.sessionID = resp.Header.Get("Mcp-Session-Id")
	if c.sessionID == "" {
		c.t.Fatal("no session ID from initialize")
	}
	// Send initialized notification
	c.rawPostWithSession(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
}

func (c *mcpClient) rawPost(body string) *http.Response {
	resp, err := http.Post(c.server.URL+"/mcp", "application/json", strings.NewReader(body))
	if err != nil {
		c.t.Fatalf("POST failed: %v", err)
	}
	return resp
}

func (c *mcpClient) rawPostWithSession(body string) *http.Response {
	req, _ := http.NewRequest("POST", c.server.URL+"/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mcp-Session-Id", c.sessionID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("POST failed: %v", err)
	}
	return resp
}

func (c *mcpClient) call(method string, params any) map[string]any {
	c.t.Helper()
	c.nextID++
	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      c.nextID,
		"method":  method,
	}
	if params != nil {
		reqBody["params"] = params
	}
	data, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", c.server.URL+"/mcp", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mcp-Session-Id", c.sessionID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s: POST failed: %v", method, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		c.t.Fatalf("%s: invalid JSON response: %s", method, string(body))
	}
	return result
}

func (c *mcpClient) toolCall(name string, args any) string {
	c.t.Helper()
	result := c.call("tools/call", map[string]any{"name": name, "arguments": args})
	r, _ := result["result"].(map[string]any)
	if r == nil {
		c.t.Fatalf("tools/call %s: no result: %v", name, result)
	}
	if isErr, _ := r["isError"].(bool); isErr {
		content := r["content"].([]any)
		text := content[0].(map[string]any)["text"].(string)
		c.t.Fatalf("tools/call %s error: %s", name, text)
	}
	content := r["content"].([]any)
	return content[0].(map[string]any)["text"].(string)
}

func (c *mcpClient) readResource(uri string) string {
	c.t.Helper()
	result := c.call("resources/read", map[string]string{"uri": uri})
	r, _ := result["result"].(map[string]any)
	if r == nil {
		c.t.Fatalf("resources/read %s: no result: %v", uri, result)
	}
	contents := r["contents"].([]any)
	return contents[0].(map[string]any)["text"].(string)
}

// --- E2E Tests ---

func TestE2E_FullAgentWorkflow(t *testing.T) {
	root := t.TempDir()
	// Scaffold an existing deck
	core.CreateInDir("Existing Deck", 3, "default", fmt.Sprintf("%s/existing", root), true)

	c := newMCPClient(t, root)

	// 1. Discover decks via resource
	decksJSON := c.readResource("slyds://decks")
	var decks []map[string]any
	json.Unmarshal([]byte(decksJSON), &decks)
	if len(decks) != 1 || decks[0]["name"] != "existing" {
		t.Fatalf("expected 1 deck 'existing', got: %v", decks)
	}

	// 2. Read deck metadata via resource
	metaJSON := c.readResource("slyds://decks/existing")
	var meta map[string]any
	json.Unmarshal([]byte(metaJSON), &meta)
	if meta["title"] != "Existing Deck" {
		t.Errorf("title = %v, want 'Existing Deck'", meta["title"])
	}

	// 3. Read slide content via resource
	slideHTML := c.readResource("slyds://decks/existing/slides/1")
	if !strings.Contains(slideHTML, `class="slide`) {
		t.Error("slide 1 missing slide class")
	}

	// 4. Create a new deck via tool
	createResult := c.toolCall("create_deck", map[string]any{
		"name": "new-deck", "title": "Agent Created", "theme": "dark", "slides": 2,
	})
	if !strings.Contains(createResult, "Agent Created") {
		t.Error("create_deck didn't return title")
	}

	// 5. Verify new deck appears in resource list
	decksJSON = c.readResource("slyds://decks")
	json.Unmarshal([]byte(decksJSON), &decks)
	if len(decks) != 2 {
		t.Errorf("expected 2 decks after create, got %d", len(decks))
	}

	// 6. Query slide h1 via tool
	queryResult := c.toolCall("query_slide", map[string]any{
		"deck": "new-deck", "slide": "1", "selector": "h1",
	})
	if !strings.Contains(queryResult, "Agent Created") {
		t.Errorf("query h1 = %s, want 'Agent Created'", queryResult)
	}

	// 7. Edit slide via tool
	c.toolCall("edit_slide", map[string]any{
		"deck": "new-deck", "position": 1,
		"content": `<div class="slide"><h1>Modified</h1></div>`,
	})

	// 8. Verify edit via read_slide tool
	readResult := c.toolCall("read_slide", map[string]any{
		"deck": "new-deck", "position": 1,
	})
	if !strings.Contains(readResult, "Modified") {
		t.Error("edit not persisted")
	}

	// 9. Add slide via tool
	c.toolCall("add_slide", map[string]any{
		"deck": "new-deck", "position": 2, "name": "extra", "layout": "content", "title": "Extra",
	})
	listResult := c.toolCall("list_slides", map[string]any{"deck": "new-deck"})
	var slides []map[string]any
	json.Unmarshal([]byte(listResult), &slides)
	if len(slides) != 3 {
		t.Errorf("after add: expected 3 slides, got %d", len(slides))
	}

	// 10. Check deck via tool
	checkResult := c.toolCall("check_deck", map[string]any{"deck": "new-deck"})
	if !strings.Contains(checkResult, "InSync") {
		t.Error("check_deck missing InSync field")
	}

	// 11. Build deck via tool
	buildResult := c.toolCall("build_deck", map[string]any{"deck": "new-deck"})
	if !strings.Contains(buildResult, "<style>") {
		t.Error("build missing inlined CSS")
	}
	if strings.Contains(buildResult, "{{#") {
		t.Error("build has unresolved includes")
	}
}

func TestE2E_ResourceTemplatesList(t *testing.T) {
	c := newMCPClient(t, t.TempDir())

	result := c.call("resources/templates/list", nil)
	r := result["result"].(map[string]any)
	templates := r["resourceTemplates"].([]any)

	names := make(map[string]bool)
	for _, tmpl := range templates {
		m := tmpl.(map[string]any)
		names[m["uriTemplate"].(string)] = true
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

func TestE2E_ToolsList(t *testing.T) {
	c := newMCPClient(t, t.TempDir())

	result := c.call("tools/list", nil)
	r := result["result"].(map[string]any)
	tools := r["tools"].([]any)

	names := make(map[string]bool)
	for _, tool := range tools {
		m := tool.(map[string]any)
		names[m["name"].(string)] = true
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

func TestE2E_ServerInfo(t *testing.T) {
	c := newMCPClient(t, t.TempDir())

	text := c.readResource("slyds://server/info")
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
