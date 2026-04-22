package cmd

import (
	"fmt"
	"strings"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/slyds/core"
)

// registerPrompts registers MCP prompt templates on the server. Prompts
// are server-defined prompt templates that clients can discover via
// prompts/list and invoke via prompts/get to get pre-built messages.
func registerPrompts(srv *server.Server) {
	srv.RegisterPrompt(
		mcpcore.PromptDef{
			Name:        "create-presentation",
			Description: "Generate guidance messages for creating a new presentation on a given topic.",
			Arguments: []mcpcore.PromptArgument{
				{Name: "topic", Description: "Presentation topic", Required: true},
				{Name: "slide_count", Description: "Number of slides (default: 5)"},
				{Name: "theme", Description: "Theme name (default: default)"},
			},
		},
		handleCreatePresentation,
	)

	srv.RegisterPrompt(
		mcpcore.PromptDef{
			Name:        "review-slides",
			Description: "Review a presentation deck for clarity, flow, and consistency.",
			Arguments: []mcpcore.PromptArgument{
				{Name: "name", Description: "Deck name", Required: true},
			},
		},
		handleReviewSlides,
	)

	srv.RegisterPrompt(
		mcpcore.PromptDef{
			Name:        "suggest-speaker-notes",
			Description: "Draft speaker notes for a specific slide in a deck.",
			Arguments: []mcpcore.PromptArgument{
				{Name: "name", Description: "Deck name", Required: true},
				{Name: "slide", Description: "Slide reference: position number, slug, or filename", Required: true},
			},
		},
		handleSuggestSpeakerNotes,
	)
}

// handleCreatePresentation returns guidance messages for creating a new deck.
// Does not require workspace access — produces generic guidance with available
// themes and layouts.
func handleCreatePresentation(ctx mcpcore.PromptContext, req mcpcore.PromptRequest) (mcpcore.PromptResult, error) {
	topic, _ := req.Arguments["topic"].(string)
	if topic == "" {
		return mcpcore.PromptResult{}, fmt.Errorf("topic is required")
	}

	slideCount := "5"
	if sc, ok := req.Arguments["slide_count"].(string); ok && sc != "" {
		slideCount = sc
	}
	theme := "default"
	if th, ok := req.Arguments["theme"].(string); ok && th != "" {
		theme = th
	}

	// Use workspace themes (built-in + external) if available, fall back to built-in only.
	var themes []string
	if ws := workspaceFromContext(ctx); ws != nil {
		themes = ws.AvailableThemes()
	} else {
		themes = core.AvailableThemeNames()
	}
	layouts, _ := core.ListLayouts()

	text := fmt.Sprintf(
		"Create a slyds presentation about %q with %s slides using the %q theme.\n\n"+
			"Available themes: %s\n"+
			"Available layouts: %s\n\n"+
			"Steps:\n"+
			"1. Use create_deck to scaffold the deck\n"+
			"2. Use edit_slide on each slide to add content\n"+
			"3. Use check_deck to validate\n"+
			"4. Use build_deck to produce the final HTML",
		topic, slideCount, theme,
		strings.Join(themes, ", "),
		strings.Join(layouts, ", "),
	)

	return mcpcore.PromptResult{
		Description: fmt.Sprintf("Create a presentation about %q", topic),
		Messages: []mcpcore.PromptMessage{
			{Role: "user", Content: mcpcore.Content{Type: "text", Text: text}},
		},
	}, nil
}

// handleReviewSlides reads all slides from a deck and returns messages asking
// the agent to review for clarity, flow, and consistency.
func handleReviewSlides(ctx mcpcore.PromptContext, req mcpcore.PromptRequest) (mcpcore.PromptResult, error) {
	name, _ := req.Arguments["name"].(string)
	if name == "" {
		return mcpcore.PromptResult{}, fmt.Errorf("name is required")
	}

	ws := workspaceFromContext(ctx)
	if ws == nil {
		return mcpcore.PromptResult{}, fmt.Errorf("no workspace available")
	}
	d, err := ws.OpenDeck(name)
	if err != nil {
		return mcpcore.PromptResult{}, fmt.Errorf("deck %q: %w", name, err)
	}

	desc, err := d.Describe()
	if err != nil {
		return mcpcore.PromptResult{}, fmt.Errorf("describe deck: %w", err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Review the presentation %q (%d slides, theme: %s) for clarity, flow, and consistency.\n\n",
		desc.Title, desc.SlideCount, desc.Theme)

	for i := 1; i <= desc.SlideCount; i++ {
		content, err := d.GetSlideContent(i)
		if err != nil {
			continue
		}
		fmt.Fprintf(&sb, "--- Slide %d ---\n%s\n\n", i, content)
	}

	sb.WriteString("Provide specific feedback on each slide and overall flow suggestions.")

	return mcpcore.PromptResult{
		Description: fmt.Sprintf("Review %q (%d slides)", desc.Title, desc.SlideCount),
		Messages: []mcpcore.PromptMessage{
			{Role: "user", Content: mcpcore.Content{Type: "text", Text: sb.String()}},
		},
	}, nil
}

// handleSuggestSpeakerNotes reads a specific slide and returns messages asking
// the agent to draft speaker notes.
func handleSuggestSpeakerNotes(ctx mcpcore.PromptContext, req mcpcore.PromptRequest) (mcpcore.PromptResult, error) {
	name, _ := req.Arguments["name"].(string)
	if name == "" {
		return mcpcore.PromptResult{}, fmt.Errorf("name is required")
	}
	slide, _ := req.Arguments["slide"].(string)
	if slide == "" {
		return mcpcore.PromptResult{}, fmt.Errorf("slide is required")
	}

	ws := workspaceFromContext(ctx)
	if ws == nil {
		return mcpcore.PromptResult{}, fmt.Errorf("no workspace available")
	}
	d, err := ws.OpenDeck(name)
	if err != nil {
		return mcpcore.PromptResult{}, fmt.Errorf("deck %q: %w", name, err)
	}

	pos, err := resolveSlidePosition(d, slide, 0)
	if err != nil {
		return mcpcore.PromptResult{}, fmt.Errorf("resolve slide: %w", err)
	}
	content, err := d.GetSlideContent(pos)
	if err != nil {
		return mcpcore.PromptResult{}, fmt.Errorf("read slide %d: %w", pos, err)
	}

	desc, _ := d.Describe()
	title := name
	if desc != nil {
		title = desc.Title
	}

	text := fmt.Sprintf(
		"Draft speaker notes for slide %d of the presentation %q.\n\n"+
			"Slide content:\n%s\n\n"+
			"The notes should:\n"+
			"- Complement the visual content, not repeat it\n"+
			"- Provide talking points and transitions\n"+
			"- Include timing guidance (approximate minutes)",
		pos, title, content,
	)

	return mcpcore.PromptResult{
		Description: fmt.Sprintf("Speaker notes for slide %d of %q", pos, title),
		Messages: []mcpcore.PromptMessage{
			{Role: "user", Content: mcpcore.Content{Type: "text", Text: text}},
		},
	}, nil
}
