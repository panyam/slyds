package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/slyds/core"
)

// TestPreviewScaffoldsFromDiskTheme verifies that CreateFromDir can scaffold
// a presentation from a theme directory on disk (not embedded in the binary),
// which is the core mechanism behind slyds preview.
func TestPreviewScaffoldsFromDiskTheme(t *testing.T) {
	// Create a minimal theme on disk
	themeDir := t.TempDir()
	writeTestTheme(t, themeDir)

	// Scaffold a presentation from it
	outDir := filepath.Join(t.TempDir(), "preview-test")
	err := core.CreateFromDir(outDir, "Preview Test", 3, themeDir)
	if err != nil {
		t.Fatalf("CreateFromDir failed: %v", err)
	}

	// Verify output
	for _, f := range []string{"index.html", "slyds.css", "slyds.js", "theme.css"} {
		if _, err := os.Stat(filepath.Join(outDir, f)); os.IsNotExist(err) {
			t.Errorf("missing file: %s", f)
		}
	}

	slides, _ := os.ReadDir(filepath.Join(outDir, "slides"))
	if len(slides) != 3 {
		t.Errorf("expected 3 slides, got %d", len(slides))
	}

	// Verify theme.css comes from our disk theme (not embedded default)
	themeCSS, _ := os.ReadFile(filepath.Join(outDir, "theme.css"))
	if !strings.Contains(string(themeCSS), "test-theme-marker") {
		t.Error("theme.css not rendered from disk theme")
	}
}

// TestPreviewAddIncludeToIndex verifies that addIncludeToIndex correctly
// inserts a templar include line before the navigation div.
func TestPreviewAddIncludeToIndex(t *testing.T) {
	tmp := t.TempDir()
	indexPath := filepath.Join(tmp, "index.html")

	indexHTML := `<div class="slideshow-container">
    {{# include "slides/01-title.html" #}}
    <div class="navigation">
    </div>
</div>`
	os.WriteFile(indexPath, []byte(indexHTML), 0644)

	err := addIncludeToIndex(indexPath, "02-custom.html")
	if err != nil {
		t.Fatalf("addIncludeToIndex failed: %v", err)
	}

	result, _ := os.ReadFile(indexPath)
	if !strings.Contains(string(result), `"slides/02-custom.html"`) {
		t.Error("include line not added")
	}

	// Should appear before navigation
	navIdx := strings.Index(string(result), "navigation")
	customIdx := strings.Index(string(result), "02-custom")
	if customIdx > navIdx {
		t.Error("include line added after navigation, should be before")
	}
}

// TestLoadThemeConfigFromDir verifies that theme.yaml can be loaded from
// a directory on disk (not from embedded FS).
func TestLoadThemeConfigFromDir(t *testing.T) {
	themeDir := t.TempDir()
	writeTestTheme(t, themeDir)

	cfg, err := core.LoadThemeConfigFromDir(themeDir)
	if err != nil {
		t.Fatalf("LoadThemeConfigFromDir failed: %v", err)
	}

	if cfg.Name != "test" {
		t.Errorf("Name = %q, want %q", cfg.Name, "test")
	}
	if _, ok := cfg.SlideTypes["content"]; !ok {
		t.Error("missing slide type 'content'")
	}
}

// writeTestTheme creates a minimal theme directory on disk for testing.
func writeTestTheme(t *testing.T, dir string) {
	t.Helper()
	os.MkdirAll(filepath.Join(dir, "slides"), 0755)

	os.WriteFile(filepath.Join(dir, "theme.yaml"), []byte(`name: test
description: Test theme
slide_types:
  title: slides/title.html.tmpl
  content: slides/content.html.tmpl
  closing: slides/closing.html.tmpl
`), 0644)

	os.WriteFile(filepath.Join(dir, "theme.css.tmpl"), []byte(
		"/* {{.Title}} — test-theme-marker */\nbody { background: red; }\n"), 0644)

	os.WriteFile(filepath.Join(dir, "index.html.tmpl"), []byte(
		"<!DOCTYPE html><html><head><title>{{.Title}}</title><link rel=\"stylesheet\" href=\"slyds.css\"><link rel=\"stylesheet\" href=\"theme.css\"></head><body><div class=\"slideshow-container\">\n{{.Includes}}    <div class=\"navigation\"></div>\n</div><script src=\"slyds.js\"></script></body></html>\n"), 0644)

	os.WriteFile(filepath.Join(dir, "slides", "title.html.tmpl"), []byte(
		"<div class=\"slide active title-slide\"><h1>{{.Title}}</h1></div>\n"), 0644)

	os.WriteFile(filepath.Join(dir, "slides", "content.html.tmpl"), []byte(
		"<div class=\"slide\"><h1>Slide {{.Number}}</h1></div>\n"), 0644)

	os.WriteFile(filepath.Join(dir, "slides", "closing.html.tmpl"), []byte(
		"<div class=\"slide conclusion-slide\"><h1>Thank You</h1></div>\n"), 0644)
}
