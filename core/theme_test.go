package core

import (
	"strings"
	"testing"

	"github.com/panyam/templar"
)

// TestListThemes verifies that ListThemes discovers all built-in themes.
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

func TestCreateWithThemeMinimal(t *testing.T) {
	_, mfs := scaffoldMem(t, "Minimal Talk", withTheme("minimal"))
	assertMemFiles(t, mfs, 3)

	themeCSS := readFile(t, mfs, "theme.css")
	if strings.Contains(themeCSS, "linear-gradient") {
		t.Error("minimal theme.css should not contain gradients")
	}
}

func TestCreateWithThemeDark(t *testing.T) {
	_, mfs := scaffoldMem(t, "Dark Talk", withTheme("dark"))
	assertMemFiles(t, mfs, 3)

	titleSlide := readFile(t, mfs, "slides/01-title.html")
	if !strings.Contains(titleSlide, "title-slide") {
		t.Error("dark theme title slide missing title-slide class")
	}
}

func TestCreateWithThemeCorporate(t *testing.T) {
	_, mfs := scaffoldMem(t, "Q3 Review", withTheme("corporate"), withSlides(4))
	assertMemFiles(t, mfs, 4)
}

func TestCreateWithInvalidTheme(t *testing.T) {
	mfs := templar.NewMemFS()
	_, err := ScaffoldDeck(mfs, ScaffoldOpts{
		Title:      "Bad Theme",
		SlideCount: 3,
		ThemeName:  "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for invalid theme, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention theme name, got: %v", err)
	}
}

func TestThemesDifferentCSS(t *testing.T) {
	themes := []string{"default", "minimal", "dark", "corporate"}
	cssContents := make(map[string]string)

	for _, theme := range themes {
		_, mfs := scaffoldMem(t, "Test "+theme, withTheme(theme))
		cssContents[theme] = readFile(t, mfs, "theme.css")
	}

	for i, a := range themes {
		for _, b := range themes[i+1:] {
			if cssContents[a] == cssContents[b] {
				t.Errorf("theme %q and %q produced identical theme.css", a, b)
			}
		}
	}
}

// assertMemFiles checks required files exist on MemFS for a scaffolded deck.
func assertMemFiles(t *testing.T, mfs *templar.MemFS, slideCount int) {
	t.Helper()

	requiredFiles := []string{"index.html", "slyds.css", "slyds.js", "theme.css"}
	for _, f := range requiredFiles {
		if !hasFile(mfs, f) {
			t.Errorf("missing file: %s", f)
		}
	}

	// Check slide count
	entries, err := mfs.ReadDir("slides")
	if err != nil {
		t.Fatalf("failed to read slides dir: %v", err)
	}
	if len(entries) != slideCount {
		t.Errorf("expected %d slides, got %d", slideCount, len(entries))
	}

	// Check index.html has includes
	indexHTML := readFile(t, mfs, "index.html")
	if !strings.Contains(indexHTML, "{{# include") {
		t.Error("index.html missing templar include directives")
	}
}
