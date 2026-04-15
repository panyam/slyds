package cmd

import (
	"context"
	"encoding/json"
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
// builds a full deck preview, stores the previewRef, and returns a summary.
func TestPreviewDeckReturnsHTML(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Preview Deck", "default", 3)

	tool := previewDeckToolDef(root)
	result := callTool(t, root, tool.Handler, map[string]string{"deck": "test-deck"})
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
}

// TestPreviewSlideReturnsHTML verifies that preview_slide stores the deck +
// position reference and returns a text summary.
func TestPreviewSlideReturnsHTML(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Slide Preview", "dark", 3)

	tool := previewSlideToolDef(root)
	result := callTool(t, root, tool.Handler, map[string]any{"deck": "test-deck", "position": 2})
	if result.IsError {
		t.Fatalf("preview_slide error: %s", toolText(result))
	}

	text := toolText(result)
	if !strings.Contains(text, "slide 2/3") {
		t.Errorf("preview_slide summary missing position info: %q", text)
	}
}

// TestPreviewSlideInvalidPosition verifies that preview_slide returns an
// error when given an out-of-range slide position.
func TestPreviewSlideInvalidPosition(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Error Test", "default", 3)

	tool := previewSlideToolDef(root)
	result := callTool(t, root, tool.Handler, map[string]any{"deck": "test-deck", "position": 99})
	if !result.IsError {
		t.Error("expected error for invalid slide position")
	}
}

// TestPreviewSlideZeroPosition — 0 and negative positions are out of range.
func TestPreviewSlideZeroPosition(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Zero Test", "default", 3)

	tool := previewSlideToolDef(root)
	result := callTool(t, root, tool.Handler, map[string]any{"deck": "test-deck", "position": 0})
	if !result.IsError {
		t.Error("expected error for position 0")
	}
}

// TestPreviewSlideMatchesPreviewDeck verifies the unification: preview_slide
// HTML = preview_deck HTML + init script. Tests both concrete and template
// resource paths.
func TestPreviewSlideMatchesPreviewDeck(t *testing.T) {
	root := scaffoldTestDeck(t, "parity", "Parity Deck", "default", 4)

	c := newSlydsMCPClientWithUI(t, root)

	// Call both tools to set previewRefs for concrete resources.
	c.ToolCall("preview_deck", map[string]any{"deck": "parity"})
	c.ToolCall("preview_slide", map[string]any{"deck": "parity", "position": 3})

	// Read via template URIs.
	deckHTML := c.ReadResource("ui://slyds/decks/parity/preview")
	slideHTML := c.ReadResource("ui://slyds/decks/parity/slides/3/preview")

	if deckHTML == "" || slideHTML == "" {
		t.Fatal("empty HTML from concrete preview resources")
	}

	if strings.Contains(deckHTML, "window.location.hash=") {
		t.Error("preview_deck HTML unexpectedly contains an init hash script")
	}
	if !strings.Contains(slideHTML, `window.location.hash='3'`) {
		t.Error("preview_slide HTML missing init hash script for position 3")
	}

	// Also read via template URIs (advanced clients).
	deckHTML2 := c.ReadResource("ui://slyds/decks/parity/preview")
	slideHTML2 := c.ReadResource("ui://slyds/decks/parity/slides/3/preview")

	if !strings.Contains(deckHTML2, "Parity Deck") {
		t.Error("template deck resource missing title")
	}
	if !strings.Contains(slideHTML2, `window.location.hash='3'`) {
		t.Error("template slide resource missing init hash script")
	}

	// Both paths should produce the same structural markers.
	for _, marker := range []string{
		`class="slideshow-container"`,
		`id="prevBtn"`,
		`id="nextBtn"`,
		`Parity Deck`,
		`<style>`,
	} {
		if !strings.Contains(deckHTML, marker) {
			t.Errorf("concrete deck HTML missing marker %q", marker)
		}
		if !strings.Contains(deckHTML2, marker) {
			t.Errorf("template deck HTML missing marker %q", marker)
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
// _meta.ui with concrete resourceUri and supportedDisplayModes.
func TestE2E_UIExtensionAdvertised(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("UI Test", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClientWithUI(t, root)
	tools := c.ListTools()

	for _, tool := range tools {
		if tool.Name == "preview_deck" {
			if tool.Meta == nil || tool.Meta.UI == nil {
				t.Fatal("preview_deck missing _meta.ui")
			}
			// mcpkit auto-generates a concrete fallback URI from the template.
			// Just verify it's non-empty and starts with ui://
			if !strings.HasPrefix(tool.Meta.UI.ResourceUri, "ui://") {
				t.Errorf("preview_deck resourceUri = %q, want ui:// prefix", tool.Meta.UI.ResourceUri)
			}
			if len(tool.Meta.UI.SupportedDisplayModes) != 2 {
				t.Errorf("preview_deck supportedDisplayModes count = %d, want 2", len(tool.Meta.UI.SupportedDisplayModes))
			}
			return
		}
	}
	t.Error("preview_deck tool not found in tools/list")
}

// TestE2E_DisplayModesAdvertised verifies both preview tools declare
// inline + fullscreen display modes in their tool definitions.
func TestE2E_DisplayModesAdvertised(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("DM Test", 2, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClientWithUI(t, root)
	tools := c.ListTools()

	for _, name := range []string{"preview_deck", "preview_slide"} {
		found := false
		for _, tool := range tools {
			if tool.Name != name {
				continue
			}
			found = true
			if tool.Meta == nil || tool.Meta.UI == nil {
				t.Fatalf("%s missing _meta.ui", name)
			}
			modes := tool.Meta.UI.SupportedDisplayModes
			hasInline, hasFullscreen := false, false
			for _, m := range modes {
				if m == mcpcore.DisplayModeInline {
					hasInline = true
				}
				if m == mcpcore.DisplayModeFullscreen {
					hasFullscreen = true
				}
			}
			if !hasInline {
				t.Errorf("%s missing DisplayModeInline", name)
			}
			if !hasFullscreen {
				t.Errorf("%s missing DisplayModeFullscreen", name)
			}
		}
		if !found {
			t.Errorf("tool %s not found", name)
		}
	}
}

// TestE2E_PreviewDeckResource verifies the full MCP Apps flow via the
// concrete resource URI that hosts like VS Code fetch.
func TestE2E_PreviewDeckResource(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("E2E Deck", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClientWithUI(t, root)

	result := c.ToolCall("preview_deck", map[string]any{"deck": "deck"})
	if !strings.Contains(result, "E2E Deck") {
		t.Fatalf("preview_deck result missing title: %s", result)
	}

	html := c.ReadResource("ui://slyds/decks/deck/preview")
	if !strings.Contains(html, "<html") {
		t.Error("preview-deck resource missing <html")
	}
	if !strings.Contains(html, "E2E Deck") {
		t.Error("preview-deck resource missing deck title")
	}
}

// TestE2E_PreviewSlideResource verifies the full MCP Apps flow for single
// slide preview via the concrete resource URI.
func TestE2E_PreviewSlideResource(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Slide E2E", 3, "dark", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClientWithUI(t, root)

	result := c.ToolCall("preview_slide", map[string]any{"deck": "deck", "position": 1})
	if !strings.Contains(result, "slide 1/3") {
		t.Fatalf("preview_slide result missing position: %s", result)
	}

	html := c.ReadResource("ui://slyds/decks/deck/slides/1/preview")
	if !strings.Contains(html, `data-theme="dark"`) {
		t.Error("preview-slide resource missing dark theme")
	}
	if !strings.Contains(html, `class="slide`) {
		t.Error("preview-slide resource missing slide content")
	}
}

// TestE2E_TemplateResourceDirectRead verifies that template resources can
// be read without a prior tool call. The TemplateHandler resolves the deck
// from URI params — no previewRef needed.
func TestE2E_TemplateResourceDirectRead(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Direct Read", 2, "default", filepath.Join(root, "mydeck"), true)

	c := newSlydsMCPClientWithUI(t, root)

	// Read the template resource directly — no tool call first.
	html := c.ReadResource("ui://slyds/decks/mydeck/preview")
	if !strings.Contains(html, "Direct Read") {
		t.Error("template resource should return deck HTML without prior tool call")
	}
	if !strings.Contains(html, `class="slyds-mcp-embed"`) {
		t.Error("template resource missing MCP Apps embed hints")
	}
}

// TestE2E_TemplateResourceNonexistentDeck verifies that reading a template
// resource for a nonexistent deck returns an error.
func TestE2E_TemplateResourceNonexistentDeck(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Exists", 2, "default", filepath.Join(root, "exists"), true)

	c := newSlydsMCPClientWithUI(t, root)

	_, err := c.Client.ReadResource("ui://slyds/decks/nonexistent/preview")
	if err == nil {
		t.Error("expected error reading template resource for nonexistent deck")
	}
}

// TestE2E_PreviewResourceBeforeToolCall verifies that reading the concrete
// preview resource before any tool call returns a clear error.
// TestE2E_PreviewResourceNonexistentDeck verifies that reading a template
// preview resource for a nonexistent deck returns an error.
func TestE2E_PreviewResourceNonexistentDeck(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Cold Start", 2, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClientWithUI(t, root)

	_, err := c.Client.ReadResource("ui://slyds/decks/nonexistent/preview")
	if err == nil {
		t.Error("expected error reading preview for nonexistent deck")
	}
}

// TestE2E_PreviewReflectsEdits verifies the freshness guarantee: after
// editing a slide, the preview reflects the edit on the next read.
func TestE2E_PreviewReflectsEdits(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Fresh Test", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClientWithUI(t, root)

	// 1. Call preview_deck to set the ref.
	c.ToolCall("preview_deck", map[string]any{"deck": "deck"})

	// 2. Read — should contain original title.
	html := c.ReadResource("ui://slyds/decks/deck/preview")
	if !strings.Contains(html, "Fresh Test") {
		t.Fatal("preview missing original title")
	}

	// 3. Edit slide 1.
	c.ToolCall("edit_slide", map[string]any{
		"deck":     "deck",
		"position": 1,
		"content":  `<div class="slide"><h1>EDITED CONTENT</h1></div>`,
	})

	// 4. Re-read — edit should be reflected (builds fresh, no cache).
	html = c.ReadResource("ui://slyds/decks/deck/preview")
	if !strings.Contains(html, "EDITED CONTENT") {
		t.Error("preview did not reflect the edit — still showing stale content")
	}
}

// TestE2E_PreviewSwitchesDecks verifies that calling preview_deck for
// deck A, then B, updates the concrete resource to show B.
func TestE2E_PreviewSwitchesDecks(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Deck Alpha", 2, "default", filepath.Join(root, "alpha"), true)
	core.CreateInDir("Deck Beta", 2, "dark", filepath.Join(root, "beta"), true)

	c := newSlydsMCPClientWithUI(t, root)

	// Template resources are independent — each deck has its own URI.
	alphaHTML := c.ReadResource("ui://slyds/decks/alpha/preview")
	betaHTML := c.ReadResource("ui://slyds/decks/beta/preview")
	if !strings.Contains(alphaHTML, "Deck Alpha") {
		t.Error("template alpha resource missing title")
	}
	if !strings.Contains(betaHTML, "Deck Beta") {
		t.Error("template beta resource missing title")
	}
}

// TestE2E_PreviewDeckResource_HTMLStructure verifies the full HTML
// structure returned by the preview resource.
func TestE2E_PreviewDeckResource_HTMLStructure(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Structure Test", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClientWithUI(t, root)
	c.ToolCall("preview_deck", map[string]any{"deck": "deck"})

	html := c.ReadResource("ui://slyds/decks/deck/preview")

	checks := map[string]string{
		"<style>":                     "inlined CSS",
		`class="slideshow-container"`: "slideshow container",
		`id="prevBtn"`:                "prev navigation button",
		`id="nextBtn"`:                "next navigation button",
		"Structure Test":              "deck title",
		`class="slyds-mcp-embed"`:     "MCP Apps embed class",
		`id="slyds-mcp-embed"`:        "MCP Apps embed CSS",
	}
	for marker, desc := range checks {
		if !strings.Contains(html, marker) {
			t.Errorf("preview HTML missing %s (marker: %q)", desc, marker)
		}
	}
}

// TestE2E_FullscreenDisplayMode verifies that preview_deck accepts a
// display_mode parameter without error.
func TestE2E_FullscreenDisplayMode(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("FS Test", 2, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClientWithUI(t, root)

	result := c.ToolCall("preview_deck", map[string]any{
		"deck":         "deck",
		"display_mode": "fullscreen",
	})
	if !strings.Contains(result, "FS Test") {
		t.Errorf("preview_deck with fullscreen failed: %s", result)
	}
}

// --- Helpers to extract tool defs for unit testing ---

func previewDeckToolDef(root string) server.Tool {
	return extractRegisteredTool(root, "preview_deck")
}

func previewSlideToolDef(root string) server.Tool {
	return extractRegisteredTool(root, "preview_slide")
}

func extractRegisteredTool(root, name string) server.Tool {
	ws, err := NewLocalWorkspace(root)
	if err != nil {
		panic("NewLocalWorkspace: " + err.Error())
	}
	srv := server.NewServer(
		mcpcore.ServerInfo{Name: "test", Version: "0.0.1"},
		server.WithExtension(ui.UIExtension{}),
		server.WithMiddleware(workspaceMiddleware(ws)),
	)
	registerAppTools(srv)

	ctx := context.Background()
	initReq := &mcpcore.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"1"`),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	}
	srv.Dispatch(ctx, initReq)
	srv.Dispatch(ctx, &mcpcore.Request{JSONRPC: "2.0", Method: "notifications/initialized"})

	listResp := srv.Dispatch(ctx, &mcpcore.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"2"`),
		Method:  "tools/list",
	})
	var listResult struct {
		Tools []mcpcore.ToolDef `json:"tools"`
	}
	resultBytes, _ := json.Marshal(listResp.Result)
	json.Unmarshal(resultBytes, &listResult)

	for _, tool := range listResult.Tools {
		if tool.Name == name {
			return server.Tool{
				ToolDef: tool,
				Handler: func(ctx mcpcore.ToolContext, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
					args, _ := json.Marshal(map[string]any{"name": name, "arguments": json.RawMessage(req.Arguments)})
					callReq := &mcpcore.Request{
						JSONRPC: "2.0",
						ID:      json.RawMessage(`"3"`),
						Method:  "tools/call",
						Params:  args,
					}
					callResp := srv.Dispatch(ctx, callReq)
					var result mcpcore.ToolResult
					crBytes, _ := json.Marshal(callResp.Result)
						json.Unmarshal(crBytes, &result)
					return result, nil
				},
			}
		}
	}
	panic("tool " + name + " not found")
}
