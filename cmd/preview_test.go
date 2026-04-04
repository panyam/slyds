package cmd

import (
	"strings"
	"testing"

	"github.com/panyam/slyds/core"
	"github.com/panyam/templar"
)

// TestPreviewScaffoldsFromDiskTheme verifies that CreateFromDir can scaffold
// a presentation from a theme directory on disk, and the output deck
// is openable via Deck API with the correct theme.
func TestPreviewScaffoldsFromDiskTheme(t *testing.T) {
	themeDir := t.TempDir()
	writeTestTheme(t, themeDir)

	outDir := t.TempDir()
	err := core.CreateFromDir(outDir, "Preview Test", 3, themeDir)
	if err != nil {
		t.Fatalf("CreateFromDir failed: %v", err)
	}

	// Open as Deck and verify structure
	d, err := core.OpenDeckDir(outDir)
	if err != nil {
		t.Fatalf("OpenDeckDir failed: %v", err)
	}

	// Verify expected files exist via DeckFS
	for _, f := range []string{"index.html", "slyds.css", "slyds.js", "theme.css"} {
		if _, err := d.FS.ReadFile(f); err != nil {
			t.Errorf("missing file: %s", f)
		}
	}

	count, _ := d.SlideCount()
	if count != 3 {
		t.Errorf("expected 3 slides, got %d", count)
	}

	// Verify theme.css comes from disk theme (not embedded default)
	themeCSS, _ := d.FS.ReadFile("theme.css")
	if !strings.Contains(string(themeCSS), "test-theme-marker") {
		t.Error("theme.css not rendered from disk theme")
	}
}

// TestLoadThemeFromLocalFS verifies that a theme directory on disk can be
// loaded as a Theme via templar.LocalFS.
func TestLoadThemeFromLocalFS(t *testing.T) {
	themeDir := t.TempDir()
	writeTestTheme(t, themeDir)

	theme, err := core.LoadTheme(templar.NewLocalFS(themeDir))
	if err != nil {
		t.Fatalf("LoadTheme failed: %v", err)
	}

	if theme.Config.Name != "test" {
		t.Errorf("Name = %q, want %q", theme.Config.Name, "test")
	}

	// Verify rendering works through the Theme
	html, err := theme.RenderSlideWithTitle("content", "demo", 1, "")
	if err != nil {
		t.Fatalf("RenderSlideWithTitle failed: %v", err)
	}
	if !strings.Contains(html, "Slide 1") {
		t.Errorf("render missing slide number: %s", html)
	}
}

// writeTestTheme creates a minimal theme directory for testing.
// Uses templar.LocalFS to write files through the FS abstraction.
func writeTestTheme(t *testing.T, dir string) {
	t.Helper()
	fs := templar.NewLocalFS(dir)
	fs.MkdirAll("slides", 0755)

	fs.WriteFile("theme.yaml", []byte(`name: test
description: Test theme
slide_types:
  title: slides/title.html.tmpl
  content: slides/content.html.tmpl
  closing: slides/closing.html.tmpl
`), 0644)

	fs.WriteFile("theme.css.tmpl", []byte(
		"/* {{.Title}} — test-theme-marker */\nbody { background: red; }\n"), 0644)

	fs.WriteFile("index.html.tmpl", []byte(
		"<!DOCTYPE html><html><head><title>{{.Title}}</title><link rel=\"stylesheet\" href=\"slyds.css\"><link rel=\"stylesheet\" href=\"theme.css\"></head><body><div class=\"slideshow-container\">\n{{.Includes}}    <div class=\"navigation\"></div>\n</div><script src=\"slyds.js\"></script></body></html>\n"), 0644)

	fs.WriteFile("slides/title.html.tmpl", []byte(
		"<div class=\"slide active title-slide\"><h1>{{.Title}}</h1></div>\n"), 0644)

	fs.WriteFile("slides/content.html.tmpl", []byte(
		"<div class=\"slide\"><h1>Slide {{.Number}}</h1></div>\n"), 0644)

	fs.WriteFile("slides/closing.html.tmpl", []byte(
		"<div class=\"slide conclusion-slide\"><h1>Thank You</h1></div>\n"), 0644)
}
