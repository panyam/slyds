package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/slyds/core"
)

// registerTools registers all semantic MCP tools on the server.
func registerTools(srv *server.Server, root string) {
	srv.RegisterTool(createDeckTool(root))
	srv.RegisterTool(describeDeckTool(root))
	srv.RegisterTool(listSlidesTool(root))
	srv.RegisterTool(readSlideTool(root))
	srv.RegisterTool(editSlideTool(root))
	srv.RegisterTool(querySlideTool(root))
	srv.RegisterTool(addSlideTool(root))
	srv.RegisterTool(removeSlideTool(root))
	srv.RegisterTool(checkDeckTool(root))
	srv.RegisterTool(buildDeckTool(root))
}

// --- Tool definitions and handlers ---

func createDeckTool(root string) (mcpcore.ToolDef, mcpcore.ToolHandler) {
	return mcpcore.ToolDef{
			Name:        "create_deck",
			Description: "Create a new presentation deck with the given name, title, theme, and slide count.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":   propString("Deck name (becomes the directory name under deck root)"),
					"title":  propString("Presentation title"),
					"theme":  propString("Theme: default, dark, minimal, corporate, hacker"),
					"slides": propInt("Number of slides to scaffold (default: 3)"),
				},
				"required": []string{"name", "title"},
			},
		}, func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			var p struct {
				Name   string `json:"name"`
				Title  string `json:"title"`
				Theme  string `json:"theme"`
				Slides int    `json:"slides"`
			}
			if err := req.Bind(&p); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			if p.Theme == "" {
				p.Theme = "default"
			}
			if p.Slides < 1 {
				p.Slides = 3
			}
			outDir := filepath.Join(root, p.Name)
			_, err := core.CreateInDir(p.Title, p.Slides, p.Theme, outDir, true)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			// Return the new deck's metadata
			d, err := openDeck(root, p.Name)
			if err != nil {
				return mcpcore.TextResult(fmt.Sprintf("Deck %q created.", p.Name)), nil
			}
			desc, err := d.Describe()
			if err != nil {
				return mcpcore.TextResult(fmt.Sprintf("Deck %q created.", p.Name)), nil
			}
			return jsonResult(desc)
		}
}

func describeDeckTool(root string) (mcpcore.ToolDef, mcpcore.ToolHandler) {
	return mcpcore.ToolDef{
			Name:        "describe_deck",
			Description: "Get structured metadata for a deck: title, theme, slide list with layouts, word counts, and notes status.",
			InputSchema: deckOnlySchema(),
		}, func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			p, err := bindDeckParam(req)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, err := openDeck(root, p.Deck)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			desc, err := d.Describe()
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			return jsonResult(desc)
		}
}

func listSlidesTool(root string) (mcpcore.ToolDef, mcpcore.ToolHandler) {
	return mcpcore.ToolDef{
			Name:        "list_slides",
			Description: "List all slides in a deck with filenames, layouts, titles, and word counts.",
			InputSchema: deckOnlySchema(),
		}, func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			p, err := bindDeckParam(req)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, err := openDeck(root, p.Deck)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			desc, err := d.Describe()
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			return jsonResult(desc.Slides)
		}
}

func readSlideTool(root string) (mcpcore.ToolDef, mcpcore.ToolHandler) {
	return mcpcore.ToolDef{
			Name:        "read_slide",
			Description: "Read the raw HTML content of a slide by position (1-based).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"deck":     propString("Deck name (subdirectory under deck root)"),
					"position": propInt("Slide position (1-based)"),
				},
				"required": []string{"deck", "position"},
			},
		}, func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
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
			content, err := d.GetSlideContent(p.Position)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			return mcpcore.TextResult(content), nil
		}
}

func editSlideTool(root string) (mcpcore.ToolDef, mcpcore.ToolHandler) {
	return mcpcore.ToolDef{
			Name:        "edit_slide",
			Description: "Replace the HTML content of a slide at the given position.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"deck":     propString("Deck name"),
					"position": propInt("Slide position (1-based)"),
					"content":  propString("New HTML content for the slide"),
				},
				"required": []string{"deck", "position", "content"},
			},
		}, func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			var p struct {
				Deck     string `json:"deck"`
				Position int    `json:"position"`
				Content  string `json:"content"`
			}
			if err := req.Bind(&p); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, err := openDeck(root, p.Deck)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			if err := d.EditSlideContent(p.Position, p.Content); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			return mcpcore.TextResult(fmt.Sprintf("Slide %d updated.", p.Position)), nil
		}
}

func querySlideTool(root string) (mcpcore.ToolDef, mcpcore.ToolHandler) {
	return mcpcore.ToolDef{
			Name:        "query_slide",
			Description: "Query or modify slide HTML using CSS selectors (goquery). Read text, attributes, inner HTML, or mutate content.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"deck":     propString("Deck name"),
					"slide":    propString("Slide reference: position number (e.g. '1') or filename substring"),
					"selector": propString("CSS selector (e.g. 'h1', '.slide-body', 'img')"),
					"html":     propBool("Return inner HTML instead of text"),
					"attr":     propString("Return the value of this attribute"),
					"count":    propBool("Return match count instead of content"),
					"set":      propString("Set inner text of matched elements"),
					"set_html": propString("Set inner HTML of matched elements"),
					"set_attr": propString("Set attribute (NAME=VALUE format)"),
					"append":   propString("Append child HTML to matched elements"),
					"remove":   propBool("Remove matched elements"),
					"all":      propBool("Apply to all matches (default: first only)"),
				},
				"required": []string{"deck", "slide", "selector"},
			},
		}, func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			var p struct {
				Deck     string  `json:"deck"`
				Slide    string  `json:"slide"`
				Selector string  `json:"selector"`
				HTML     bool    `json:"html"`
				Attr     string  `json:"attr"`
				Count    bool    `json:"count"`
				Set      *string `json:"set"`
				SetHTML  *string `json:"set_html"`
				SetAttr  *string `json:"set_attr"`
				Append   *string `json:"append"`
				Remove   bool    `json:"remove"`
				All      bool    `json:"all"`
			}
			if err := req.Bind(&p); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, err := openDeck(root, p.Deck)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			opts := core.QueryOpts{
				HTML:    p.HTML,
				Attr:    p.Attr,
				Count:   p.Count,
				Set:     p.Set,
				SetHTML: p.SetHTML,
				SetAttr: p.SetAttr,
				Append:  p.Append,
				Remove:  p.Remove,
				All:     p.All,
			}
			results, err := d.Query(p.Slide, p.Selector, opts)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			return jsonResult(results)
		}
}

func addSlideTool(root string) (mcpcore.ToolDef, mcpcore.ToolHandler) {
	return mcpcore.ToolDef{
			Name:        "add_slide",
			Description: "Insert a new slide at the given position using a layout template.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"deck":     propString("Deck name"),
					"position": propInt("Position to insert at (1-based)"),
					"name":     propString("Slide filename (without .html extension or number prefix)"),
					"layout":   propString("Layout template: title, content, two-col, section, blank, closing"),
					"title":    propString("Slide title (used in template rendering)"),
				},
				"required": []string{"deck", "position", "name"},
			},
		}, func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			var p struct {
				Deck     string `json:"deck"`
				Position int    `json:"position"`
				Name     string `json:"name"`
				Layout   string `json:"layout"`
				Title    string `json:"title"`
			}
			if err := req.Bind(&p); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			if p.Layout == "" {
				p.Layout = "content"
			}
			d, err := openDeck(root, p.Deck)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			if err := d.InsertSlide(p.Position, p.Name, p.Layout, p.Title); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			return mcpcore.TextResult(fmt.Sprintf("Slide %q inserted at position %d.", p.Name, p.Position)), nil
		}
}

func removeSlideTool(root string) (mcpcore.ToolDef, mcpcore.ToolHandler) {
	return mcpcore.ToolDef{
			Name:        "remove_slide",
			Description: "Remove a slide by filename or position number. Remaining slides are renumbered.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"deck":  propString("Deck name"),
					"slide": propString("Slide filename (e.g. '02-slide.html') or position number"),
				},
				"required": []string{"deck", "slide"},
			},
		}, func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			var p struct {
				Deck  string `json:"deck"`
				Slide string `json:"slide"`
			}
			if err := req.Bind(&p); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, err := openDeck(root, p.Deck)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			// Resolve slide reference to filename
			filename, err := d.ResolveSlide(p.Slide)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			if err := d.RemoveSlide(filename); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			return mcpcore.TextResult(fmt.Sprintf("Slide %q removed.", filename)), nil
		}
}

func checkDeckTool(root string) (mcpcore.ToolDef, mcpcore.ToolHandler) {
	return mcpcore.ToolDef{
			Name:        "check_deck",
			Description: "Validate a deck: check for missing files, broken includes, missing speaker notes, and other issues.",
			InputSchema: deckOnlySchema(),
		}, func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			p, err := bindDeckParam(req)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, err := openDeck(root, p.Deck)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			issues, err := d.Check()
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			return jsonResult(issues)
		}
}

func buildDeckTool(root string) (mcpcore.ToolDef, mcpcore.ToolHandler) {
	return mcpcore.ToolDef{
			Name:        "build_deck",
			Description: "Build a self-contained HTML file from the deck. Resolves all includes, inlines CSS/JS/images.",
			InputSchema: deckOnlySchema(),
		}, func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			p, err := bindDeckParam(req)
			if err != nil {
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
			// Return build result with warnings if any
			if len(result.Warnings) > 0 {
				out := map[string]any{
					"html":     result.HTML,
					"warnings": result.Warnings,
				}
				return jsonResult(out)
			}
			return mcpcore.TextResult(result.HTML), nil
		}
}

// --- Helpers ---

// deckOnlySchema returns a JSON Schema for tools that only need a deck name.
func deckOnlySchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"deck": propString("Deck name (subdirectory under deck root, or '.' for root deck)"),
		},
		"required": []string{"deck"},
	}
}

type deckParam struct {
	Deck string `json:"deck"`
}

func bindDeckParam(req mcpcore.ToolRequest) (deckParam, error) {
	var p deckParam
	if err := req.Bind(&p); err != nil {
		return p, err
	}
	if p.Deck == "" {
		return p, fmt.Errorf("deck is required")
	}
	return p, nil
}

func jsonResult(v any) (mcpcore.ToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	return mcpcore.TextResult(string(data)), nil
}

func propString(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func propInt(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}

func propBool(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}
