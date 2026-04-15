package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/slyds/core"
)

// --- E2E prompt tests using TestClient ---

// TestE2E_PromptsList verifies that prompts/list returns all registered prompts.
func TestE2E_PromptsList(t *testing.T) {
	root := t.TempDir()
	c := newSlydsMCPClient(t, root)

	result := c.Call("prompts/list", nil)
	var parsed struct {
		Prompts []mcpcore.PromptDef `json:"prompts"`
	}
	if err := result.Unmarshal(&parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.Prompts) != 3 {
		t.Fatalf("expected 3 prompts, got %d", len(parsed.Prompts))
	}

	names := map[string]bool{}
	for _, p := range parsed.Prompts {
		names[p.Name] = true
	}
	for _, want := range []string{"create-presentation", "review-slides", "suggest-speaker-notes"} {
		if !names[want] {
			t.Errorf("missing prompt %q", want)
		}
	}
}

// TestE2E_PromptsGet_CreatePresentation verifies the create-presentation prompt
// returns messages mentioning the topic and available themes/layouts.
func TestE2E_PromptsGet_CreatePresentation(t *testing.T) {
	root := t.TempDir()
	c := newSlydsMCPClient(t, root)

	result := c.Call("prompts/get", map[string]any{
		"name":      "create-presentation",
		"arguments": map[string]any{"topic": "Kubernetes"},
	})
	var parsed mcpcore.PromptResult
	if err := result.Unmarshal(&parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.Messages) == 0 {
		t.Fatal("expected at least one message")
	}
	text := parsed.Messages[0].Content.Text
	if !strings.Contains(text, "Kubernetes") {
		t.Error("message should mention the topic")
	}
	if !strings.Contains(text, "default") {
		t.Error("message should list available themes")
	}
	if !strings.Contains(text, "create_deck") {
		t.Error("message should reference create_deck tool")
	}
}

// TestE2E_PromptsGet_CreatePresentation_CustomArgs verifies that optional
// arguments (slide_count, theme) are reflected in the prompt output.
func TestE2E_PromptsGet_CreatePresentation_CustomArgs(t *testing.T) {
	root := t.TempDir()
	c := newSlydsMCPClient(t, root)

	result := c.Call("prompts/get", map[string]any{
		"name": "create-presentation",
		"arguments": map[string]any{
			"topic":       "GraphQL",
			"slide_count": "10",
			"theme":       "dark",
		},
	})
	var parsed mcpcore.PromptResult
	if err := result.Unmarshal(&parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	text := parsed.Messages[0].Content.Text
	if !strings.Contains(text, "GraphQL") {
		t.Error("message should mention GraphQL")
	}
	if !strings.Contains(text, "10 slides") {
		t.Error("message should mention 10 slides")
	}
	if !strings.Contains(text, `"dark"`) {
		t.Error("message should mention dark theme")
	}
}

// TestE2E_PromptsGet_ReviewSlides verifies that the review-slides prompt
// reads all slide content from a deck and includes it in the messages.
func TestE2E_PromptsGet_ReviewSlides(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Test Deck", 3, "default", filepath.Join(root, "mydeck"), true)
	c := newSlydsMCPClient(t, root)

	result := c.Call("prompts/get", map[string]any{
		"name":      "review-slides",
		"arguments": map[string]any{"name": "mydeck"},
	})
	var parsed mcpcore.PromptResult
	if err := result.Unmarshal(&parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.Messages) == 0 {
		t.Fatal("expected at least one message")
	}
	text := parsed.Messages[0].Content.Text
	if !strings.Contains(text, "Test Deck") {
		t.Error("message should mention the deck title")
	}
	if !strings.Contains(text, "Slide 1") {
		t.Error("message should include slide 1 content")
	}
	if !strings.Contains(text, "Slide 3") {
		t.Error("message should include slide 3 content")
	}
	if !strings.Contains(text, "feedback") {
		t.Error("message should ask for feedback")
	}
}

// TestE2E_PromptsGet_ReviewSlides_DeckNotFound verifies that the review-slides
// prompt returns an error for a nonexistent deck.
func TestE2E_PromptsGet_ReviewSlides_DeckNotFound(t *testing.T) {
	root := t.TempDir()
	c := newSlydsMCPClient(t, root)

	// Use the raw client.Call to avoid t.Fatal on expected errors.
	result, err := c.Client.Call("prompts/get", map[string]any{
		"name":      "review-slides",
		"arguments": map[string]any{"name": "nonexistent"},
	})
	if err == nil && result != nil {
		var parsed mcpcore.PromptResult
		if result.Unmarshal(&parsed) == nil && len(parsed.Messages) > 0 {
			t.Error("expected error or empty result for nonexistent deck")
		}
	}
	// Getting an error is the expected/correct outcome.
}

// TestE2E_PromptsGet_SuggestSpeakerNotes verifies the suggest-speaker-notes
// prompt reads a specific slide and includes its content.
func TestE2E_PromptsGet_SuggestSpeakerNotes(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Notes Deck", 3, "default", filepath.Join(root, "notes"), true)
	c := newSlydsMCPClient(t, root)

	result := c.Call("prompts/get", map[string]any{
		"name":      "suggest-speaker-notes",
		"arguments": map[string]any{"name": "notes", "slide": "2"},
	})
	var parsed mcpcore.PromptResult
	if err := result.Unmarshal(&parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.Messages) == 0 {
		t.Fatal("expected at least one message")
	}
	text := parsed.Messages[0].Content.Text
	if !strings.Contains(text, "slide 2") {
		t.Error("message should reference slide 2")
	}
	if !strings.Contains(text, "speaker notes") || !strings.Contains(text, "talking points") {
		t.Error("message should include speaker notes guidance")
	}
}
