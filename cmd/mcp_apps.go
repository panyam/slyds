package cmd

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"strings"

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

// slidePreviewTmpl is parsed once from the embedded template.
var slidePreviewTmpl = func() *template.Template {
	tmplFS, _ := fs.Sub(assets.TemplatesFS, "templates")
	return template.Must(template.ParseFS(tmplFS, "slide-preview.html.tmpl"))
}()

// registerAppTools registers MCP Apps (UI extension) tools that render
// slide previews as inline HTML iframes in LLM hosts.
func registerAppTools(srv *server.Server, root string) {
	// preview_deck — full navigable presentation
	ui.RegisterAppTool(srv, ui.AppToolConfig{
		Name:        "preview_deck",
		Description: "Build and preview a full presentation deck rendered with its theme. The host renders the deck as an interactive iframe.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"deck": propString("Deck name (subdirectory under deck root)"),
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
			d, err := openDeck(root, p.Deck)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			result, err := d.Build()
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
					Text:     html,
				}},
			}, nil
		},
		Visibility: []mcpcore.UIVisibility{mcpcore.UIVisibilityModel, mcpcore.UIVisibilityApp},
		Domain:     "slyds",
	})

	// preview_slide — single slide with theme
	ui.RegisterAppTool(srv, ui.AppToolConfig{
		Name:        "preview_slide",
		Description: "Preview a single slide rendered with its deck theme. The host renders the slide as an inline iframe.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"deck":     propString("Deck name (subdirectory under deck root)"),
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
			d, err := openDeck(root, p.Deck)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			html, err := renderSlidePreview(d, p.Position)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			previewCache.Store("ui://slyds/preview-slide", html)

			// Build a text summary for non-UI clients
			content, _ := d.GetSlideContent(p.Position)
			layout := core.DetectLayout(content)
			heading := core.ExtractFirstHeading(content)
			summary := fmt.Sprintf("Slide %d of %q (%s: %s). Preview available.",
				p.Position, d.Title(), layout, heading)
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
					Text:     html,
				}},
			}, nil
		},
		Visibility: []mcpcore.UIVisibility{mcpcore.UIVisibilityModel, mcpcore.UIVisibilityApp},
		Domain:     "slyds",
	})
}

// slidePreviewData is the template context for slide-preview.html.tmpl.
type slidePreviewData struct {
	Title    string
	Theme    string
	BaseCSS  template.HTML
	ThemeCSS template.HTML
	SlideHTML template.HTML
}

// renderSlidePreview renders a single slide as a self-contained HTML page
// with the deck's theme CSS inlined.
func renderSlidePreview(d *core.Deck, position int) (string, error) {
	slideHTML, err := d.GetSlideContent(position)
	if err != nil {
		return "", err
	}

	// Read deck's rendered theme.css
	themeCSS, _ := d.FS.ReadFile("theme.css")

	// Read base theme CSS from embedded assets
	themeFiles := assets.ThemeFiles()
	var cssBuilder strings.Builder
	if base, ok := themeFiles["_base.css"]; ok {
		cssBuilder.WriteString(base)
		cssBuilder.WriteString("\n")
	}
	if named, ok := themeFiles[d.Theme()+".css"]; ok {
		cssBuilder.WriteString(named)
		cssBuilder.WriteString("\n")
	}
	cssBuilder.Write(themeCSS)

	heading := core.ExtractFirstHeading(slideHTML)
	if heading == "" {
		heading = fmt.Sprintf("Slide %d", position)
	}

	data := slidePreviewData{
		Title:    heading + " — " + d.Title(),
		Theme:    d.Theme(),
		BaseCSS:  template.HTML(assets.SlydsCSS),
		ThemeCSS: template.HTML(cssBuilder.String()),
		SlideHTML: template.HTML(slideHTML),
	}

	var buf strings.Builder
	if err := slidePreviewTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render slide preview: %w", err)
	}
	return buf.String(), nil
}
