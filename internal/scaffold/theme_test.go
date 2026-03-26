package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestListThemes verifies that ListThemes discovers all built-in themes
// from the embedded filesystem (default + minimal + dark + corporate).
func TestListThemes(t *testing.T) {
	themes, err := ListThemes()
	if err != nil {
		t.Fatalf("ListThemes failed: %v", err)
	}

	expected := []string{"corporate", "dark", "default", "hacker", "minimal"}
	if len(themes) != len(expected) {
		t.Fatalf("ListThemes returned %d themes %v, want %d %v", len(themes), themes, len(expected), expected)
	}
	for i, name := range expected {
		if themes[i] != name {
			t.Errorf("themes[%d] = %q, want %q", i, themes[i], name)
		}
	}
}

// TestCreateWithThemeMinimal verifies that scaffolding with the "minimal" theme
// produces a valid presentation with minimal-specific styling in theme.css.
func TestCreateWithThemeMinimal(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	slug, err := CreateWithTheme("Minimal Talk", 3, "minimal")
	if err != nil {
		t.Fatalf("CreateWithTheme(minimal) failed: %v", err)
	}

	dir := filepath.Join(tmp, slug)
	assertPresentationFiles(t, dir, 3)

	// theme.css should have minimal-specific content (no gradients)
	themeCSS, _ := os.ReadFile(filepath.Join(dir, "theme.css"))
	if strings.Contains(string(themeCSS), "linear-gradient") {
		t.Error("minimal theme.css should not contain gradients")
	}
}

// TestCreateWithThemeDark verifies that scaffolding with the "dark" theme
// produces a presentation with dark-specific styling (dark backgrounds).
func TestCreateWithThemeDark(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	slug, err := CreateWithTheme("Dark Talk", 3, "dark")
	if err != nil {
		t.Fatalf("CreateWithTheme(dark) failed: %v", err)
	}

	dir := filepath.Join(tmp, slug)
	assertPresentationFiles(t, dir, 3)

	// Title slide should have dark-specific class
	titleSlide, _ := os.ReadFile(filepath.Join(dir, "slides", "01-title.html"))
	if !strings.Contains(string(titleSlide), "title-slide") {
		t.Error("dark theme title slide missing title-slide class")
	}
}

// TestCreateWithThemeCorporate verifies that scaffolding with the "corporate" theme
// produces a presentation with corporate-specific styling.
func TestCreateWithThemeCorporate(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	slug, err := CreateWithTheme("Q3 Review", 4, "corporate")
	if err != nil {
		t.Fatalf("CreateWithTheme(corporate) failed: %v", err)
	}

	dir := filepath.Join(tmp, slug)
	assertPresentationFiles(t, dir, 4)
}

// TestCreateWithInvalidTheme verifies that requesting a non-existent theme
// returns a clear error rather than creating a broken presentation.
func TestCreateWithInvalidTheme(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	_, err := CreateWithTheme("Bad Theme", 3, "nonexistent")
	if err == nil {
		t.Fatal("expected error for invalid theme, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention theme name, got: %v", err)
	}
}

// TestThemesDifferentCSS verifies that each theme produces distinct theme.css
// content so they are actually different visual experiences.
func TestThemesDifferentCSS(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	themes := []string{"default", "minimal", "dark", "corporate"}
	cssContents := make(map[string]string)

	for _, theme := range themes {
		subdir := filepath.Join(tmp, theme+"-test")
		os.MkdirAll(subdir, 0755)
		os.Chdir(subdir)

		slug, err := CreateWithTheme("Test "+theme, 3, theme)
		if err != nil {
			t.Fatalf("CreateWithTheme(%s) failed: %v", theme, err)
		}
		css, err := os.ReadFile(filepath.Join(subdir, slug, "theme.css"))
		if err != nil {
			t.Fatalf("failed to read theme.css for %s: %v", theme, err)
		}
		cssContents[theme] = string(css)
	}

	// Each theme's CSS should be distinct
	for i, a := range themes {
		for _, b := range themes[i+1:] {
			if cssContents[a] == cssContents[b] {
				t.Errorf("theme %q and %q produced identical theme.css", a, b)
			}
		}
	}
}

// assertPresentationFiles checks that all required files exist for a scaffolded presentation.
func assertPresentationFiles(t *testing.T, dir string, slideCount int) {
	t.Helper()

	requiredFiles := []string{"index.html", "slyds.css", "slyds.js", "theme.css"}
	for _, f := range requiredFiles {
		if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
			t.Errorf("missing file: %s", f)
		}
	}

	// Check slide count
	slides, err := os.ReadDir(filepath.Join(dir, "slides"))
	if err != nil {
		t.Fatalf("failed to read slides dir: %v", err)
	}
	if len(slides) != slideCount {
		t.Errorf("expected %d slides, got %d", slideCount, len(slides))
	}

	// Check index.html has includes
	indexHTML, _ := os.ReadFile(filepath.Join(dir, "index.html"))
	if !strings.Contains(string(indexHTML), "{{# include") {
		t.Error("index.html missing templar include directives")
	}
}
