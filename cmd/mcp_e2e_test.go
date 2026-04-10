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

	"github.com/panyam/mcpkit/client"
	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/mcpkit/testutil"
	"github.com/panyam/slyds/core"
)

// --- Test-only result structs for ToolCallTyped ---

// testDeckSummary matches the JSON shape returned by list_decks:
// an array of objects with name, title, theme, and slide count.
type testDeckSummary struct {
	Name   string `json:"name"`
	Title  string `json:"title"`
	Theme  string `json:"theme"`
	Slides int    `json:"slides"`
}

// testDeckDescription matches the JSON shape returned by describe_deck,
// mirroring core.DeckDescription but limited to the fields we verify.
type testDeckDescription struct {
	Title      string                 `json:"title"`
	Theme      string                 `json:"theme"`
	SlideCount int                    `json:"slide_count"`
	Slides     []testSlideDescription `json:"slides"`
}

// testSlideDescription matches the slide entries in a describe_deck response.
type testSlideDescription struct {
	Position int    `json:"position"`
	File     string `json:"file"`
	Layout   string `json:"layout"`
	Title    string `json:"title"`
}

// testCheckResult matches the JSON shape returned by check_deck,
// mirroring core.CheckResult but limited to the fields we verify.
type testCheckResult struct {
	SlideCount int  `json:"slide_count"`
	InSync     bool `json:"in_sync"`
}

// toolCallTyped is a test helper that calls client.ToolCallTyped and fails
// the test on error. Requires tools to return mcpcore.StructuredResult.
func toolCallTyped[T any](t *testing.T, tc *testutil.TestClient, name string, args any) T {
	t.Helper()
	result, err := client.ToolCallTyped[T](tc.Client, name, args)
	if err != nil {
		t.Fatalf("ToolCallTyped(%s): %v", name, err)
	}
	return result
}

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

// TestE2E_ListDecks verifies that the list_decks tool returns typed structured
// content with name, title, theme, and slide count for each deck. Uses
// ToolCallTyped to verify structured result unmarshaling end-to-end.
func TestE2E_ListDecks(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Alpha Talk", 3, "default", fmt.Sprintf("%s/alpha", root), true)
	core.CreateInDir("Beta Talk", 5, "dark", fmt.Sprintf("%s/beta", root), true)

	c := newSlydsMCPClient(t, root)

	decks := toolCallTyped[[]testDeckSummary](t, c, "list_decks", map[string]any{})

	if len(decks) != 2 {
		t.Fatalf("expected 2 decks, got %d", len(decks))
	}

	names := make(map[string]bool)
	for _, d := range decks {
		names[d.Name] = true
		if d.Title == "" {
			t.Errorf("deck %s has empty title", d.Name)
		}
		if d.Theme == "" {
			t.Errorf("deck %s has empty theme", d.Name)
		}
	}
	if !names["alpha"] || !names["beta"] {
		t.Errorf("expected decks alpha and beta, got: %v", names)
	}
}

// TestE2E_ToolCallTyped verifies that tools returning JSON produce structured
// content that can be unmarshaled via client.ToolCallTyped into typed Go structs.
// Exercises list_decks, describe_deck, list_slides, and check_deck — the four
// tools that return structured JSON via jsonResult. This test validates the
// StructuredResult integration end-to-end: tool handler → wire format →
// client deserialization.
func TestE2E_ToolCallTyped(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Typed Test", 3, "default", fmt.Sprintf("%s/typed", root), true)

	c := newSlydsMCPClient(t, root)

	// 1. list_decks → []testDeckSummary
	decks := toolCallTyped[[]testDeckSummary](t, c, "list_decks", map[string]any{})
	if len(decks) != 1 {
		t.Fatalf("list_decks: expected 1 deck, got %d", len(decks))
	}
	if decks[0].Name != "typed" {
		t.Errorf("list_decks: name = %q, want 'typed'", decks[0].Name)
	}
	if decks[0].Slides != 3 {
		t.Errorf("list_decks: slides = %d, want 3", decks[0].Slides)
	}

	// 2. describe_deck → testDeckDescription
	desc := toolCallTyped[testDeckDescription](t, c, "describe_deck", map[string]any{"deck": "typed"})
	if desc.Title != "Typed Test" {
		t.Errorf("describe_deck: title = %q, want 'Typed Test'", desc.Title)
	}
	if desc.Theme != "default" {
		t.Errorf("describe_deck: theme = %q, want 'default'", desc.Theme)
	}
	if desc.SlideCount != 3 {
		t.Errorf("describe_deck: slide_count = %d, want 3", desc.SlideCount)
	}
	if len(desc.Slides) != 3 {
		t.Errorf("describe_deck: len(slides) = %d, want 3", len(desc.Slides))
	}

	// 3. list_slides → []testSlideDescription
	slides := toolCallTyped[[]testSlideDescription](t, c, "list_slides", map[string]any{"deck": "typed"})
	if len(slides) != 3 {
		t.Fatalf("list_slides: expected 3 slides, got %d", len(slides))
	}
	if slides[0].Position != 1 {
		t.Errorf("list_slides: first slide position = %d, want 1", slides[0].Position)
	}

	// 4. check_deck → testCheckResult
	check := toolCallTyped[testCheckResult](t, c, "check_deck", map[string]any{"deck": "typed"})
	if !check.InSync {
		t.Error("check_deck: expected in_sync = true")
	}
	if check.SlideCount != 3 {
		t.Errorf("check_deck: slide_count = %d, want 3", check.SlideCount)
	}
}

// TestPerToolTimeout verifies that per-tool timeout values are set on the
// ToolDef returned by tool factory functions. ToolDef.Timeout is server-side
// only (json:"-"), so this is a unit test inspecting the struct directly
// rather than an E2E test via tools/list.
func TestPerToolTimeout(t *testing.T) {
	root := t.TempDir()

	build := buildDeckTool(root)
	if build.Timeout != 30*time.Second {
		t.Errorf("build_deck timeout = %v, want 30s", build.Timeout)
	}

	check := checkDeckTool(root)
	if check.Timeout != 10*time.Second {
		t.Errorf("check_deck timeout = %v, want 10s", check.Timeout)
	}

	// Other tools should have no per-tool timeout (use server default)
	list := listDecksTool(root)
	if list.Timeout != 0 {
		t.Errorf("list_decks timeout = %v, want 0 (server default)", list.Timeout)
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
