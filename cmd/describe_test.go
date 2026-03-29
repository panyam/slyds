package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDescribeDeck verifies that describeDeck returns a complete structured
// summary of a freshly scaffolded presentation, including correct slide count,
// layout detection, title extraction, and available themes/layouts.
func TestDescribeDeck(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	desc, err := describeDeck(root)
	if err != nil {
		t.Fatalf("describeDeck failed: %v", err)
	}

	if desc.Title != "Test Pres" {
		t.Errorf("title = %q, want %q", desc.Title, "Test Pres")
	}
	if desc.Theme != "default" {
		t.Errorf("theme = %q, want %q", desc.Theme, "default")
	}
	if desc.SlideCount != 4 {
		t.Errorf("slide_count = %d, want %d", desc.SlideCount, 4)
	}

	// Should have themes and layouts available
	if len(desc.ThemesAvailable) == 0 {
		t.Error("expected non-empty themes_available")
	}
	if len(desc.LayoutsAvailable) == 0 {
		t.Error("expected non-empty layouts_available")
	}
}

// TestDescribeSlideMetadata verifies that each slide in the description
// has the expected metadata: position, file name, layout, title, and word count.
func TestDescribeSlideMetadata(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	desc, err := describeDeck(root)
	if err != nil {
		t.Fatalf("describeDeck failed: %v", err)
	}

	if len(desc.Slides) != 4 {
		t.Fatalf("expected 4 slides, got %d", len(desc.Slides))
	}

	// First slide should be title layout
	first := desc.Slides[0]
	if first.Position != 1 {
		t.Errorf("first slide position = %d, want 1", first.Position)
	}
	if first.Layout != "title" {
		t.Errorf("first slide layout = %q, want %q", first.Layout, "title")
	}
	if first.File != "01-title.html" {
		t.Errorf("first slide file = %q, want %q", first.File, "01-title.html")
	}
	if first.Title == "" {
		t.Error("first slide has empty title")
	}

	// Last slide should be closing layout
	last := desc.Slides[len(desc.Slides)-1]
	if last.Layout != "closing" {
		t.Errorf("last slide layout = %q, want %q", last.Layout, "closing")
	}
}

// TestDescribeDetectsLayouts verifies that describeDeck correctly reports
// which layouts are used in the deck.
func TestDescribeDetectsLayouts(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	desc, err := describeDeck(root)
	if err != nil {
		t.Fatalf("describeDeck failed: %v", err)
	}

	// A default scaffolded deck should have title, content, and closing layouts
	layoutSet := map[string]bool{}
	for _, l := range desc.LayoutsUsed {
		layoutSet[l] = true
	}

	if !layoutSet["title"] {
		t.Error("expected 'title' in layouts_used")
	}
	if !layoutSet["closing"] {
		t.Error("expected 'closing' in layouts_used")
	}
}

// TestDescribeSpeakerNotes verifies that describeDeck detects the presence
// of speaker notes in slides.
func TestDescribeSpeakerNotes(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	desc, err := describeDeck(root)
	if err != nil {
		t.Fatalf("describeDeck failed: %v", err)
	}

	// Scaffolded slides should have speaker notes
	for _, slide := range desc.Slides {
		if !slide.HasNotes {
			t.Errorf("slide %d (%s) missing speaker notes", slide.Position, slide.File)
		}
	}
}

// TestDescribeWordCount verifies that describeDeck counts words in visible
// slide content (excluding speaker notes and HTML tags).
func TestDescribeWordCount(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	desc, err := describeDeck(root)
	if err != nil {
		t.Fatalf("describeDeck failed: %v", err)
	}

	for _, slide := range desc.Slides {
		if slide.Words <= 0 {
			t.Errorf("slide %d (%s) has %d words — expected positive count", slide.Position, slide.File, slide.Words)
		}
	}
}

// TestDescribeJSONOutput verifies that the deck description can be marshaled
// to valid JSON with expected fields present.
func TestDescribeJSONOutput(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	desc, err := describeDeck(root)
	if err != nil {
		t.Fatalf("describeDeck failed: %v", err)
	}

	data, err := json.MarshalIndent(desc, "", "  ")
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	jsonStr := string(data)
	for _, field := range []string{"title", "theme", "slide_count", "slides", "layouts_used"} {
		if !strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("JSON output missing field %q", field)
		}
	}
}

// TestDescribeWithInsertedSlide verifies that describeDeck correctly handles
// a deck with a manually inserted slide, detecting its layout and metadata.
func TestDescribeWithInsertedSlide(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Insert a two-col slide
	err := runInsert(root, 2, "comparison", "two-col", "Side by Side")
	if err != nil {
		t.Fatalf("runInsert failed: %v", err)
	}

	desc, err := describeDeck(root)
	if err != nil {
		t.Fatalf("describeDeck failed: %v", err)
	}

	if desc.SlideCount != 5 {
		t.Errorf("slide_count = %d, want 5", desc.SlideCount)
	}

	// Second slide should be our inserted two-col
	second := desc.Slides[1]
	if second.Layout != "two-col" {
		t.Errorf("inserted slide layout = %q, want %q", second.Layout, "two-col")
	}
	if second.Title != "Side by Side" {
		t.Errorf("inserted slide title = %q, want %q", second.Title, "Side by Side")
	}
}

// TestAgentMDGenerated verifies that slyds init creates an AGENT.md file
// and a CLAUDE.md symlink in the deck directory.
func TestAgentMDGenerated(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Check AGENT.md exists
	agentPath := filepath.Join(root, "AGENT.md")
	if _, err := os.Stat(agentPath); os.IsNotExist(err) {
		t.Error("AGENT.md not generated by scaffold")
	}

	// Check CLAUDE.md symlink exists
	claudePath := filepath.Join(root, "CLAUDE.md")
	info, err := os.Lstat(claudePath)
	if os.IsNotExist(err) {
		t.Error("CLAUDE.md symlink not created")
	} else if err == nil && info.Mode()&os.ModeSymlink == 0 {
		t.Error("CLAUDE.md exists but is not a symlink")
	}
}

// TestAgentMDContainsLayouts verifies that the generated AGENT.md includes
// documentation about available layouts and their slots.
func TestAgentMDContainsLayouts(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	data, err := os.ReadFile(filepath.Join(root, "AGENT.md"))
	if err != nil {
		t.Fatalf("failed to read AGENT.md: %v", err)
	}
	content := string(data)

	for _, expected := range []string{"## Available Layouts", "two-col", "title", "content", "## Available Themes", "## Quick Reference", "slyds describe"} {
		if !strings.Contains(content, expected) {
			t.Errorf("AGENT.md missing expected content: %q", expected)
		}
	}
}

// TestAgentMDContainsTitle verifies that the generated AGENT.md includes
// the presentation title from the manifest.
func TestAgentMDContainsTitle(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	data, err := os.ReadFile(filepath.Join(root, "AGENT.md"))
	if err != nil {
		t.Fatalf("failed to read AGENT.md: %v", err)
	}
	if !strings.Contains(string(data), "Test Pres") {
		t.Error("AGENT.md missing presentation title")
	}
}
