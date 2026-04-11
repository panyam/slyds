package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/slyds/core"
)

// registerTools registers all semantic MCP tools on the server using
// single-struct registration (mcpkit v0.1.15). Tool handlers resolve the
// active Workspace from request context — never from a captured path —
// so the same factory works for localhost (single static workspace) and
// for future hosted multi-tenant deployments (per-request workspace).
func registerTools(srv *server.Server) {
	srv.Register(
		listDecksTool(),
		createDeckTool(),
		describeDeckTool(),
		listSlidesTool(),
		readSlideTool(),
		editSlideTool(),
		querySlideTool(),
		addSlideTool(),
		removeSlideTool(),
		checkDeckTool(),
		buildDeckTool(),
	)
}

// --- Result structs ---

// deckSummary is the per-deck entry returned by list_decks.
type deckSummary struct {
	Name   string `json:"name"`
	Title  string `json:"title"`
	Theme  string `json:"theme"`
	Slides int    `json:"slides"`
}

// buildWarningResult is returned by build_deck when the build succeeds
// but produces warnings (e.g. missing assets, unresolved references).
type buildWarningResult struct {
	HTML     string   `json:"html"`
	Warnings []string `json:"warnings"`
}

// --- Tool definitions and handlers ---

func listDecksTool() server.Tool {
	return server.Tool{
		ToolDef: mcpcore.ToolDef{
			Name:        "list_decks",
			Description: "List all presentation decks visible to the current workspace with name, title, theme, and slide count.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		Handler: func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			ws, errResult := requireWorkspace(ctx)
			if errResult != nil {
				return *errResult, nil
			}
			refs, err := ws.ListDecks()
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			var decks []deckSummary
			for _, ref := range refs {
				d, err := ws.OpenDeck(ref.Name)
				if err != nil {
					continue
				}
				count, _ := d.SlideCount()
				decks = append(decks, deckSummary{
					Name:   ref.Name,
					Title:  d.Title(),
					Theme:  d.Theme(),
					Slides: count,
				})
			}
			return jsonResult(decks)
		},
	}
}

func createDeckTool() server.Tool {
	return server.Tool{
		ToolDef: mcpcore.ToolDef{
			Name:        "create_deck",
			Description: "Create a new presentation deck with the given name, title, theme, and slide count.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":   propString("Deck name (becomes the deck identifier in the workspace)"),
					"title":  propString("Presentation title"),
					"theme":  propString("Theme: default, dark, minimal, corporate, hacker"),
					"slides": propInt("Number of slides to scaffold (default: 3)"),
				},
				"required": []string{"name", "title"},
			},
		},
		Handler: func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			ws, errResult := requireWorkspace(ctx)
			if errResult != nil {
				return *errResult, nil
			}
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
			d, err := ws.CreateDeck(p.Name, p.Title, p.Theme, p.Slides)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			mcpcore.NotifyResourcesChanged(ctx)
			desc, err := d.Describe()
			if err != nil {
				return mcpcore.TextResult(fmt.Sprintf("Deck %q created.", p.Name)), nil
			}
			return jsonResult(desc)
		},
	}
}

func describeDeckTool() server.Tool {
	return server.Tool{
		ToolDef: mcpcore.ToolDef{
			Name:        "describe_deck",
			Description: "Get structured metadata for a deck: title, theme, slide list with layouts, word counts, and notes status.",
			InputSchema: deckOnlySchema(),
		},
		Handler: func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			p, err := bindDeckParam(req)
			if err != nil {
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
			return jsonResult(desc)
		},
	}
}

func listSlidesTool() server.Tool {
	return server.Tool{
		ToolDef: mcpcore.ToolDef{
			Name:        "list_slides",
			Description: "List all slides in a deck with filenames, layouts, titles, and word counts.",
			InputSchema: deckOnlySchema(),
		},
		Handler: func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			p, err := bindDeckParam(req)
			if err != nil {
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
			return jsonResult(desc.Slides)
		},
	}
}

func readSlideTool() server.Tool {
	return server.Tool{
		ToolDef: mcpcore.ToolDef{
			Name:        "read_slide",
			Description: "Read the raw HTML content of a slide by position (1-based).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"deck":     propString("Deck name (workspace-scoped identifier)"),
					"position": propInt("Slide position (1-based)"),
				},
				"required": []string{"deck", "position"},
			},
		},
		Handler: func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
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
			content, err := d.GetSlideContent(p.Position)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			return mcpcore.TextResult(content), nil
		},
	}
}

func editSlideTool() server.Tool {
	return server.Tool{
		ToolDef: mcpcore.ToolDef{
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
		},
		Handler: func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			var p struct {
				Deck     string `json:"deck"`
				Position int    `json:"position"`
				Content  string `json:"content"`
			}
			if err := req.Bind(&p); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
			if err := d.EditSlideContent(p.Position, p.Content); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			mcpcore.NotifyResourcesChanged(ctx)
			return mcpcore.TextResult(fmt.Sprintf("Slide %d updated.", p.Position)), nil
		},
	}
}

func querySlideTool() server.Tool {
	return server.Tool{
		ToolDef: mcpcore.ToolDef{
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
		},
		Handler: func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
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
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
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
		},
	}
}

func addSlideTool() server.Tool {
	return server.Tool{
		ToolDef: mcpcore.ToolDef{
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
		},
		Handler: func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
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
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
			if err := d.InsertSlide(p.Position, p.Name, p.Layout, p.Title); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			mcpcore.NotifyResourcesChanged(ctx)
			return mcpcore.TextResult(fmt.Sprintf("Slide %q inserted at position %d.", p.Name, p.Position)), nil
		},
	}
}

func removeSlideTool() server.Tool {
	return server.Tool{
		ToolDef: mcpcore.ToolDef{
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
		},
		Handler: func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			var p struct {
				Deck  string `json:"deck"`
				Slide string `json:"slide"`
			}
			if err := req.Bind(&p); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
			// Resolve slide reference to filename
			filename, err := d.ResolveSlide(p.Slide)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			if err := d.RemoveSlide(filename); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			mcpcore.NotifyResourcesChanged(ctx)
			return mcpcore.TextResult(fmt.Sprintf("Slide %q removed.", filename)), nil
		},
	}
}

func checkDeckTool() server.Tool {
	return server.Tool{
		ToolDef: mcpcore.ToolDef{
			Name:        "check_deck",
			Description: "Validate a deck: check for missing files, broken includes, missing speaker notes, and other issues.",
			InputSchema: deckOnlySchema(),
			Timeout:     10 * time.Second,
		},
		Handler: func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			p, err := bindDeckParam(req)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
			issues, err := d.Check()
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			return jsonResult(issues)
		},
	}
}

func buildDeckTool() server.Tool {
	return server.Tool{
		ToolDef: mcpcore.ToolDef{
			Name:        "build_deck",
			Description: "Build a self-contained HTML file from the deck. Resolves all includes, inlines CSS/JS/images.",
			InputSchema: deckOnlySchema(),
			Timeout:     30 * time.Second,
		},
		Handler: func(ctx context.Context, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			p, err := bindDeckParam(req)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
			result, err := d.Build()
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			// Return build result with warnings if any
			if len(result.Warnings) > 0 {
				return jsonResult(buildWarningResult{
					HTML:     result.HTML,
					Warnings: result.Warnings,
				})
			}
			return mcpcore.TextResult(result.HTML), nil
		},
	}
}

// --- Helpers ---

// deckOnlySchema returns a JSON Schema for tools that only need a deck name.
func deckOnlySchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"deck": propString("Deck name (workspace-scoped identifier, or '.' for a deck at the workspace root)"),
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

// jsonResult marshals v to indented JSON and returns it as a StructuredResult.
// The text content contains the formatted JSON for backward-compatible clients,
// while structuredContent carries the raw Go value for ToolCallTyped consumers.
func jsonResult(v any) (mcpcore.ToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	return mcpcore.StructuredResult(string(data), v), nil
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
