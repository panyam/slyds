package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/slyds/core"
)



// TestInitWithThemeFlag verifies that slyds init --theme creates a presentation
// using the specified theme rather than the default.
func TestInitWithThemeFlag(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	slug, err := core.CreateWithTheme("Dark Talk", 3, "dark")
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
	jsPath := filepath.Join("..", "core", "slyds.js")
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
