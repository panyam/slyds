package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/panyam/gocurrent"
	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/ext/ui"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/slyds/assets"
	"github.com/panyam/slyds/core"
)

// previewCache stores rendered HTML keyed by resource URI.
// Tool handlers write, resource handlers read.
var previewCache gocurrent.SyncMap[string, string]

// mcpAppsEmbedStyleTag wraps the embedded MCP Apps embed CSS (loaded from
// assets/mcp-embed.css) in a <style> element that applyMCPAppEmbedHints
// injects into preview HTML. Computed once at init to avoid repeating the
// wrapper on every resources/read call.
//
// MCP Apps hosts control outer iframe size via hostContext.containerDimensions
// — not via resources/read _meta (see io.modelcontextprotocol/ui spec). These
// overrides give the host-provided box a scrollable slide area until we ship
// the postMessage `ui/notifications/size-changed` shim (GH issue #75).
var mcpAppsEmbedStyleTag = `<style id="slyds-mcp-embed">` + "\n" + assets.MCPEmbedCSS + `</style>`

// applyMCPAppEmbedHints adds a root class and embed CSS for MCP App iframes.
func applyMCPAppEmbedHints(html string) string {
	html = strings.Replace(html, "<html", "<html class=\"slyds-mcp-embed\"", 1)
	html = strings.Replace(html, "<head>", "<head>\n"+mcpAppsEmbedStyleTag+"\n", 1)
	return html
}

// buildDeckForPreview is the single rendering path for both preview_deck and
// preview_slide. Goes through d.Build() (same templar loader as `slyds serve`),
// then passes through inlineAssets so the result is a self-contained document
// suitable for an MCP Apps resource.
func buildDeckForPreview(d *core.Deck) (*core.Result, error) {
	return d.Build()
}

// injectInitialSlide inserts a small inline script at the start of <body>
// that sets window.location.hash = '#N' before slyds.js runs. slyds.js reads
// the hash in its IIFE init (getSlideFromHash) so the deck opens on slide N.
// Navigation (Prev/Next, hash history) remains fully functional.
//
// Uses goquery so the mutation is DOM-safe (per CONSTRAINTS.md "no regex
// HTML mutation"). The deck HTML coming out of Build() is a full document,
// so the body selector always matches.
func injectInitialSlide(html string, position int) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", fmt.Errorf("parse deck HTML: %w", err)
	}
	body := doc.Find("body").First()
	if body.Length() == 0 {
		return "", fmt.Errorf("deck HTML has no <body>")
	}
	script := fmt.Sprintf(
		`<script>/* slyds: open on slide %d */window.location.hash='%d';</script>`,
		position, position,
	)
	body.PrependHtml(script)
	return doc.Html()
}

// registerAppTools registers MCP Apps (UI extension) tools that render
// slide previews as inline HTML iframes in LLM hosts. Handlers resolve the
// active Workspace from request context so the same registration works
// for localhost and future hosted deployments.
func registerAppTools(srv *server.Server) {
	// preview_deck — full navigable presentation
	ui.RegisterAppTool(srv, ui.AppToolConfig{
		Name:        "preview_deck",
		Description: "Build and preview a full presentation deck rendered with its theme. The host renders the deck as an interactive iframe.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"deck": propString("Deck name (workspace-scoped identifier)"),
			},
			"required": []string{"deck"},
		},
		ResourceURI: "ui://slyds/preview-deck",
		ToolHandler: func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			var p struct {
				Deck string `json:"deck"`
			}
			if err := req.Bind(&p); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
			mcpcore.EmitContent(ctx, req.RequestID, mcpcore.Content{
				Type: "text", Text: fmt.Sprintf("Building preview for %q...", p.Deck),
			})
			result, err := buildDeckForPreview(d)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			previewCache.Store("ui://slyds/preview-deck", result.HTML)

			desc, _ := d.Describe()
			summary := fmt.Sprintf("Built deck %q (%d slides, theme: %s). Preview available.",
				d.Title(), desc.SlideCount, d.Theme())
			if len(result.Warnings) > 0 {
				summary += fmt.Sprintf(" Warnings: %s", strings.Join(result.Warnings, "; "))
			}
			return mcpcore.TextResult(summary), nil
		},
		ResourceHandler: func(ctx context.Context, req mcpcore.ResourceRequest) (mcpcore.ResourceResult, error) {
			html, ok := previewCache.Load("ui://slyds/preview-deck")
			if !ok {
				return mcpcore.ResourceResult{}, fmt.Errorf("no preview available — call preview_deck first")
			}
			return mcpcore.ResourceResult{
				Contents: []mcpcore.ResourceReadContent{{
					URI:      req.URI,
					MimeType: mcpcore.AppMIMEType,
					Text:     applyMCPAppEmbedHints(html),
				}},
			}, nil
		},
		Visibility: []mcpcore.UIVisibility{mcpcore.UIVisibilityModel, mcpcore.UIVisibilityApp},
		Domain:     "slyds",
	})

	// preview_slide — same pipeline as preview_deck, with an init script
	// that opens the deck on the requested slide. The user can still navigate
	// forward/backward from there.
	ui.RegisterAppTool(srv, ui.AppToolConfig{
		Name:        "preview_slide",
		Description: "Preview a presentation deck opened to a specific slide. Uses the same render pipeline as preview_deck; navigation remains functional.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"deck":     propString("Deck name (workspace-scoped identifier)"),
				"position": propInt("Slide position (1-based)"),
			},
			"required": []string{"deck", "position"},
		},
		ResourceURI: "ui://slyds/preview-slide",
		ToolHandler: func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			var p struct {
				Deck     string `json:"deck"`
				Position int    `json:"position"`
			}
			if err := req.Bind(&p); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
			// Validate position against the deck's slide list. This also
			// lets us surface a helpful "out of range" error before spending
			// cycles on the full Build().
			desc, err := d.Describe()
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			if p.Position < 1 || p.Position > desc.SlideCount {
				return mcpcore.ErrorResult(fmt.Sprintf(
					"slide %d out of range (deck has %d slides)", p.Position, desc.SlideCount,
				)), nil
			}

			mcpcore.EmitContent(ctx, req.RequestID, mcpcore.Content{
				Type: "text", Text: fmt.Sprintf("Building preview for %q (slide %d)...", p.Deck, p.Position),
			})
			result, err := buildDeckForPreview(d)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			html, err := injectInitialSlide(result.HTML, p.Position)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			previewCache.Store("ui://slyds/preview-slide", html)

			// Build a text summary for non-UI clients.
			heading := ""
			if content, err := d.GetSlideContent(p.Position); err == nil {
				heading = core.ExtractFirstHeading(content)
			}
			summary := fmt.Sprintf("Preview of %q opened at slide %d/%d (%s). Preview available.",
				d.Title(), p.Position, desc.SlideCount, heading)
			if len(result.Warnings) > 0 {
				summary += fmt.Sprintf(" Warnings: %s", strings.Join(result.Warnings, "; "))
			}
			return mcpcore.TextResult(summary), nil
		},
		ResourceHandler: func(ctx context.Context, req mcpcore.ResourceRequest) (mcpcore.ResourceResult, error) {
			html, ok := previewCache.Load("ui://slyds/preview-slide")
			if !ok {
				return mcpcore.ResourceResult{}, fmt.Errorf("no preview available — call preview_slide first")
			}
			return mcpcore.ResourceResult{
				Contents: []mcpcore.ResourceReadContent{{
					URI:      req.URI,
					MimeType: mcpcore.AppMIMEType,
					Text:     applyMCPAppEmbedHints(html),
				}},
			}, nil
		},
		Visibility: []mcpcore.UIVisibility{mcpcore.UIVisibilityModel, mcpcore.UIVisibilityApp},
		Domain:     "slyds",
	})
}
