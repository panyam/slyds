package cmd

import (
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

// slideReadResult is the structured response from read_slide, including
// the content version for optimistic concurrency.
type slideReadResult struct {
	Content     string `json:"content"`
	Version     string `json:"version"`
	DeckVersion string `json:"deck_version"`
}

// slideEditResult is the structured response from edit_slide after a
// successful write, including the new content version.
type slideEditResult struct {
	Message     string `json:"message"`
	Version     string `json:"version"`
	DeckVersion string `json:"deck_version"`
}

// versionConflict is returned as an error result when expected_version
// doesn't match the current content. Includes the current state so the
// agent can recover without an extra round-trip.
type versionConflict struct {
	Error          string `json:"error"`
	CurrentVersion string `json:"current_version"`
	CurrentContent string `json:"current_content,omitempty"`
	DeckVersion    string `json:"deck_version,omitempty"`
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
		Handler: func(ctx mcpcore.ToolContext, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
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
			return jsonResult(map[string]any{"decks": decks})
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
		Handler: func(ctx mcpcore.ToolContext, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
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
			mcpcore.NotifyResourceUpdated(ctx, "ui://slyds/decks/"+p.Name+"/preview")
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
		Handler: func(ctx mcpcore.ToolContext, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
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
		Handler: func(ctx mcpcore.ToolContext, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
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
			return jsonResult(map[string]any{"slides": desc.Slides})
		},
	}
}

func readSlideTool() server.Tool {
	return server.Tool{
		ToolDef: mcpcore.ToolDef{
			Name:        "read_slide",
			Description: "Read the raw HTML content of a slide. Supply either 'slide' (preferred: slug, filename, or position as string) or 'position' (legacy: 1-based integer).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"deck":     propString("Deck name (workspace-scoped identifier)"),
					"slide":    propString("Slide reference: slug (e.g. 'metrics'), filename (e.g. '02-metrics.html'), or position number as string"),
					"position": propInt("Slide position (1-based). Legacy — prefer 'slide' for stability across inserts."),
				},
				"required": []string{"deck"},
			},
		},
		Handler: func(ctx mcpcore.ToolContext, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			var p struct {
				Deck     string `json:"deck"`
				Slide    string `json:"slide"`
				Position int    `json:"position"`
			}
			if err := req.Bind(&p); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
			pos, err := resolveSlidePosition(d, p.Slide, p.Position)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			content, err := d.GetSlideContent(pos)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			ver, _ := d.SlideVersion(pos)
			deckVer, _ := d.DeckVersion()
			return jsonResult(slideReadResult{
				Content:     content,
				Version:     ver,
				DeckVersion: deckVer,
			})
		},
	}
}

func editSlideTool() server.Tool {
	return server.Tool{
		ToolDef: mcpcore.ToolDef{
			Name:        "edit_slide",
			Description: "Replace the HTML content of a slide. Supply either 'slide' (preferred: slug, filename, or position as string) or 'position' (legacy: 1-based integer). Pass expected_version (from read_slide or describe_deck) for optimistic concurrency; omit or pass 'latest' for last-write-wins.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"deck":             propString("Deck name"),
					"slide":            propString("Slide reference: slug (e.g. 'metrics'), filename (e.g. '02-metrics.html'), or position number as string"),
					"position":         propInt("Slide position (1-based). Legacy — prefer 'slide' for stability across inserts."),
					"content":          propString("New HTML content for the slide"),
					"expected_version": propString("Expected slide version from read_slide. Omit or pass 'latest' to skip the check."),
				},
				"required": []string{"deck", "content"},
			},
		},
		Handler: func(ctx mcpcore.ToolContext, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			var p struct {
				Deck            string `json:"deck"`
				Slide           string `json:"slide"`
				Position        int    `json:"position"`
				Content         string `json:"content"`
				ExpectedVersion string `json:"expected_version"`
			}
			if err := req.Bind(&p); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
			pos, err := resolveSlidePosition(d, p.Slide, p.Position)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			// Optimistic version check
			if p.ExpectedVersion != "" && p.ExpectedVersion != "latest" {
				currentVer, err := d.SlideVersion(pos)
				if err != nil {
					return mcpcore.ErrorResult(err.Error()), nil
				}
				if currentVer != p.ExpectedVersion {
					currentContent, _ := d.GetSlideContent(pos)
					deckVer, _ := d.DeckVersion()
					conflict, _ := json.Marshal(versionConflict{
						Error:          "version_conflict",
						CurrentVersion: currentVer,
						CurrentContent: currentContent,
						DeckVersion:    deckVer,
					})
					return mcpcore.ErrorResult(string(conflict)), nil
				}
			}
			if err := d.EditSlideContent(pos, p.Content); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			newVer, _ := d.SlideVersion(pos)
			deckVer, _ := d.DeckVersion()
			mcpcore.NotifyResourceUpdated(ctx, fmt.Sprintf("ui://slyds/decks/%s/preview", p.Deck))
			mcpcore.NotifyResourceUpdated(ctx, fmt.Sprintf("ui://slyds/decks/%s/slides/%d/preview", p.Deck, pos))
			return jsonResult(slideEditResult{
				Message:     fmt.Sprintf("Slide %d updated.", pos),
				Version:     newVer,
				DeckVersion: deckVer,
			})
		},
	}
}

// resolveSlidePosition turns the (slide, position) parameter pair from
// read_slide / edit_slide into a concrete 1-based position. The `slide`
// string takes precedence over `position` if both are provided; this
// lets agents migrate from position-based refs to slug-based refs on
// their own schedule. Returns a descriptive error if neither is set or
// the slide string cannot be resolved.
func resolveSlidePosition(d *core.Deck, slide string, position int) (int, error) {
	if slide != "" {
		filename, err := d.ResolveSlide(slide)
		if err != nil {
			return 0, err
		}
		slides, err := d.SlideFilenames()
		if err != nil {
			return 0, err
		}
		for i, s := range slides {
			if s == filename {
				return i + 1, nil
			}
		}
		return 0, fmt.Errorf("resolved slide %q (%s) not found in deck ordering", slide, filename)
	}
	if position >= 1 {
		return position, nil
	}
	return 0, fmt.Errorf("either 'slide' or 'position' is required")
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
		Handler: func(ctx mcpcore.ToolContext, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
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
			return jsonResult(map[string]any{"results": results})
		},
	}
}

func addSlideTool() server.Tool {
	return server.Tool{
		ToolDef: mcpcore.ToolDef{
			Name:        "add_slide",
			Description: "Insert a new slide at the given position using a layout template. Pass expected_deck_version (from describe_deck or read_slide) for optimistic concurrency; omit or pass 'latest' for last-write-wins.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"deck":                  propString("Deck name"),
					"position":              propInt("Position to insert at (1-based)"),
					"name":                  propString("Slide filename (without .html extension or number prefix)"),
					"layout":                propString("Layout template: title, content, two-col, section, blank, closing"),
					"title":                 propString("Slide title (used in template rendering)"),
					"expected_deck_version": propString("Expected deck version from describe_deck or read_slide. Omit or pass 'latest' to skip."),
				},
				"required": []string{"deck", "position", "name"},
			},
		},
		Handler: func(ctx mcpcore.ToolContext, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			var p struct {
				Deck                string `json:"deck"`
				Position            int    `json:"position"`
				Name                string `json:"name"`
				Layout              string `json:"layout"`
				Title               string `json:"title"`
				ExpectedDeckVersion string `json:"expected_deck_version"`
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
			// Optimistic deck version check
			if p.ExpectedDeckVersion != "" && p.ExpectedDeckVersion != "latest" {
				currentDeckVer, err := d.DeckVersion()
				if err != nil {
					return mcpcore.ErrorResult(err.Error()), nil
				}
				if currentDeckVer != p.ExpectedDeckVersion {
					conflict, _ := json.Marshal(versionConflict{
						Error:          "version_conflict",
						CurrentVersion: currentDeckVer,
						DeckVersion:    currentDeckVer,
					})
					return mcpcore.ErrorResult(string(conflict)), nil
				}
			}
			finalSlug, slideID, err := d.InsertSlide(p.Position, p.Name, p.Layout, p.Title)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			deckVer, _ := d.DeckVersion()
			mcpcore.NotifyResourcesChanged(ctx)
			mcpcore.NotifyResourceUpdated(ctx, "ui://slyds/decks/"+p.Deck+"/preview")
			if finalSlug != p.Name {
				return mcpcore.TextResult(fmt.Sprintf(
					"Slide %q inserted at position %d (slug auto-suffixed to %q to avoid collision, slide_id: %q, deck_version: %q).",
					p.Name, p.Position, finalSlug, slideID, deckVer,
				)), nil
			}
			return mcpcore.TextResult(fmt.Sprintf(
				"Slide %q inserted at position %d (slide_id: %q, deck_version: %q).",
				p.Name, p.Position, slideID, deckVer,
			)), nil
		},
	}
}

func removeSlideTool() server.Tool {
	return server.Tool{
		ToolDef: mcpcore.ToolDef{
			Name:        "remove_slide",
			Description: "Remove a slide by filename or position number. Remaining slides are renumbered. Pass expected_deck_version for optimistic concurrency; omit or pass 'latest' for last-write-wins.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"deck":                  propString("Deck name"),
					"slide":                 propString("Slide reference: slug (e.g. 'metrics'), filename (e.g. '02-metrics.html'), or position number as string"),
					"expected_deck_version": propString("Expected deck version. Omit or pass 'latest' to skip."),
				},
				"required": []string{"deck", "slide"},
			},
		},
		Handler: func(ctx mcpcore.ToolContext, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			var p struct {
				Deck                string `json:"deck"`
				Slide               string `json:"slide"`
				ExpectedDeckVersion string `json:"expected_deck_version"`
			}
			if err := req.Bind(&p); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
			// Optimistic deck version check
			if p.ExpectedDeckVersion != "" && p.ExpectedDeckVersion != "latest" {
				currentDeckVer, err := d.DeckVersion()
				if err != nil {
					return mcpcore.ErrorResult(err.Error()), nil
				}
				if currentDeckVer != p.ExpectedDeckVersion {
					conflict, _ := json.Marshal(versionConflict{
						Error:          "version_conflict",
						CurrentVersion: currentDeckVer,
						DeckVersion:    currentDeckVer,
					})
					return mcpcore.ErrorResult(string(conflict)), nil
				}
			}
			filename, err := d.ResolveSlide(p.Slide)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			if err := d.RemoveSlide(filename); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			deckVer, _ := d.DeckVersion()
			mcpcore.NotifyResourcesChanged(ctx)
			mcpcore.NotifyResourceUpdated(ctx, "ui://slyds/decks/"+p.Deck+"/preview")
			return mcpcore.TextResult(fmt.Sprintf("Slide %q removed (deck_version: %q).", filename, deckVer)), nil
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
		Handler: func(ctx mcpcore.ToolContext, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			p, err := bindDeckParam(req)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
			mcpcore.EmitContent(ctx, req.RequestID, mcpcore.Content{
				Type: "text", Text: fmt.Sprintf("Validating deck %q...", p.Deck),
			})
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
		Handler: func(ctx mcpcore.ToolContext, req mcpcore.ToolRequest) (mcpcore.ToolResult, error) {
			p, err := bindDeckParam(req)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
			mcpcore.EmitContent(ctx, req.RequestID, mcpcore.Content{
				Type: "text", Text: fmt.Sprintf("Building deck %q...", p.Deck),
			})
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
