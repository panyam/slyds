package cmd

import (
	"fmt"
	"strings"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
)

// registerCompletions registers completion handlers for resource template
// URI parameters. Hosts send completion/complete with ref/resource to get
// autocomplete suggestions for template variables like {name} and {n}.
//
// Two completion types:
//   - Deck name: queries the workspace for available deck names
//   - Slide position: queries a specific deck for its slide count
//
// Templates with both deck and slide params (e.g. slyds://decks/{name}/slides/{n})
// use a combined handler that dispatches on arg.Name.
func registerCompletions(srv *server.Server) {
	// Templates with only a deck name parameter.
	for _, uri := range []string{
		"slyds://decks/{name}",
		"slyds://decks/{name}/slides",
		"slyds://decks/{name}/config",
		"slyds://decks/{name}/agent",
		"ui://slyds/decks/{deck}/preview",
	} {
		srv.RegisterCompletion("ref/resource", uri, completeDeckName)
	}

	// Templates with both deck name and slide position parameters.
	for _, uri := range []string{
		"slyds://decks/{name}/slides/{n}",
		"ui://slyds/decks/{deck}/slides/{position}/preview",
	} {
		srv.RegisterCompletion("ref/resource", uri, completeDeckOrSlide)
	}
}

// completeDeckName returns deck names matching the partial input.
func completeDeckName(ctx mcpcore.PromptContext, _ mcpcore.CompletionRef, arg mcpcore.CompletionArgument) (mcpcore.CompletionResult, error) {
	ws := workspaceFromContext(ctx)
	if ws == nil {
		return mcpcore.CompletionResult{}, nil
	}
	refs, err := ws.ListDecks()
	if err != nil {
		return mcpcore.CompletionResult{}, nil
	}
	prefix := strings.ToLower(arg.Value)
	var matches []string
	for _, ref := range refs {
		if prefix == "" || strings.HasPrefix(strings.ToLower(ref.Name), prefix) {
			matches = append(matches, ref.Name)
		}
	}
	return mcpcore.CompletionResult{
		Values:  matches,
		Total:   len(matches),
		HasMore: false,
	}, nil
}

// completeDeckOrSlide dispatches to deck name or slide position completion
// based on the argument name. Used for templates that have both parameters
// (e.g. slyds://decks/{name}/slides/{n}).
func completeDeckOrSlide(ctx mcpcore.PromptContext, ref mcpcore.CompletionRef, arg mcpcore.CompletionArgument) (mcpcore.CompletionResult, error) {
	switch arg.Name {
	case "n", "position":
		return completeSlidePosition(ctx, ref, arg)
	default:
		return completeDeckName(ctx, ref, arg)
	}
}

// completeSlidePosition returns slide position numbers matching the partial
// input. Since completion/complete doesn't carry other resolved params, we
// can't know which deck to query — we return a generic range hint. If the
// workspace has exactly one deck, we use its slide count.
func completeSlidePosition(ctx mcpcore.PromptContext, _ mcpcore.CompletionRef, arg mcpcore.CompletionArgument) (mcpcore.CompletionResult, error) {
	ws := workspaceFromContext(ctx)
	if ws == nil {
		return mcpcore.CompletionResult{}, nil
	}

	// Try to infer the deck. If there's only one, use it.
	// Otherwise we can't know which deck's slide count to use.
	refs, err := ws.ListDecks()
	if err != nil || len(refs) == 0 {
		return mcpcore.CompletionResult{}, nil
	}

	// Use the first (or only) deck as a reasonable default.
	d, err := ws.OpenDeck(refs[0].Name)
	if err != nil {
		return mcpcore.CompletionResult{}, nil
	}
	count, err := d.SlideCount()
	if err != nil || count == 0 {
		return mcpcore.CompletionResult{}, nil
	}

	prefix := arg.Value
	var matches []string
	for i := 1; i <= count; i++ {
		s := fmt.Sprintf("%d", i)
		if prefix == "" || strings.HasPrefix(s, prefix) {
			matches = append(matches, s)
		}
	}
	return mcpcore.CompletionResult{
		Values:  matches,
		Total:   len(matches),
		HasMore: false,
	}, nil
}
