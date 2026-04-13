package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/ext/ui"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/slyds/assets"
	"github.com/panyam/slyds/core"
)

// previewRef stores which deck (and optional slide position) a preview
// resource should render. Written by tool handlers, read by the concrete
// resource handlers that hosts like VS Code fetch from _meta.ui.resourceUri.
//
// Template resource handlers (ui://slyds/decks/{deck}/preview) extract the
// deck name from URI params and don't use these refs at all.
type previewRef struct {
	Deck     string
	Position int // 0 = full deck, >0 = open on this slide
}

var previewDeckRef previewRef
var previewSlideRef previewRef

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

// buildPreviewHTML builds a preview HTML for the given deck, optionally
// opening on a specific slide. Used by both concrete and template resource
// handlers.
func buildPreviewHTML(ctx context.Context, deckName string, position int) (string, error) {
	d, err := openDeckForResource(ctx, deckName)
	if err != nil {
		return "", err
	}
	result, err := buildDeckForPreview(d)
	if err != nil {
		return "", err
	}
	html := result.HTML
	if position > 0 {
		html, err = injectInitialSlide(html, position)
		if err != nil {
			return "", err
		}
	}
	return applyMCPAppEmbedHints(html), nil
}

// buildPreviewForRef builds preview HTML from a previewRef. Used by the
// concrete resource handlers that current hosts (VS Code, Claude Desktop)
// fetch via the literal _meta.ui.resourceUri.
func buildPreviewForRef(ctx context.Context, ref previewRef) (string, error) {
	if ref.Deck == "" {
		return "", fmt.Errorf("no preview available — call preview_deck or preview_slide first")
	}
	return buildPreviewHTML(ctx, ref.Deck, ref.Position)
}

// previewDisplayModes is the set of display modes supported by slyds preview
// tools. Declared as inline/fullscreen so hosts can offer mode switching.
var previewDisplayModes = []mcpcore.DisplayMode{
	mcpcore.DisplayModeInline,
	mcpcore.DisplayModeFullscreen,
}

// registerAppTools registers MCP Apps (UI extension) tools that render
// slide previews as inline HTML iframes in LLM hosts. Handlers resolve the
// active Workspace from request context so the same registration works
// for localhost and future hosted deployments.
//
// Each tool is registered with a concrete resourceUri (e.g. ui://slyds/preview-deck)
// because current hosts (VS Code, Claude Desktop) fetch _meta.ui.resourceUri
// literally — they don't substitute template variables. The concrete resource
// handler reads from previewRef (set by the tool handler).
//
// Template resources (ui://slyds/decks/{deck}/preview) are registered
// separately for advanced clients that construct URIs from tool arguments.
// These extract the deck name from URI params and don't use previewRef.
//
// TODO(mcpkit#213): once RegisterAppTool auto-generates a concrete fallback
// for template URIs, remove previewRef and switch to template-only registration.
func registerAppTools(srv *server.Server) {
	// preview_deck — full navigable presentation
	ui.RegisterAppTool(srv, ui.AppToolConfig{
		Name:        "preview_deck",
		Description: "Build and preview a full presentation deck rendered with its theme. The host renders the deck as an interactive iframe. Pass display_mode='fullscreen' for presentation mode.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"deck":         propString("Deck name (workspace-scoped identifier)"),
				"display_mode": propString("Display mode: 'inline' (default) or 'fullscreen' for presentation mode"),
			},
			"required": []string{"deck"},
		},
		ResourceURI:           "ui://slyds/preview-deck",
		SupportedDisplayModes: previewDisplayModes,
		ToolHandler: func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			var p struct {
				Deck        string `json:"deck"`
				DisplayMode string `json:"display_mode"`
			}
			if err := req.Bind(&p); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}

			// Store the deck reference early — hosts may fetch the resource
			// URI in parallel with the tool call, before the handler returns.
			previewDeckRef = previewRef{Deck: p.Deck}

			mcpcore.EmitContent(ctx, req.RequestID, mcpcore.Content{
				Type: "text", Text: fmt.Sprintf("Building preview for %q...", p.Deck),
			})
			result, err := buildDeckForPreview(d)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}

			// Request fullscreen if the agent asked for presentation mode.
			if p.DisplayMode == "fullscreen" {
				ui.RequestDisplayMode(ctx, mcpcore.DisplayModeFullscreen)
			}

			desc, _ := d.Describe()
			summary := fmt.Sprintf("Built deck %q (%d slides, theme: %s). Preview available.",
				d.Title(), desc.SlideCount, d.Theme())
			if len(result.Warnings) > 0 {
				summary += fmt.Sprintf(" Warnings: %s", strings.Join(result.Warnings, "; "))
			}
			return mcpcore.TextResult(summary), nil
		},
		ResourceHandler: func(ctx context.Context, req mcpcore.ResourceRequest) (mcpcore.ResourceResult, error) {
			html, err := buildPreviewForRef(ctx, previewDeckRef)
			if err != nil {
				return mcpcore.ResourceResult{}, err
			}
			return mcpcore.ResourceResult{
				Contents: []mcpcore.ResourceReadContent{{
					URI:      req.URI,
					MimeType: mcpcore.AppMIMEType,
					Text:     html,
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
		ResourceURI:           "ui://slyds/preview-slide",
		SupportedDisplayModes: previewDisplayModes,
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
			desc, err := d.Describe()
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			if p.Position < 1 || p.Position > desc.SlideCount {
				return mcpcore.ErrorResult(fmt.Sprintf(
					"slide %d out of range (deck has %d slides)", p.Position, desc.SlideCount,
				)), nil
			}

			// Store the ref early — hosts may fetch the resource in parallel.
			previewSlideRef = previewRef{Deck: p.Deck, Position: p.Position}

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
			_ = html

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
			html, err := buildPreviewForRef(ctx, previewSlideRef)
			if err != nil {
				return mcpcore.ResourceResult{}, err
			}
			return mcpcore.ResourceResult{
				Contents: []mcpcore.ResourceReadContent{{
					URI:      req.URI,
					MimeType: mcpcore.AppMIMEType,
					Text:     html,
				}},
			}, nil
		},
		Visibility: []mcpcore.UIVisibility{mcpcore.UIVisibilityModel, mcpcore.UIVisibilityApp},
		Domain:     "slyds",
	})

	// Template resources — registered separately for clients that can
	// construct concrete URIs from tool arguments. These don't require
	// a prior tool call; the deck name (and position) come from the URI.
	srv.Register(
		server.ResourceTemplate{
			ResourceTemplate: mcpcore.ResourceTemplate{
				URITemplate: "ui://slyds/decks/{deck}/preview",
				Name:        "Deck Preview",
				Description: "Full navigable deck preview by name (MCP Apps)",
				MimeType:    mcpcore.AppMIMEType,
			},
			Handler: func(ctx context.Context, uri string, params map[string]string) (mcpcore.ResourceResult, error) {
				html, err := buildPreviewHTML(ctx, params["deck"], 0)
				if err != nil {
					return mcpcore.ResourceResult{}, err
				}
				return mcpcore.ResourceResult{
					Contents: []mcpcore.ResourceReadContent{{
						URI:      uri,
						MimeType: mcpcore.AppMIMEType,
						Text:     html,
					}},
				}, nil
			},
		},
		server.ResourceTemplate{
			ResourceTemplate: mcpcore.ResourceTemplate{
				URITemplate: "ui://slyds/decks/{deck}/slides/{position}/preview",
				Name:        "Slide Preview",
				Description: "Deck preview opened on a specific slide (MCP Apps)",
				MimeType:    mcpcore.AppMIMEType,
			},
			Handler: func(ctx context.Context, uri string, params map[string]string) (mcpcore.ResourceResult, error) {
				position := 0
				if posStr, ok := params["position"]; ok {
					n, err := strconv.Atoi(posStr)
					if err != nil {
						return mcpcore.ResourceResult{}, fmt.Errorf("invalid slide position %q", posStr)
					}
					position = n
				}
				html, err := buildPreviewHTML(ctx, params["deck"], position)
				if err != nil {
					return mcpcore.ResourceResult{}, err
				}
				return mcpcore.ResourceResult{
					Contents: []mcpcore.ResourceReadContent{{
						URI:      uri,
						MimeType: mcpcore.AppMIMEType,
						Text:     html,
					}},
				}, nil
			},
		},
	)
}
