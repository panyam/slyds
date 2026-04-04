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
	d, err := core.OpenDeckDir(dir)
	if err != nil {
		t.Fatalf("OpenDeckDir failed: %v", err)
	}

	themeCSS, _ := d.FS.ReadFile("theme.css")
	themeCSSStr := string(themeCSS)

	if !strings.Contains(themeCSSStr, "color-mix") && !strings.Contains(themeCSSStr, "nth-child") &&
		!strings.Contains(themeCSSStr, "text-shadow") {
		t.Error("dark theme.css doesn't contain dark-specific structural styling")
	}
}

// TestPositionAwareCSS verifies that the embedded slyds.js sets CSS custom
// properties for slide position (--slide-index, --slide-progress).
func TestPositionAwareCSS(t *testing.T) {
	jsStr := core.SlydsJS
	if !strings.Contains(jsStr, "--slide-index") {
		t.Error("slyds.js missing --slide-index custom property")
	}
	if !strings.Contains(jsStr, "--slide-progress") {
		t.Error("slyds.js missing --slide-progress custom property")
	}
}
