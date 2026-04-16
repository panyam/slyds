package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/slyds/core"
)

// registerTools registers all semantic MCP tools on the server using
// typed tool registration (mcpkit v0.2.26). InputSchema is auto-derived
// from Go struct tags — no manual schema maps needed.
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
		improveSlideTool(),
		checkDeckTool(),
		buildDeckTool(),
	)
}

// --- Input structs (drive InputSchema via jsonschema tags) ---

type listDecksInput struct{}

type createDeckInput struct {
	Name   string `json:"name"   jsonschema:"required,description=Deck name (becomes the deck identifier in the workspace)"`
	Title  string `json:"title"  jsonschema:"required,description=Presentation title"`
	Theme  string `json:"theme,omitempty"  jsonschema:"description=Theme (optional — omit to let the user choose interactively)"`
	Slides int    `json:"slides,omitempty" jsonschema:"description=Number of slides to scaffold (default: 3)"`
}

type deckInput struct {
	Deck string `json:"deck" jsonschema:"required,description=Deck name (workspace-scoped identifier or '.' for root deck)"`
}

type readSlideInput struct {
	Deck     string `json:"deck"               jsonschema:"required,description=Deck name"`
	Slide    string `json:"slide,omitempty"     jsonschema:"description=Slide reference: slug (e.g. 'metrics')\\, filename\\, or position as string"`
	Position int    `json:"position,omitempty"  jsonschema:"description=Slide position (1-based). Legacy — prefer 'slide'"`
}

type editSlideInput struct {
	Deck            string `json:"deck"                       jsonschema:"required,description=Deck name"`
	Slide           string `json:"slide,omitempty"            jsonschema:"description=Slide reference: slug\\, filename\\, or position as string"`
	Position        int    `json:"position,omitempty"         jsonschema:"description=Slide position (1-based). Legacy — prefer 'slide'"`
	Content         string `json:"content"                    jsonschema:"required,description=New HTML content for the slide"`
	ExpectedVersion string `json:"expected_version,omitempty" jsonschema:"description=Expected slide version from read_slide. Omit or pass 'latest' to skip."`
}

type querySlideInput struct {
	Deck     string  `json:"deck"               jsonschema:"required,description=Deck name"`
	Slide    string  `json:"slide"              jsonschema:"required,description=Slide reference: position number or filename substring"`
	Selector string  `json:"selector"           jsonschema:"required,description=CSS selector (e.g. 'h1'\\, '.slide-body'\\, 'img')"`
	HTML     bool    `json:"html,omitempty"      jsonschema:"description=Return inner HTML instead of text"`
	Attr     string  `json:"attr,omitempty"      jsonschema:"description=Return the value of this attribute"`
	Count    bool    `json:"count,omitempty"     jsonschema:"description=Return match count instead of content"`
	Set      *string `json:"set,omitempty"       jsonschema:"description=Set inner text of matched elements"`
	SetHTML  *string `json:"set_html,omitempty"  jsonschema:"description=Set inner HTML of matched elements"`
	SetAttr  *string `json:"set_attr,omitempty"  jsonschema:"description=Set attribute (NAME=VALUE format)"`
	Append   *string `json:"append,omitempty"    jsonschema:"description=Append child HTML to matched elements"`
	Remove   bool    `json:"remove,omitempty"    jsonschema:"description=Remove matched elements"`
	All      bool    `json:"all,omitempty"       jsonschema:"description=Apply to all matches (default: first only)"`
}

type addSlideInput struct {
	Deck                string `json:"deck"                             jsonschema:"required,description=Deck name"`
	Position            int    `json:"position"                         jsonschema:"required,description=Position to insert at (1-based)"`
	Name                string `json:"name"                             jsonschema:"required,description=Slide filename (without .html extension or number prefix)"`
	Layout              string `json:"layout,omitempty"                 jsonschema:"description=Layout template: title\\, content\\, two-col\\, section\\, blank\\, closing"`
	Title               string `json:"title,omitempty"                  jsonschema:"description=Slide title (used in template rendering)"`
	ExpectedDeckVersion string `json:"expected_deck_version,omitempty"  jsonschema:"description=Expected deck version. Omit or pass 'latest' to skip."`
}

type removeSlideInput struct {
	Deck                string `json:"deck"                             jsonschema:"required,description=Deck name"`
	Slide               string `json:"slide"                            jsonschema:"required,description=Slide reference: slug\\, filename\\, or position number as string"`
	ExpectedDeckVersion string `json:"expected_deck_version,omitempty"  jsonschema:"description=Expected deck version. Omit or pass 'latest' to skip."`
}

type improveSlideInput struct {
	Deck        string `json:"deck"        jsonschema:"required,description=Deck name"`
	Slide       string `json:"slide"       jsonschema:"required,description=Slide reference: position number\\, slug\\, or filename"`
	Instruction string `json:"instruction" jsonschema:"required,description=What to improve (e.g. 'make the bullet points more concise')"`
}

// --- Output structs ---

type deckSummary struct {
	Name   string `json:"name"`
	Title  string `json:"title"`
	Theme  string `json:"theme"`
	Slides int    `json:"slides"`
}

type buildWarningResult struct {
	HTML     string   `json:"html"`
	Warnings []string `json:"warnings"`
}

type slideReadResult struct {
	Content     string `json:"content"`
	Version     string `json:"version"`
	DeckVersion string `json:"deck_version"`
}

type slideEditResult struct {
	Message     string `json:"message"`
	Version     string `json:"version"`
	DeckVersion string `json:"deck_version"`
}

type versionConflict struct {
	Error          string `json:"error"`
	CurrentVersion string `json:"current_version"`
	CurrentContent string `json:"current_content,omitempty"`
	DeckVersion    string `json:"deck_version,omitempty"`
}

// --- Tool definitions ---

func listDecksTool() mcpcore.TypedToolResult {
	return mcpcore.TypedTool[listDecksInput, mcpcore.ToolResult](
		"list_decks",
		"List all presentation decks visible to the current workspace with name, title, theme, and slide count.",
		func(ctx mcpcore.ToolContext, _ listDecksInput) (mcpcore.ToolResult, error) {
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
	)
}

func createDeckTool() mcpcore.TypedToolResult {
	return mcpcore.TypedTool[createDeckInput, mcpcore.ToolResult](
		"create_deck",
		"Create a new presentation deck with the given name, title, and slide count. Omit theme to let the user choose interactively via the host UI.",
		func(ctx mcpcore.ToolContext, p createDeckInput) (mcpcore.ToolResult, error) {
			ws, errResult := requireWorkspace(ctx)
			if errResult != nil {
				return *errResult, nil
			}
			if p.Theme == "" {
				themes := core.AvailableThemeNames()
				themesJSON, _ := json.Marshal(themes)
				schema := fmt.Sprintf(`{
					"type": "object",
					"properties": {"theme": {"type": "string", "enum": %s, "description": "Presentation theme"}},
					"required": ["theme"]
				}`, string(themesJSON))
				elicitResult, elicitErr := ctx.Elicit(mcpcore.ElicitationRequest{
					Message:         fmt.Sprintf("Choose a theme for %q:", p.Title),
					RequestedSchema: json.RawMessage(schema),
				})
				if elicitErr == nil && elicitResult.Action == "accept" {
					if theme, ok := elicitResult.Content["theme"].(string); ok && theme != "" {
						p.Theme = theme
					}
				}
				if p.Theme == "" {
					p.Theme = "default"
				}
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
	)
}

func describeDeckTool() mcpcore.TypedToolResult {
	return mcpcore.TypedTool[deckInput, mcpcore.ToolResult](
		"describe_deck",
		"Get structured metadata for a deck: title, theme, slide list with layouts, word counts, and notes status.",
		func(ctx mcpcore.ToolContext, p deckInput) (mcpcore.ToolResult, error) {
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
	)
}

func listSlidesTool() mcpcore.TypedToolResult {
	return mcpcore.TypedTool[deckInput, mcpcore.ToolResult](
		"list_slides",
		"List all slides in a deck with filenames, layouts, titles, and word counts.",
		func(ctx mcpcore.ToolContext, p deckInput) (mcpcore.ToolResult, error) {
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
	)
}

func readSlideTool() mcpcore.TypedToolResult {
	return mcpcore.TypedTool[readSlideInput, mcpcore.ToolResult](
		"read_slide",
		"Read the raw HTML content of a slide. Supply either 'slide' (preferred: slug, filename, or position as string) or 'position' (legacy: 1-based integer).",
		func(ctx mcpcore.ToolContext, p readSlideInput) (mcpcore.ToolResult, error) {
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
	)
}

func editSlideTool() mcpcore.TypedToolResult {
	return mcpcore.TypedTool[editSlideInput, mcpcore.ToolResult](
		"edit_slide",
		"Replace the HTML content of a slide. Content MUST be a raw HTML fragment (not JSON-escaped) whose root element is <div class=\"slide\" data-layout=\"...\">. Do NOT use class=\"slide-content\" or other variants — the engine requires exactly class=\"slide\" for pagination. Do NOT escape quotes with backslashes. Do NOT include <style> blocks — they pollute global CSS and break navigation; use inline style= attributes instead. Supply either 'slide' (preferred: slug, filename, or position as string) or 'position' (legacy: 1-based integer). Pass expected_version (from read_slide or describe_deck) for optimistic concurrency; omit or pass 'latest' for last-write-wins.",
		func(ctx mcpcore.ToolContext, p editSlideInput) (mcpcore.ToolResult, error) {
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
			pos, err := resolveSlidePosition(d, p.Slide, p.Position)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
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
			if issues := core.LintSlideContent(p.Content); issues.HasErrors() {
				return mcpcore.ErrorResult("rejected: " + issues[0].Detail), nil
			}
			content, sanitizeWarnings := core.SanitizeSlideContent(p.Content)
			if err := d.EditSlideContent(pos, content); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			newVer, _ := d.SlideVersion(pos)
			deckVer, _ := d.DeckVersion()
			mcpcore.NotifyResourceUpdated(ctx, fmt.Sprintf("ui://slyds/decks/%s/preview", p.Deck))
			mcpcore.NotifyResourceUpdated(ctx, fmt.Sprintf("ui://slyds/decks/%s/slides/%d/preview", p.Deck, pos))
			msg := fmt.Sprintf("Slide %d updated.", pos)
			for _, w := range sanitizeWarnings {
				msg += " WARNING: " + w.Detail
			}
			return jsonResult(slideEditResult{
				Message:     msg,
				Version:     newVer,
				DeckVersion: deckVer,
			})
		},
	)
}

// resolveSlidePosition turns the (slide, position) parameter pair from
// read_slide / edit_slide into a concrete 1-based position.
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

func querySlideTool() mcpcore.TypedToolResult {
	return mcpcore.TypedTool[querySlideInput, mcpcore.ToolResult](
		"query_slide",
		"Query or modify slide HTML using CSS selectors (goquery). Read text, attributes, inner HTML, or mutate content.",
		func(ctx mcpcore.ToolContext, p querySlideInput) (mcpcore.ToolResult, error) {
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
	)
}

func addSlideTool() mcpcore.TypedToolResult {
	return mcpcore.TypedTool[addSlideInput, mcpcore.ToolResult](
		"add_slide",
		"Insert a new slide at the given position using a layout template. Pass expected_deck_version (from describe_deck or read_slide) for optimistic concurrency; omit or pass 'latest' for last-write-wins.",
		func(ctx mcpcore.ToolContext, p addSlideInput) (mcpcore.ToolResult, error) {
			if p.Layout == "" {
				p.Layout = "content"
			}
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
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
	)
}

func removeSlideTool() mcpcore.TypedToolResult {
	return mcpcore.TypedTool[removeSlideInput, mcpcore.ToolResult](
		"remove_slide",
		"Remove a slide by filename or position number. Remaining slides are renumbered. Pass expected_deck_version for optimistic concurrency; omit or pass 'latest' for last-write-wins.",
		func(ctx mcpcore.ToolContext, p removeSlideInput) (mcpcore.ToolResult, error) {
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
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
			elicitResult, elicitErr := ctx.Elicit(mcpcore.ElicitationRequest{
				Message: fmt.Sprintf("Remove slide %q from deck %q? This cannot be undone.", filename, p.Deck),
				RequestedSchema: json.RawMessage(`{
					"type": "object",
					"properties": {"confirm": {"type": "boolean", "description": "Confirm slide removal"}},
					"required": ["confirm"]
				}`),
			})
			if elicitErr == nil {
				if elicitResult.Action == "decline" || elicitResult.Action == "cancel" {
					return mcpcore.TextResult("Slide removal cancelled."), nil
				}
				if confirm, ok := elicitResult.Content["confirm"].(bool); ok && !confirm {
					return mcpcore.TextResult("Slide removal declined."), nil
				}
			}
			if err := d.RemoveSlide(filename); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			deckVer, _ := d.DeckVersion()
			mcpcore.NotifyResourcesChanged(ctx)
			mcpcore.NotifyResourceUpdated(ctx, "ui://slyds/decks/"+p.Deck+"/preview")
			return mcpcore.TextResult(fmt.Sprintf("Slide %q removed (deck_version: %q).", filename, deckVer)), nil
		},
	)
}

func improveSlideTool() mcpcore.TypedToolResult {
	return mcpcore.TypedTool[improveSlideInput, mcpcore.ToolResult](
		"improve_slide",
		"Improve a slide's content using AI. Reads the current slide, sends it to the client's LLM with your instruction, and applies the result. Requires the client to support sampling.",
		func(ctx mcpcore.ToolContext, p improveSlideInput) (mcpcore.ToolResult, error) {
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
			pos, err := resolveSlidePosition(d, p.Slide, 0)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			content, err := d.GetSlideContent(pos)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			sampleResult, err := ctx.Sample(mcpcore.CreateMessageRequest{
				SystemPrompt: "You are an HTML presentation slide editor for slyds. " +
					"You edit raw HTML fragments. The root element must be <div class=\"slide\" data-layout=\"...\">. " +
					"Do NOT use <style> blocks — use inline style= attributes. " +
					"Do NOT escape quotes with backslashes. " +
					"Return ONLY the HTML fragment, no explanation.",
				Messages: []mcpcore.SamplingMessage{
					{Role: "user", Content: mcpcore.Content{
						Type: "text",
						Text: fmt.Sprintf("Current slide HTML:\n\n%s\n\nInstruction: %s", content, p.Instruction),
					}},
				},
				MaxTokens: 4000,
			})
			if errors.Is(err, mcpcore.ErrSamplingNotSupported) {
				return mcpcore.ErrorResult("sampling not supported by this client — use edit_slide directly with your own content"), nil
			}
			if err != nil {
				return mcpcore.ErrorResult(fmt.Sprintf("sampling failed: %v", err)), nil
			}
			newContent := sampleResult.Content.Text
			if issues := core.LintSlideContent(newContent); issues.HasErrors() {
				return mcpcore.ErrorResult(fmt.Sprintf(
					"LLM-generated HTML failed lint: %s\n\nRaw output:\n%s",
					issues[0].Detail, newContent,
				)), nil
			}
			newContent, _ = core.SanitizeSlideContent(newContent)
			if err := d.EditSlideContent(pos, newContent); err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			ver, _ := d.SlideVersion(pos)
			deckVer, _ := d.DeckVersion()
			mcpcore.NotifyResourceUpdated(ctx, "ui://slyds/decks/"+p.Deck+"/preview")
			return jsonResult(slideEditResult{
				Message:     fmt.Sprintf("Slide %d improved.", pos),
				Version:     ver,
				DeckVersion: deckVer,
			})
		},
	)
}

func checkDeckTool() mcpcore.TypedToolResult {
	return mcpcore.TypedTool[deckInput, mcpcore.ToolResult](
		"check_deck",
		"Validate a deck: check for missing files, broken includes, missing speaker notes, and other issues.",
		func(ctx mcpcore.ToolContext, p deckInput) (mcpcore.ToolResult, error) {
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
			ctx.EmitContent(nil, mcpcore.Content{
				Type: "text", Text: fmt.Sprintf("Validating deck %q...", p.Deck),
			})
			issues, err := d.Check()
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			return jsonResult(issues)
		},
		mcpcore.WithTypedToolTimeout(10*time.Second),
	)
}

func buildDeckTool() mcpcore.TypedToolResult {
	return mcpcore.TypedTool[deckInput, mcpcore.ToolResult](
		"build_deck",
		"Build a self-contained HTML file from the deck. Resolves all includes, inlines CSS/JS/images.",
		func(ctx mcpcore.ToolContext, p deckInput) (mcpcore.ToolResult, error) {
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
			ctx.EmitContent(nil, mcpcore.Content{
				Type: "text", Text: fmt.Sprintf("Building deck %q...", p.Deck),
			})
			result, err := d.Build()
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			return jsonResult(buildWarningResult{
				HTML:     result.HTML,
				Warnings: result.Warnings,
			})
		},
		mcpcore.WithTypedToolTimeout(30*time.Second),
	)
}

// --- Helpers ---

// propString/propInt are JSON Schema property helpers used by mcp_apps.go
// (app tools still use map[string]any schemas since RegisterAppTool doesn't
// support TypedAppTool for template handlers yet).
func propString(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func propInt(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}

// jsonResult marshals v to indented JSON and returns it as a StructuredResult.
func jsonResult(v any) (mcpcore.ToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	return mcpcore.StructuredResult(string(data), v), nil
}
