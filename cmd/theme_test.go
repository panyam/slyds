package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/slyds/internal/scaffold"
)

// TestRenderSlideFromThemeTwoColumn verifies that a two-column layout slide
// can be rendered from the embedded theme template and contains the expected
// CSS class and column structure.
func TestRenderSlideFromThemeTwoColumn(t *testing.T) {
	content, err := renderSlideFromTheme("", "architecture", "two-column", 3)
	if err != nil {
		t.Fatalf("renderSlideFromTheme(two-column) failed: %v", err)
	}

	if !strings.Contains(content, "layout-two-column") {
		t.Error("two-column slide missing layout-two-column class")
	}
	if !strings.Contains(content, "col-left") {
		t.Error("two-column slide missing col-left")
	}
	if !strings.Contains(content, "col-right") {
		t.Error("two-column slide missing col-right")
	}
}

// TestRenderSlideFromThemeSection verifies that a section divider slide
// can be rendered and contains the expected CSS class.
func TestRenderSlideFromThemeSection(t *testing.T) {
	content, err := renderSlideFromTheme("", "part-two", "section", 4)
	if err != nil {
		t.Fatalf("renderSlideFromTheme(section) failed: %v", err)
	}

	if !strings.Contains(content, "section-slide") {
		t.Error("section slide missing section-slide class")
	}
}

// TestInitWithThemeFlag verifies that slyds init --theme creates a presentation
// using the specified theme rather than the default.
func TestInitWithThemeFlag(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	slug, err := scaffold.CreateWithTheme("Dark Talk", 3, "dark")
	if err != nil {
		t.Fatalf("CreateWithTheme(dark) failed: %v", err)
	}

	dir := filepath.Join(tmp, slug)
	themeCSS, _ := os.ReadFile(filepath.Join(dir, "theme.css"))
	themeCSSStr := string(themeCSS)

	// Dark theme should contain structural overrides (color-mix progress, nth-child alternation)
	if !strings.Contains(themeCSSStr, "color-mix") && !strings.Contains(themeCSSStr, "nth-child") &&
		!strings.Contains(themeCSSStr, "text-shadow") {
		t.Error("dark theme.css doesn't contain dark-specific structural styling")
	}
}

// TestPositionAwareCSS verifies that slyds.js sets CSS custom properties
// for slide position (--slide-index, --slide-progress) by checking that
// the JS source contains the property-setting code.
func TestPositionAwareCSS(t *testing.T) {
	// Read slyds.js and verify it sets position custom properties
	jsPath := filepath.Join("..", "assets", "slyds.js")
	js, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("failed to read slyds.js: %v", err)
	}
	jsStr := string(js)

	if !strings.Contains(jsStr, "--slide-index") {
		t.Error("slyds.js missing --slide-index custom property")
	}
	if !strings.Contains(jsStr, "--slide-progress") {
		t.Error("slyds.js missing --slide-progress custom property")
	}
}
