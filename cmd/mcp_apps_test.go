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

// TestPreviewSlideReturnsHTML verifies that preview_slide now goes through
// the same Build() pipeline as preview_deck and injects an init script that
// opens the deck on the requested slide. The cached HTML must be a full
// presentation (multiple slides, navigation, slyds.js) with the hash-setter
// in place so slyds.js's getSlideFromHash() picks up the right starting
// position on load. Navigation (Prev/Next) remains functional from there.
func TestPreviewSlideReturnsHTML(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Slide Preview", "dark", 3)

	_, handler := previewSlideToolParts(root)
	result := callTool(t, handler, map[string]any{"deck": "test-deck", "position": 2})
	if result.IsError {
		t.Fatalf("preview_slide error: %s", toolText(result))
	}

	text := toolText(result)
	if !strings.Contains(text, "slide 2/3") {
		t.Errorf("preview_slide summary missing position info: %q", text)
	}
	if !strings.Contains(text, "Preview available") {
		t.Error("preview_slide summary missing 'Preview available'")
	}

	// Verify cache has a full deck HTML, not a single-slide preview.
	html, ok := previewCache.Load("ui://slyds/preview-slide")
	if !ok {
		t.Fatal("preview cache empty after preview_slide call")
	}
	if !strings.Contains(html, `data-theme="dark"`) {
		t.Error("cached slide HTML missing data-theme attribute")
	}
	if !strings.Contains(html, "<style>") {
		t.Error("cached slide HTML missing <style> (CSS not inlined)")
	}
	if !strings.Contains(html, `class="slide`) {
		t.Error("cached slide HTML missing slide class")
	}
	// Full deck markers: navigation buttons + slyds.js engine runtime.
	if !strings.Contains(html, `id="prevBtn"`) || !strings.Contains(html, `id="nextBtn"`) {
		t.Error("cached slide HTML missing navigation buttons — preview_slide is not going through Build()")
	}
	// Init script must set the hash to the requested slide BEFORE slyds.js
	// runs so the initial render lands on position 2.
	if !strings.Contains(html, `window.location.hash='2'`) {
		t.Errorf("cached slide HTML missing init script for position 2:\n%s", snippet(html, 500))
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

// TestPreviewSlideZeroPosition — 0 and negative positions are out of range.
func TestPreviewSlideZeroPosition(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Zero Test", "default", 3)

	_, handler := previewSlideToolParts(root)
	result := callTool(t, handler, map[string]any{"deck": "test-deck", "position": 0})
	if !result.IsError {
		t.Error("expected error for position 0")
	}
}

// TestPreviewSlideMatchesPreviewDeck verifies the unification: the HTML
// cached by preview_slide is the HTML cached by preview_deck *plus* the
// injected init script. If this test fails, the two tools have drifted
// apart again — fix whichever one is doing something bespoke.
func TestPreviewSlideMatchesPreviewDeck(t *testing.T) {
	root := scaffoldTestDeck(t, "parity", "Parity Deck", "default", 4)

	_, deckHandler := previewDeckToolParts(root)
	_, slideHandler := previewSlideToolParts(root)

	if r := callTool(t, deckHandler, map[string]string{"deck": "parity"}); r.IsError {
		t.Fatalf("preview_deck: %s", toolText(r))
	}
	if r := callTool(t, slideHandler, map[string]any{"deck": "parity", "position": 3}); r.IsError {
		t.Fatalf("preview_slide: %s", toolText(r))
	}

	deckHTML, _ := previewCache.Load("ui://slyds/preview-deck")
	slideHTML, _ := previewCache.Load("ui://slyds/preview-slide")

	if deckHTML == "" || slideHTML == "" {
		t.Fatal("preview cache empty after calls")
	}

	// preview_slide HTML must contain the init script; preview_deck HTML
	// must NOT (otherwise the unification leaked the wrong way round).
	if strings.Contains(deckHTML, "window.location.hash=") {
		t.Error("preview_deck HTML unexpectedly contains an init hash script")
	}
	if !strings.Contains(slideHTML, `window.location.hash='3'`) {
		t.Error("preview_slide HTML missing init hash script for position 3")
	}

	// Removing the init script from preview_slide should leave HTML that
	// matches preview_deck structurally (same deck title, same slide count,
	// same asset inlining).
	for _, marker := range []string{
		`class="slideshow-container"`,
		`id="prevBtn"`,
		`id="nextBtn"`,
		`Parity Deck`,
		`<style>`,
	} {
		if !strings.Contains(deckHTML, marker) {
			t.Errorf("preview_deck HTML missing marker %q", marker)
		}
		if !strings.Contains(slideHTML, marker) {
			t.Errorf("preview_slide HTML missing marker %q — divergence from preview_deck", marker)
		}
	}
}

// snippet returns the first n runes of s, for use in error messages.
func snippet(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
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
	if !strings.Contains(result, "slide 1/3") {
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
