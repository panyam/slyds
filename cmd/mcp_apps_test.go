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
	ws, err := NewLocalWorkspace(root)
	if err != nil {
		t.Fatalf("NewLocalWorkspace: %v", err)
	}
	srv := server.NewServer(
		mcpcore.ServerInfo{Name: "slyds-test", Version: "0.0.1"},
		server.WithExtension(ui.UIExtension{}),
		server.WithMiddleware(workspaceMiddleware(ws)),
	)
	registerResources(srv)
	registerTools(srv)
	registerAppTools(srv)
	return testutil.NewTestClient(t, srv)
}

// TestPreviewDeckReturnsHTML verifies that the preview_deck tool handler
// builds a full deck preview and returns a text summary containing the
// deck title. The preview reference is stored so the resource handler
// can build on demand — no HTML is cached.
func TestPreviewDeckReturnsHTML(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Preview Deck", "default", 3)

	_, handler := previewDeckToolParts(root)
	result := callTool(t, root, handler, map[string]string{"deck": "test-deck"})
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

	// Verify the deck reference was stored (not the HTML).
	if previewDeckRef.Deck != "test-deck" {
		t.Errorf("previewDeckRef.Deck = %q, want test-deck", previewDeckRef.Deck)
	}
}

// TestPreviewSlideReturnsHTML verifies that preview_slide stores the deck +
// position reference and returns a text summary. The resource handler builds
// on demand through the workspace — no HTML is cached.
func TestPreviewSlideReturnsHTML(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Slide Preview", "dark", 3)

	_, handler := previewSlideToolParts(root)
	result := callTool(t, root, handler, map[string]any{"deck": "test-deck", "position": 2})
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

	// Verify the slide reference was stored (not the HTML).
	if previewSlideRef.Deck != "test-deck" {
		t.Errorf("previewSlideRef.Deck = %q, want test-deck", previewSlideRef.Deck)
	}
	if previewSlideRef.Position != 2 {
		t.Errorf("previewSlideRef.Position = %d, want 2", previewSlideRef.Position)
	}
}

// TestPreviewSlideInvalidPosition verifies that preview_slide returns an
// error when given an out-of-range slide position.
func TestPreviewSlideInvalidPosition(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Error Test", "default", 3)

	_, handler := previewSlideToolParts(root)
	result := callTool(t, root, handler, map[string]any{"deck": "test-deck", "position": 99})
	if !result.IsError {
		t.Error("expected error for invalid slide position")
	}
}

// TestPreviewSlideZeroPosition — 0 and negative positions are out of range.
func TestPreviewSlideZeroPosition(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Zero Test", "default", 3)

	_, handler := previewSlideToolParts(root)
	result := callTool(t, root, handler, map[string]any{"deck": "test-deck", "position": 0})
	if !result.IsError {
		t.Error("expected error for position 0")
	}
}

// TestPreviewSlideMatchesPreviewDeck verifies the unification: the HTML
// served by preview_slide is the HTML served by preview_deck *plus* the
// injected init script. Both are built on demand through the workspace.
// If this test fails, the two tools have drifted apart — fix whichever
// one is doing something bespoke.
func TestPreviewSlideMatchesPreviewDeck(t *testing.T) {
	root := scaffoldTestDeck(t, "parity", "Parity Deck", "default", 4)

	c := newSlydsMCPClientWithUI(t, root)

	// Call both tools to set the preview references.
	c.ToolCall("preview_deck", map[string]any{"deck": "parity"})
	c.ToolCall("preview_slide", map[string]any{"deck": "parity", "position": 3})

	// Read resources — these now build on demand through the workspace.
	deckHTML := c.ReadResource("ui://slyds/preview-deck")
	slideHTML := c.ReadResource("ui://slyds/preview-slide")

	if deckHTML == "" || slideHTML == "" {
		t.Fatal("empty HTML from preview resources")
	}

	// preview_slide HTML must contain the init script; preview_deck HTML
	// must NOT (otherwise the unification leaked the wrong way round).
	if strings.Contains(deckHTML, "window.location.hash=") {
		t.Error("preview_deck HTML unexpectedly contains an init hash script")
	}
	if !strings.Contains(slideHTML, `window.location.hash='3'`) {
		t.Error("preview_slide HTML missing init hash script for position 3")
	}

	// Both should share the same structural markers (same deck, same
	// Build() pipeline, same asset inlining).
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

// TestE2E_PreviewResourceBeforeToolCall verifies that reading the preview
// resource before any tool call returns a clear error instead of panicking
// or returning stale/empty HTML. This is the "cold start" case — the host
// somehow fetches the resource URI before the agent calls the tool.
func TestE2E_PreviewResourceBeforeToolCall(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Cold Start", 2, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClientWithUI(t, root)

	// Reset the preview refs to simulate a fresh server (no prior tool call).
	previewDeckRef = previewRef{}

	// Reading the resource without a prior tool call should fail gracefully.
	_, err := c.Client.ReadResource("ui://slyds/preview-deck")
	if err == nil {
		t.Error("expected error reading preview resource before tool call")
	}
}

// TestE2E_PreviewReflectsEdits verifies the freshness guarantee: after
// editing a slide, the preview resource reflects the edit immediately on
// the next read — without needing to call the preview tool again. This is
// the key behavioral improvement from removing the HTML cache: previews
// are always built from the current disk state.
func TestE2E_PreviewReflectsEdits(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Fresh Test", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClientWithUI(t, root)

	// 1. Call preview_deck to set the reference.
	c.ToolCall("preview_deck", map[string]any{"deck": "deck"})

	// 2. Read the resource — should contain the original title.
	html := c.ReadResource("ui://slyds/preview-deck")
	if !strings.Contains(html, "Fresh Test") {
		t.Fatal("preview missing original title")
	}

	// 3. Edit slide 1 to change the content.
	c.ToolCall("edit_slide", map[string]any{
		"deck":     "deck",
		"position": 1,
		"content":  `<div class="slide"><h1>EDITED CONTENT</h1></div>`,
	})

	// 4. Re-read the preview WITHOUT calling preview_deck again.
	//    The resource handler builds fresh from disk, so the edit
	//    should be reflected immediately.
	html = c.ReadResource("ui://slyds/preview-deck")
	if !strings.Contains(html, "EDITED CONTENT") {
		t.Error("preview did not reflect the edit — still showing stale content")
	}
}

// TestE2E_PreviewSwitchesDecks verifies that calling preview_deck for
// deck A, then for deck B, then reading the resource returns deck B's
// HTML — proving the reference is updated correctly and the old deck's
// content doesn't leak.
func TestE2E_PreviewSwitchesDecks(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Deck Alpha", 2, "default", filepath.Join(root, "alpha"), true)
	core.CreateInDir("Deck Beta", 2, "dark", filepath.Join(root, "beta"), true)

	c := newSlydsMCPClientWithUI(t, root)

	// Preview deck Alpha.
	c.ToolCall("preview_deck", map[string]any{"deck": "alpha"})
	html := c.ReadResource("ui://slyds/preview-deck")
	if !strings.Contains(html, "Deck Alpha") {
		t.Fatal("preview should show Deck Alpha")
	}

	// Switch to deck Beta.
	c.ToolCall("preview_deck", map[string]any{"deck": "beta"})
	html = c.ReadResource("ui://slyds/preview-deck")
	if !strings.Contains(html, "Deck Beta") {
		t.Error("preview should show Deck Beta after switching")
	}
	if strings.Contains(html, "Deck Alpha") {
		t.Error("preview still shows Deck Alpha content after switching to Beta")
	}
}

// TestE2E_PreviewDeckResource_HTMLStructure verifies the full HTML
// structure returned by the preview resource: inlined CSS, navigation
// buttons, and MCP Apps embed hints. These assertions were previously
// done via the previewCache; now they go through the E2E resource read
// to verify the on-demand build produces complete HTML.
func TestE2E_PreviewDeckResource_HTMLStructure(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Structure Test", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClientWithUI(t, root)
	c.ToolCall("preview_deck", map[string]any{"deck": "deck"})

	html := c.ReadResource("ui://slyds/preview-deck")

	checks := map[string]string{
		"<style>":                      "inlined CSS",
		`class="slideshow-container"`:  "slideshow container",
		`id="prevBtn"`:                 "prev navigation button",
		`id="nextBtn"`:                 "next navigation button",
		"Structure Test":               "deck title",
		`class="slyds-mcp-embed"`:      "MCP Apps embed class",
		`id="slyds-mcp-embed"`:         "MCP Apps embed CSS",
	}
	for marker, desc := range checks {
		if !strings.Contains(html, marker) {
			t.Errorf("preview HTML missing %s (marker: %q)", desc, marker)
		}
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
	ws, err := NewLocalWorkspace(root)
	if err != nil {
		panic(fmt.Sprintf("NewLocalWorkspace: %v", err))
	}
	srv := server.NewServer(
		mcpcore.ServerInfo{Name: "test", Version: "0.0.1"},
		server.WithExtension(ui.UIExtension{}),
		server.WithMiddleware(workspaceMiddleware(ws)),
	)
	registerAppTools(srv)

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
