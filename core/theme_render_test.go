package core

import (
	"strings"
	"testing"

	"github.com/panyam/templar"
)

func testThemeFS() *templar.MemFS {
	mfs := templar.NewMemFS()
	mfs.SetFile("theme.yaml", []byte(`name: test-theme
description: A test theme
slide_types:
  title: title.html
  content: content.html
  section: section.html
`))
	mfs.SetFile("title.html", []byte(`<div class="slide title-slide"><h1>{{ .Title }}</h1><p>Slide {{ .Number }}</p></div>`))
	mfs.SetFile("content.html", []byte(`<div class="slide content-slide"><h1>{{ .Title }}</h1><div class="body">Body</div></div>`))
	mfs.SetFile("section.html", []byte(`<div class="slide section-slide"><h1>{{ .Title }}</h1></div>`))
	return mfs
}

// TestLoadTheme verifies that LoadTheme reads theme.yaml and populates Config.
func TestLoadTheme(t *testing.T) {
	theme, err := LoadTheme(testThemeFS())
	if err != nil {
		t.Fatal(err)
	}
	if theme.Config.Name != "test-theme" {
		t.Errorf("name = %q, want test-theme", theme.Config.Name)
	}
	if len(theme.Config.SlideTypes) != 3 {
		t.Errorf("slide types = %d, want 3", len(theme.Config.SlideTypes))
	}
}

// TestLoadThemeNoYaml verifies error when theme.yaml is missing.
func TestLoadThemeNoYaml(t *testing.T) {
	mfs := templar.NewMemFS()
	_, err := LoadTheme(mfs)
	if err == nil {
		t.Error("expected error for missing theme.yaml")
	}
}

// TestThemeRenderSlide verifies that RenderSlide produces correct HTML
// with template interpolation.
func TestThemeRenderSlide(t *testing.T) {
	theme, _ := LoadTheme(testThemeFS())

	html, err := theme.RenderSlide("title", map[string]any{
		"Title":  "Welcome",
		"Number": 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "Welcome") {
		t.Errorf("missing title: %s", html)
	}
	if !strings.Contains(html, "Slide 1") {
		t.Errorf("missing number: %s", html)
	}
	if !strings.Contains(html, "title-slide") {
		t.Errorf("missing CSS class: %s", html)
	}
}

// TestThemeRenderSlideContent verifies content layout rendering.
func TestThemeRenderSlideContent(t *testing.T) {
	theme, _ := LoadTheme(testThemeFS())

	html, err := theme.RenderSlide("content", map[string]any{
		"Title":  "Details",
		"Number": 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "content-slide") {
		t.Errorf("missing content-slide class: %s", html)
	}
}

// TestThemeRenderSlideUnknownType verifies error for unknown slide type.
func TestThemeRenderSlideUnknownType(t *testing.T) {
	theme, _ := LoadTheme(testThemeFS())

	_, err := theme.RenderSlide("nonexistent", map[string]any{})
	if err == nil {
		t.Error("expected error for unknown slide type")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want 'not found'", err)
	}
}

// TestThemeRenderSlideWithTitle verifies the convenience method with
// slug-to-title conversion and explicit override.
func TestThemeRenderSlideWithTitle(t *testing.T) {
	theme, _ := LoadTheme(testThemeFS())

	// Auto title from slug
	html, err := theme.RenderSlideWithTitle("title", "my-intro", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "My Intro") {
		t.Errorf("auto title not applied: %s", html)
	}

	// Explicit title override
	html2, _ := theme.RenderSlideWithTitle("title", "ignored-slug", 1, "Custom Title")
	if !strings.Contains(html2, "Custom Title") {
		t.Errorf("title override not applied: %s", html2)
	}
}

// TestLoadEmbeddedTheme verifies that built-in themes load from embedded FS.
func TestLoadEmbeddedTheme(t *testing.T) {
	theme, err := LoadEmbeddedTheme("default")
	if err != nil {
		t.Fatal(err)
	}
	if theme.Config.Name == "" {
		t.Error("expected non-empty theme name")
	}
	if len(theme.Config.SlideTypes) == 0 {
		t.Error("expected slide types in default theme")
	}
}

// TestLoadEmbeddedThemeRender verifies end-to-end: load embedded → render slide.
func TestLoadEmbeddedThemeRender(t *testing.T) {
	theme, _ := LoadEmbeddedTheme("default")

	html, err := theme.RenderSlideWithTitle("content", "test-slide", 5, "")
	if err != nil {
		t.Fatal(err)
	}
	if html == "" {
		t.Error("empty render output")
	}
	// Embedded content template uses "Slide N" as heading, not the title
	if !strings.Contains(html, "Slide 5") {
		t.Errorf("render output missing slide number: %s", html)
	}
}

// TestThemeSlideTypes verifies the SlideTypes() listing method.
func TestThemeSlideTypes(t *testing.T) {
	theme, _ := LoadTheme(testThemeFS())
	types := theme.SlideTypes()
	if len(types) != 3 {
		t.Errorf("got %d types, want 3", len(types))
	}
}
