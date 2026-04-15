package cmd

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/mcpkit/client"
	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/slyds/core"
)

// TestE2E_ImproveSlide_Success verifies that improve_slide calls the client's
// sampling handler, applies the returned HTML, and reports the new version.
func TestE2E_ImproveSlide_Success(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Sampling Deck", 2, "default", filepath.Join(root, "deck"), true)

	improvedHTML := `<div class="slide" data-layout="content"><h1>Improved</h1><p>Better content</p></div>`

	c := newSlydsMCPClient(t, root,
		client.WithSamplingHandler(func(_ context.Context, req mcpcore.CreateMessageRequest) (mcpcore.CreateMessageResult, error) {
			// Verify the request includes the current slide content.
			if len(req.Messages) == 0 {
				t.Error("sampling request should have messages")
			}
			return mcpcore.CreateMessageResult{
				Model: "test-model",
				Role:  "assistant",
				Content: mcpcore.Content{
					Type: "text",
					Text: improvedHTML,
				},
			}, nil
		}),
	)

	result := c.ToolCall("improve_slide", map[string]any{
		"deck":        "deck",
		"slide":       "1",
		"instruction": "make it better",
	})

	if !strings.Contains(result, "improved") && !strings.Contains(result, "Improved") {
		// The result is JSON with a message field; just check it's not an error.
		if strings.Contains(result, "error") {
			t.Fatalf("unexpected error: %s", result)
		}
	}

	// Verify the slide was actually updated.
	content := readSlideContent(t, c, map[string]any{"deck": "deck", "slide": "1"})
	if !strings.Contains(content, "Better content") {
		t.Errorf("slide should contain improved content, got: %s", content)
	}
}

// TestE2E_ImproveSlide_NotSupported verifies that improve_slide returns a
// helpful error when the client doesn't support sampling.
func TestE2E_ImproveSlide_NotSupported(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("No Sampling", 2, "default", filepath.Join(root, "deck"), true)

	// No WithSamplingHandler — client doesn't support sampling.
	c := newSlydsMCPClient(t, root)

	text, err := c.Client.ToolCall("improve_slide", map[string]any{
		"deck":        "deck",
		"slide":       "1",
		"instruction": "make it better",
	})

	// Tool should return an error (either via err or via error text).
	msg := text
	if err != nil {
		msg = err.Error()
	}
	if !strings.Contains(msg, "sampling not supported") {
		t.Errorf("expected 'sampling not supported' error, got: %s", msg)
	}
}

// TestE2E_ImproveSlide_InvalidHTML verifies that improve_slide rejects
// LLM output that fails lint (e.g. missing class="slide").
func TestE2E_ImproveSlide_InvalidHTML(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Lint Test", 2, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClient(t, root,
		client.WithSamplingHandler(func(_ context.Context, _ mcpcore.CreateMessageRequest) (mcpcore.CreateMessageResult, error) {
			// Return HTML without the required class="slide".
			return mcpcore.CreateMessageResult{
				Model: "test-model",
				Role:  "assistant",
				Content: mcpcore.Content{
					Type: "text",
					Text: `<div><h1>No slide class</h1></div>`,
				},
			}, nil
		}),
	)

	text, err := c.Client.ToolCall("improve_slide", map[string]any{
		"deck":        "deck",
		"slide":       "1",
		"instruction": "anything",
	})

	msg := text
	if err != nil {
		msg = err.Error()
	}
	if !strings.Contains(msg, "lint") && !strings.Contains(msg, `class="slide"`) {
		t.Errorf("expected lint error about class='slide', got: %s", msg)
	}
}
