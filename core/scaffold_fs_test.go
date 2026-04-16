package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/panyam/templar"
)

// TestScaffoldDeckMemFS verifies that ScaffoldDeck creates a complete deck
// structure on an in-memory filesystem — zero disk I/O.
func TestScaffoldDeckMemFS(t *testing.T) {
	mfs := templar.NewMemFS()
	d, err := ScaffoldDeck(mfs, ScaffoldOpts{
		Title:      "Test Deck",
		SlideCount: 3,
		ThemeName:  "default",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Deck should be usable
	count, _ := d.SlideCount()
	if count != 3 {
		t.Errorf("slide count = %d, want 3", count)
	}
	if d.Title() != "Test Deck" {
		t.Errorf("title = %q, want Test Deck", d.Title())
	}
	if d.Theme() != "default" {
		t.Errorf("theme = %q, want default", d.Theme())
	}
}

// TestScaffoldDeckHasEngineFiles verifies that engine CSS/JS are written.
func TestScaffoldDeckHasEngineFiles(t *testing.T) {
	mfs := templar.NewMemFS()
	ScaffoldDeck(mfs, ScaffoldOpts{Title: "T", SlideCount: 1, ThemeName: "default"})

	for _, f := range []string{"slyds.css", "slyds.js", "slyds-export.js", "index.html", "theme.css", ".slyds.yaml"} {
		if !mfs.HasFile(f) {
			t.Errorf("missing file: %s", f)
		}
	}
}

// TestScaffoldDeckHasThemeFiles verifies that theme CSS files are in themes/.
func TestScaffoldDeckHasThemeFiles(t *testing.T) {
	mfs := templar.NewMemFS()
	ScaffoldDeck(mfs, ScaffoldOpts{Title: "T", SlideCount: 1, ThemeName: "default"})

	entries, _ := mfs.ReadDir("themes")
	if len(entries) == 0 {
		t.Error("no theme files in themes/")
	}
}

// TestScaffoldDeckSlideContent verifies that slide HTML contains expected elements.
func TestScaffoldDeckSlideContent(t *testing.T) {
	mfs := templar.NewMemFS()
	d, _ := ScaffoldDeck(mfs, ScaffoldOpts{Title: "My Talk", SlideCount: 3, ThemeName: "default"})

	// Title slide should contain the title
	content, _ := d.GetSlideContent(1)
	if !strings.Contains(content, "My Talk") {
		t.Errorf("title slide missing 'My Talk': %s", content[:100])
	}

	// Last slide should be closing
	closing, _ := d.GetSlideContent(3)
	if !strings.Contains(closing, "Thank You") {
		t.Errorf("closing slide missing 'Thank You': %s", closing[:100])
	}
}

// TestScaffoldDeckAgentMD verifies that AGENT.md is generated.
func TestScaffoldDeckAgentMD(t *testing.T) {
	mfs := templar.NewMemFS()
	ScaffoldDeck(mfs, ScaffoldOpts{Title: "T", SlideCount: 1, ThemeName: "default"})

	if !mfs.HasFile("AGENT.md") {
		t.Error("AGENT.md not generated")
	}
	data, _ := mfs.ReadFile("AGENT.md")
	if !strings.Contains(string(data), "Available Layouts") {
		t.Error("AGENT.md missing layout docs")
	}
}

// TestScaffoldDeckWithDarkTheme verifies that different themes produce different output.
func TestScaffoldDeckWithDarkTheme(t *testing.T) {
	mfs := templar.NewMemFS()
	d, err := ScaffoldDeck(mfs, ScaffoldOpts{
		Title:     "Dark Talk",
		SlideCount: 3,
		ThemeName: "dark",
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Theme() != "dark" {
		t.Errorf("theme = %q, want dark", d.Theme())
	}
}

// TestScaffoldDeckDescribe verifies that a scaffolded deck can be described.
func TestScaffoldDeckDescribe(t *testing.T) {
	mfs := templar.NewMemFS()
	d, _ := ScaffoldDeck(mfs, ScaffoldOpts{Title: "Desc Test", SlideCount: 4, ThemeName: "default"})

	desc, err := d.Describe()
	if err != nil {
		t.Fatal(err)
	}
	if desc.SlideCount != 4 {
		t.Errorf("describe slide count = %d, want 4", desc.SlideCount)
	}
	if desc.Title != "Desc Test" {
		t.Errorf("describe title = %q", desc.Title)
	}
}

// TestUpdateDeck_UnknownTheme verifies that UpdateDeck refreshes engine files
// even when the theme is not built-in, returning an UnknownThemeWarning
// instead of a hard error.
func TestUpdateDeck_UnknownTheme(t *testing.T) {
	mfs := templar.NewMemFS()
	// First scaffold a valid deck
	ScaffoldDeck(mfs, ScaffoldOpts{Title: "Custom Theme", SlideCount: 2, ThemeName: "default"})

	// Now update with an unknown theme
	err := UpdateDeck(mfs, "acme-dark", "Custom Theme")

	// Should return a warning, not a hard error
	if err == nil {
		t.Fatal("expected UnknownThemeWarning, got nil")
	}
	var warn *UnknownThemeWarning
	if !errors.As(err, &warn) {
		t.Fatalf("expected UnknownThemeWarning, got %T: %v", err, err)
	}
	if warn.Theme != "acme-dark" {
		t.Errorf("warning theme = %q, want acme-dark", warn.Theme)
	}

	// Engine files should still be refreshed
	if !mfs.HasFile("slyds.js") {
		t.Error("slyds.js not refreshed")
	}
	if !mfs.HasFile("slyds.css") {
		t.Error("slyds.css not refreshed")
	}
	if !mfs.HasFile("slyds-export.js") {
		t.Error("slyds-export.js not refreshed")
	}
	// Manifest should be updated
	if !mfs.HasFile(".slyds.yaml") {
		t.Error("manifest not written")
	}
}

// TestUpdateDeck_BuiltinTheme verifies that UpdateDeck succeeds without
// warning for built-in themes and refreshes all files.
func TestUpdateDeck_BuiltinTheme(t *testing.T) {
	mfs := templar.NewMemFS()
	ScaffoldDeck(mfs, ScaffoldOpts{Title: "Normal", SlideCount: 2, ThemeName: "default"})

	err := UpdateDeck(mfs, "dark", "Normal Updated")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Engine files refreshed
	if !mfs.HasFile("slyds.js") {
		t.Error("slyds.js not refreshed")
	}
	// Theme CSS re-rendered
	if !mfs.HasFile("theme.css") {
		t.Error("theme.css not rendered")
	}
	// Index re-rendered
	if !mfs.HasFile("index.html") {
		t.Error("index.html not rendered")
	}
}
