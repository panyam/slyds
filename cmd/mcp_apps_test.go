package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/ext/ui"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/mcpkit/testutil"
	"github.com/panyam/slyds/core"
)

// newSlydsMCPClientWithUI creates a TestClient connected to a slyds MCP server
// that has the MCP Apps (UI) extension enabled. Uses mcpkit/testutil for
// automatic httptest server lifecycle, session management, and t.Fatal on errors.
func newSlydsMCPClientWithUI(t *testing.T, root string) *testutil.TestClient {
	t.Helper()
	srv := server.NewServer(
		mcpcore.ServerInfo{Name: "slyds-test", Version: "0.0.1"},
		server.WithExtension(ui.UIExtension{}),
	)
	registerResources(srv, root)
	registerTools(srv, root)
	registerAppTools(srv, root)
	return testutil.NewTestClient(t, srv)
}

// TestPreviewDeckReturnsHTML verifies that the preview_deck tool handler
// builds a full deck preview, stores it in the preview cache, and returns
// a text summary containing the deck title. The cached HTML should contain
// inlined CSS (<style>) and the deck's slide content.
func TestPreviewDeckReturnsHTML(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Preview Deck", "default", 3)

	_, handler := previewDeckToolParts(root)
	result := callTool(t, handler, map[string]string{"deck": "test-deck"})
	if result.IsError {
		t.Fatalf("preview_deck error: %s", toolText(result))
	}

	text := toolText(result)
	if !strings.Contains(text, "Preview Deck") {
		t.Error("preview_deck summary missing deck title")
	}
	if !strings.Contains(text, "Preview available") {
		t.Error("preview_deck summary missing 'Preview available'")
	}

	// Verify cache has HTML
	html, ok := previewCache.Load("ui://slyds/preview-deck")
	if !ok {
		t.Fatal("preview cache empty after preview_deck call")
	}
	if !strings.Contains(html, "<style>") {
		t.Error("cached deck HTML missing <style>")
	}
	if !strings.Contains(html, "Preview Deck") {
		t.Error("cached deck HTML missing deck title")
	}
}

// TestPreviewSlideReturnsHTML verifies that the preview_slide tool handler
// renders a single slide with theme CSS, stores it in the preview cache,
// and returns a text summary. The cached HTML should be a self-contained
// page with the slide content, base CSS, and theme CSS inlined.
func TestPreviewSlideReturnsHTML(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Slide Preview", "dark", 3)

	_, handler := previewSlideToolParts(root)
	result := callTool(t, handler, map[string]any{"deck": "test-deck", "position": 1})
	if result.IsError {
		t.Fatalf("preview_slide error: %s", toolText(result))
	}

	text := toolText(result)
	if !strings.Contains(text, "Slide 1") {
		t.Error("preview_slide summary missing slide position")
	}
	if !strings.Contains(text, "Preview available") {
		t.Error("preview_slide summary missing 'Preview available'")
	}

	// Verify cache has self-contained HTML with theme
	html, ok := previewCache.Load("ui://slyds/preview-slide")
	if !ok {
		t.Fatal("preview cache empty after preview_slide call")
	}
	if !strings.Contains(html, `data-theme="dark"`) {
		t.Error("cached slide HTML missing data-theme attribute")
	}
	if !strings.Contains(html, "<style>") {
		t.Error("cached slide HTML missing <style>")
	}
	if !strings.Contains(html, `class="slide`) {
		t.Error("cached slide HTML missing slide class")
	}
}

// TestPreviewSlideInvalidPosition verifies that preview_slide returns an
// error when given an out-of-range slide position.
func TestPreviewSlideInvalidPosition(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Error Test", "default", 3)

	_, handler := previewSlideToolParts(root)
	result := callTool(t, handler, map[string]any{"deck": "test-deck", "position": 99})
	if !result.IsError {
		t.Error("expected error for invalid slide position")
	}
}

// TestE2E_UIExtensionAdvertised verifies that the server advertises the
// io.modelcontextprotocol/ui extension and that preview tools include
// _meta.ui with the correct resourceUri in their tool definitions.
func TestE2E_UIExtensionAdvertised(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("UI Test", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClientWithUI(t, root)
	tools := c.ListTools()

	// Find preview_deck and check _meta.ui
	for _, tool := range tools {
		if tool.Name == "preview_deck" {
			if tool.Meta == nil || tool.Meta.UI == nil {
				t.Fatal("preview_deck missing _meta.ui")
			}
			if tool.Meta.UI.ResourceUri != "ui://slyds/preview-deck" {
				t.Errorf("preview_deck resourceUri = %q, want ui://slyds/preview-deck", tool.Meta.UI.ResourceUri)
			}
			return
		}
	}
	t.Error("preview_deck tool not found in tools/list")
}

// TestE2E_PreviewDeckResource verifies the full MCP Apps flow: call the
// preview_deck tool, then read the ui://slyds/preview-deck resource and
// verify it returns HTML with the correct MCP App MIME type.
func TestE2E_PreviewDeckResource(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("E2E Deck", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClientWithUI(t, root)

	// Call the tool first to populate the cache
	result := c.ToolCall("preview_deck", map[string]any{"deck": "deck"})
	if !strings.Contains(result, "E2E Deck") {
		t.Fatalf("preview_deck result missing title: %s", result)
	}

	// Read the resource — should return HTML with AppMIMEType
	html := c.ReadResource("ui://slyds/preview-deck")
	if !strings.Contains(html, "<html") {
		t.Error("preview-deck resource missing <html")
	}
	if !strings.Contains(html, "E2E Deck") {
		t.Error("preview-deck resource missing deck title")
	}
}

// TestE2E_PreviewSlideResource verifies the full MCP Apps flow for single
// slide preview: call preview_slide, then read the ui:// resource.
func TestE2E_PreviewSlideResource(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Slide E2E", 3, "dark", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClientWithUI(t, root)

	result := c.ToolCall("preview_slide", map[string]any{"deck": "deck", "position": 1})
	if !strings.Contains(result, "Slide 1") {
		t.Fatalf("preview_slide result missing position: %s", result)
	}

	html := c.ReadResource("ui://slyds/preview-slide")
	if !strings.Contains(html, `data-theme="dark"`) {
		t.Error("preview-slide resource missing dark theme")
	}
	if !strings.Contains(html, `class="slide`) {
		t.Error("preview-slide resource missing slide content")
	}
}

// --- Helpers to extract tool parts for unit testing ---

// previewDeckToolParts returns the ToolDef and ToolHandler for preview_deck
// by registering app tools on a temporary server and extracting them.
func previewDeckToolParts(root string) (mcpcore.ToolDef, mcpcore.ToolHandler) {
	return extractAppTool(root, "preview_deck")
}

// previewSlideToolParts returns the ToolDef and ToolHandler for preview_slide.
func previewSlideToolParts(root string) (mcpcore.ToolDef, mcpcore.ToolHandler) {
	return extractAppTool(root, "preview_slide")
}

func extractAppTool(root, name string) (mcpcore.ToolDef, mcpcore.ToolHandler) {
	srv := server.NewServer(
		mcpcore.ServerInfo{Name: "test", Version: "0.0.1"},
		server.WithExtension(ui.UIExtension{}),
	)
	registerAppTools(srv, root)

	// Use tools/list to find the tool definition
	req := &mcpcore.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"1"`),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	}
	srv.Dispatch(context.Background(), req)
	srv.Dispatch(context.Background(), &mcpcore.Request{JSONRPC: "2.0", Method: "notifications/initialized"})

	listReq := &mcpcore.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"2"`),
		Method:  "tools/list",
	}
	resp := srv.Dispatch(context.Background(), listReq)
	var listResult struct {
		Tools []mcpcore.ToolDef `json:"tools"`
	}
	json.Unmarshal(resp.Result, &listResult)

	for _, tool := range listResult.Tools {
		if tool.Name == name {
			// Return the tool def + a handler that dispatches via the server
			handler := func(ctx context.Context, toolReq mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
				args, _ := json.Marshal(map[string]any{"name": name, "arguments": json.RawMessage(toolReq.Arguments)})
				callReq := &mcpcore.Request{
					JSONRPC: "2.0",
					ID:      json.RawMessage(`"3"`),
					Method:  "tools/call",
					Params:  args,
				}
				callResp := srv.Dispatch(ctx, callReq)
				var result mcpcore.ToolResult
				json.Unmarshal(callResp.Result, &result)
				return result, nil
			}
			return tool, handler
		}
	}
	panic(fmt.Sprintf("tool %q not found", name))
}
