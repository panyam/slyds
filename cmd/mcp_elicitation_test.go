package cmd

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/mcpkit/client"
	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/slyds/core"
)

// --- remove_slide elicitation tests ---

// TestE2E_RemoveSlide_Confirmed verifies that when the client accepts the
// elicitation, the slide is removed.
func TestE2E_RemoveSlide_Confirmed(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Elicit Deck", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClient(t, root,
		client.WithElicitationHandler(func(_ context.Context, req mcpcore.ElicitationRequest) (mcpcore.ElicitationResult, error) {
			if !strings.Contains(req.Message, "Remove slide") {
				t.Errorf("unexpected elicitation message: %s", req.Message)
			}
			return mcpcore.ElicitationResult{
				Action:  "accept",
				Content: map[string]any{"confirm": true},
			}, nil
		}),
	)

	c.ToolCall("remove_slide", map[string]any{
		"deck":  "deck",
		"slide": "2",
	})

	// Verify slide count decreased.
	result := c.ToolCall("describe_deck", map[string]any{"deck": "deck"})
	var desc testDeckDescription
	if err := json.Unmarshal([]byte(result), &desc); err != nil {
		t.Fatalf("unmarshal describe: %v", err)
	}
	if desc.SlideCount != 2 {
		t.Errorf("expected 2 slides after removal, got %d", desc.SlideCount)
	}
}

// TestE2E_RemoveSlide_Declined verifies that when the client declines the
// elicitation, the slide is NOT removed.
func TestE2E_RemoveSlide_Declined(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Keep Deck", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClient(t, root,
		client.WithElicitationHandler(func(_ context.Context, _ mcpcore.ElicitationRequest) (mcpcore.ElicitationResult, error) {
			return mcpcore.ElicitationResult{Action: "decline"}, nil
		}),
	)

	result := c.ToolCall("remove_slide", map[string]any{
		"deck":  "deck",
		"slide": "2",
	})

	if !strings.Contains(result, "cancelled") {
		t.Errorf("expected cancellation message, got: %s", result)
	}

	// Verify slide count is unchanged.
	descResult := c.ToolCall("describe_deck", map[string]any{"deck": "deck"})
	var desc testDeckDescription
	if err := json.Unmarshal([]byte(descResult), &desc); err != nil {
		t.Fatalf("unmarshal describe: %v", err)
	}
	if desc.SlideCount != 3 {
		t.Errorf("expected 3 slides (unchanged), got %d", desc.SlideCount)
	}
}

// TestE2E_RemoveSlide_NoElicitation verifies backward compatibility: when the
// client doesn't support elicitation, the slide is still removed.
func TestE2E_RemoveSlide_NoElicitation(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Compat Deck", 3, "default", filepath.Join(root, "deck"), true)

	// No elicitation handler — client doesn't support it.
	c := newSlydsMCPClient(t, root)

	c.ToolCall("remove_slide", map[string]any{
		"deck":  "deck",
		"slide": "2",
	})

	// Slide should still be removed (backward compatible).
	descResult := c.ToolCall("describe_deck", map[string]any{"deck": "deck"})
	var desc testDeckDescription
	if err := json.Unmarshal([]byte(descResult), &desc); err != nil {
		t.Fatalf("unmarshal describe: %v", err)
	}
	if desc.SlideCount != 2 {
		t.Errorf("expected 2 slides after removal, got %d", desc.SlideCount)
	}
}

// --- create_deck elicitation tests ---

// TestE2E_CreateDeck_ElicitTheme verifies that when no theme is provided and
// the client supports elicitation, the theme is chosen via elicitation.
func TestE2E_CreateDeck_ElicitTheme(t *testing.T) {
	root := t.TempDir()

	c := newSlydsMCPClient(t, root,
		client.WithElicitationHandler(func(_ context.Context, req mcpcore.ElicitationRequest) (mcpcore.ElicitationResult, error) {
			if !strings.Contains(req.Message, "Choose a theme") {
				t.Errorf("unexpected elicitation message: %s", req.Message)
			}
			return mcpcore.ElicitationResult{
				Action:  "accept",
				Content: map[string]any{"theme": "dark"},
			}, nil
		}),
	)

	c.ToolCall("create_deck", map[string]any{
		"name":  "elicit-deck",
		"title": "Elicited Theme",
		// theme intentionally omitted
	})

	// Verify deck was created with the elicited theme.
	descResult := c.ToolCall("describe_deck", map[string]any{"deck": "elicit-deck"})
	var desc testDeckDescription
	if err := json.Unmarshal([]byte(descResult), &desc); err != nil {
		t.Fatalf("unmarshal describe: %v", err)
	}
	if desc.Theme != "dark" {
		t.Errorf("expected theme 'dark', got %q", desc.Theme)
	}
}

// TestE2E_CreateDeck_ElicitFallback verifies that when the client doesn't
// support elicitation, the deck falls back to the "default" theme.
func TestE2E_CreateDeck_ElicitFallback(t *testing.T) {
	root := t.TempDir()

	// No elicitation handler.
	c := newSlydsMCPClient(t, root)

	c.ToolCall("create_deck", map[string]any{
		"name":  "fallback-deck",
		"title": "Fallback Theme",
		// theme intentionally omitted
	})

	descResult := c.ToolCall("describe_deck", map[string]any{"deck": "fallback-deck"})
	var desc testDeckDescription
	if err := json.Unmarshal([]byte(descResult), &desc); err != nil {
		t.Fatalf("unmarshal describe: %v", err)
	}
	if desc.Theme != "default" {
		t.Errorf("expected theme 'default', got %q", desc.Theme)
	}
}
