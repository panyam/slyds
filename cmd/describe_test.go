package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/panyam/slyds/core"
)

// openAndDescribe opens a Deck from root and calls Describe.
func openAndDescribe(t *testing.T, root string) (*core.DeckDescription, error) {
	t.Helper()
	d, err := core.OpenDeckDir(root)
	if err != nil {
		return nil, err
	}
	return d.Describe()
}

// TestDescribeDeck verifies that Describe returns a complete structured
// summary of a freshly scaffolded presentation.
func TestDescribeDeck(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	desc, err := openAndDescribe(t, root)
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
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
	if len(desc.ThemesAvailable) == 0 {
		t.Error("expected non-empty themes_available")
	}
	if len(desc.LayoutsAvailable) == 0 {
		t.Error("expected non-empty layouts_available")
	}
}

// TestDescribeSlideMetadata verifies per-slide metadata.
func TestDescribeSlideMetadata(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	desc, err := openAndDescribe(t, root)
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
	}

	if len(desc.Slides) != 4 {
		t.Fatalf("expected 4 slides, got %d", len(desc.Slides))
	}

	first := desc.Slides[0]
	if first.Position != 1 {
		t.Errorf("first slide position = %d, want 1", first.Position)
	}
	if first.Layout != "title" {
		t.Errorf("first slide layout = %q, want %q", first.Layout, "title")
	}
	if first.Title == "" {
		t.Error("first slide has empty title")
	}

	last := desc.Slides[len(desc.Slides)-1]
	if last.Layout != "closing" {
		t.Errorf("last slide layout = %q, want %q", last.Layout, "closing")
	}
}

// TestDescribeDetectsLayouts verifies layout detection in Describe.
func TestDescribeDetectsLayouts(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	desc, err := openAndDescribe(t, root)
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
	}

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

// TestDescribeSpeakerNotes verifies speaker notes detection.
func TestDescribeSpeakerNotes(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	desc, err := openAndDescribe(t, root)
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
	}

	for _, slide := range desc.Slides {
		if !slide.HasNotes {
			t.Errorf("slide %d (%s) missing speaker notes", slide.Position, slide.File)
		}
	}
}

// TestDescribeWordCount verifies word counting in Describe.
func TestDescribeWordCount(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	desc, err := openAndDescribe(t, root)
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
	}

	for _, slide := range desc.Slides {
		if slide.Words <= 0 {
			t.Errorf("slide %d (%s) has %d words", slide.Position, slide.File, slide.Words)
		}
	}
}

// TestDescribeJSONOutput verifies JSON serialization of the description.
func TestDescribeJSONOutput(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	desc, err := openAndDescribe(t, root)
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
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

// TestDescribeWithInsertedSlide verifies Describe after inserting a slide.
func TestDescribeWithInsertedSlide(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	err := runInsert(root, 2, "comparison", "two-col", "Side by Side")
	if err != nil {
		t.Fatalf("InsertSlide failed: %v", err)
	}

	desc, err := openAndDescribe(t, root)
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
	}

	if desc.SlideCount != 5 {
		t.Errorf("slide_count = %d, want 5", desc.SlideCount)
	}

	second := desc.Slides[1]
	if second.Layout != "two-col" {
		t.Errorf("inserted slide layout = %q, want %q", second.Layout, "two-col")
	}
	if second.Title != "Side by Side" {
		t.Errorf("inserted slide title = %q, want %q", second.Title, "Side by Side")
	}
}

// TestAgentMDGenerated verifies that scaffold creates AGENT.md.
// Uses Deck.FS to check file existence instead of os.Stat.
func TestAgentMDGenerated(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	d, _ := core.OpenDeckDir(root)
	if _, err := d.FS.ReadFile("AGENT.md"); err != nil {
		t.Error("AGENT.md not generated by scaffold")
	}
}

// TestAgentMDContainsLayouts verifies AGENT.md documents layouts.
func TestAgentMDContainsLayouts(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	d, _ := core.OpenDeckDir(root)
	data, err := d.FS.ReadFile("AGENT.md")
	if err != nil {
		t.Fatalf("failed to read AGENT.md: %v", err)
	}
	content := string(data)

	for _, expected := range []string{"## Available Layouts", "two-col", "title", "content", "## Available Themes", "## Quick Reference", "slyds describe"} {
		if !strings.Contains(content, expected) {
			t.Errorf("AGENT.md missing: %q", expected)
		}
	}
}

// TestAgentMDContainsTitle verifies AGENT.md includes the presentation title.
func TestAgentMDContainsTitle(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	d, _ := core.OpenDeckDir(root)
	data, err := d.FS.ReadFile("AGENT.md")
	if err != nil {
		t.Fatalf("failed to read AGENT.md: %v", err)
	}
	if !strings.Contains(string(data), "Test Pres") {
		t.Error("AGENT.md missing presentation title")
	}
}
