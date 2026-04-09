package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/mcpkit/testutil"
	"github.com/panyam/slyds/core"
)

// newSlydsMCPClient creates a TestClient connected to a slyds MCP server
// with the given deck root. Uses mcpkit/testutil for automatic httptest
// server lifecycle, session management, and t.Fatal on errors.
func newSlydsMCPClient(t *testing.T, root string) *testutil.TestClient {
	t.Helper()
	srv := server.NewServer(mcpcore.ServerInfo{Name: "slyds-test", Version: "0.0.1"})
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
	if !strings.Contains(checkResult, "in_sync") {
		t.Error("check_deck missing in_sync field")
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
		"list_decks", "create_deck", "describe_deck", "list_slides", "read_slide",
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

// TestE2E_ListDecks verifies that the list_decks tool returns a JSON array
// with name, title, theme, and slide count for each deck under the deck root.
// This is the tool-based alternative to reading the slyds://decks resource.
func TestE2E_ListDecks(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Alpha Talk", 3, "default", fmt.Sprintf("%s/alpha", root), true)
	core.CreateInDir("Beta Talk", 5, "dark", fmt.Sprintf("%s/beta", root), true)

	c := newSlydsMCPClient(t, root)

	result := c.ToolCall("list_decks", map[string]any{})
	var decks []map[string]any
	json.Unmarshal([]byte(result), &decks)

	if len(decks) != 2 {
		t.Fatalf("expected 2 decks, got %d: %s", len(decks), result)
	}

	names := make(map[string]bool)
	for _, d := range decks {
		names[d["name"].(string)] = true
		// Verify all expected fields are present
		for _, field := range []string{"name", "title", "theme", "slides"} {
			if _, ok := d[field]; !ok {
				t.Errorf("deck %v missing field: %s", d["name"], field)
			}
		}
	}
	if !names["alpha"] || !names["beta"] {
		t.Errorf("expected decks alpha and beta, got: %v", names)
	}
}

// TestE2E_StdioTransport verifies that the slyds MCP server works over the
// stdio transport (Content-Length framed JSON-RPC over stdin/stdout). This test
// creates a server with all tools and resources registered, wires it to a pair
// of io.Pipes simulating stdin/stdout, performs the MCP initialize handshake,
// then calls tools/list and verifies all 10 slyds tools are returned.
//
// This test exercises the full dispatch pipeline over stdio — the same code path
// used when an editor (Cursor, Claude Desktop) spawns `slyds mcp --stdio`.
func TestE2E_StdioTransport(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Stdio Test", 2, "default", fmt.Sprintf("%s/test-deck", root), true)

	srv := server.NewServer(mcpcore.ServerInfo{Name: "slyds-stdio", Version: "0.0.1"})
	registerResources(srv, root)
	registerTools(srv, root)

	// Create pipe pairs: server reads from sr, client writes to cw;
	// client reads from cr, server writes to sw.
	sr, cw := io.Pipe()
	cr, sw := io.Pipe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- srv.RunStdio(ctx, server.WithStdioInput(sr), server.WithStdioOutput(sw))
	}()

	reader := bufio.NewReader(cr)

	// 1. Initialize handshake.
	writeStdioFrame(t, cw, `{"jsonrpc":"2.0","id":"1","method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`)
	initResp := readStdioFrame(t, reader)
	var resp mcpcore.Response
	if err := json.Unmarshal(initResp, &resp); err != nil {
		t.Fatalf("unmarshal init response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("initialize error: %v", resp.Error)
	}

	// 2. Send initialized notification.
	writeStdioFrame(t, cw, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	// 3. List tools.
	writeStdioFrame(t, cw, `{"jsonrpc":"2.0","id":"2","method":"tools/list","params":{}}`)
	toolsResp := readStdioFrame(t, reader)
	if err := json.Unmarshal(toolsResp, &resp); err != nil {
		t.Fatalf("unmarshal tools/list response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("tools/list error: %v", resp.Error)
	}

	var toolsResult struct {
		Tools []struct{ Name string } `json:"tools"`
	}
	json.Unmarshal(resp.Result, &toolsResult)
	names := make(map[string]bool)
	for _, tool := range toolsResult.Tools {
		names[tool.Name] = true
	}
	expected := []string{
		"list_decks", "create_deck", "describe_deck", "list_slides", "read_slide",
		"edit_slide", "query_slide", "add_slide", "remove_slide",
		"check_deck", "build_deck",
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("stdio: missing tool %q", name)
		}
	}

	// 4. Call describe_deck via stdio to verify tool dispatch.
	writeStdioFrame(t, cw, `{"jsonrpc":"2.0","id":"3","method":"tools/call","params":{"name":"describe_deck","arguments":{"deck":"test-deck"}}}`)
	descResp := readStdioFrame(t, reader)
	if err := json.Unmarshal(descResp, &resp); err != nil {
		t.Fatalf("unmarshal describe_deck response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("describe_deck error: %v", resp.Error)
	}
	var toolResult struct {
		Content []struct{ Text string } `json:"content"`
	}
	json.Unmarshal(resp.Result, &toolResult)
	if len(toolResult.Content) == 0 || !strings.Contains(toolResult.Content[0].Text, "Stdio Test") {
		t.Error("stdio describe_deck didn't return deck title")
	}

	// Clean shutdown.
	cw.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunStdio: %v", err)
		}
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("RunStdio did not exit")
	}
}

// writeStdioFrame writes a Content-Length framed JSON-RPC message to a writer.
func writeStdioFrame(t *testing.T, w io.Writer, msg string) {
	t.Helper()
	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(msg), msg)
	if _, err := io.WriteString(w, frame); err != nil {
		t.Fatalf("writeStdioFrame: %v", err)
	}
}

// readStdioFrame reads a Content-Length framed JSON-RPC message from a reader.
// Parses the Content-Length header and reads exactly that many bytes of body.
func readStdioFrame(t *testing.T, r *bufio.Reader) []byte {
	t.Helper()
	// Read headers until blank line.
	contentLength := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("readStdioFrame header: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == "Content-Length" {
			fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &contentLength)
		}
	}
	if contentLength < 0 {
		t.Fatal("readStdioFrame: missing Content-Length")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		t.Fatalf("readStdioFrame body: %v", err)
	}
	return body
}
