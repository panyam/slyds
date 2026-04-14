package core

import (
	"strings"
	"testing"

	"github.com/panyam/templar"
)

func TestFlattenIncludes(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.WriteFile("slides/01-title.html", []byte(`<div class="slide"><h1>Hello</h1></div>`), 0644)

	d := &Deck{FS: mfs}
	html := `<html>
{{# include "slides/01-title.html" #}}
</html>`

	result, err := d.FlattenIncludes(html)
	if err != nil {
		t.Fatalf("FlattenIncludes failed: %v", err)
	}

	if !strings.Contains(result, `<div class="slide"><h1>Hello</h1></div>`) {
		t.Error("include directive not resolved")
	}
	if strings.Contains(result, "{{#") {
		t.Error("include directive still present in output")
	}
}

func TestFlattenIncludesMultiple(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.WriteFile("slides/01.html", []byte(`<div>Slide 1</div>`), 0644)
	mfs.WriteFile("slides/02.html", []byte(`<div>Slide 2</div>`), 0644)

	d := &Deck{FS: mfs}
	html := `<html>
{{# include "slides/01.html" #}}
{{# include "slides/02.html" #}}
</html>`

	result, err := d.FlattenIncludes(html)
	if err != nil {
		t.Fatalf("FlattenIncludes failed: %v", err)
	}

	if !strings.Contains(result, "Slide 1") || !strings.Contains(result, "Slide 2") {
		t.Error("not all includes resolved")
	}
}

func TestBuildIncludesExportJS(t *testing.T) {
	d, _ := scaffoldMem(t, "Build Export Test")

	result, err := d.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if !strings.Contains(result.HTML, "exportPresentation") {
		t.Error("built HTML missing inlined export JS (exportPresentation function)")
	}
	if !strings.Contains(result.HTML, `class="export-btn"`) {
		t.Error("built HTML missing export button")
	}
}

// TestBuild_AllThemes verifies that Build succeeds across all built-in themes
// and produces output with the correct data-theme attribute.
func TestBuild_AllThemes(t *testing.T) {
	for _, theme := range []string{"default", "dark", "minimal", "corporate", "hacker"} {
		t.Run(theme, func(t *testing.T) {
			d, _ := scaffoldMem(t, "Theme "+theme, withTheme(theme), withSlides(2))
			result, err := d.Build()
			if err != nil {
				t.Fatalf("Build with theme %s: %v", theme, err)
			}
			if !strings.Contains(result.HTML, `data-theme="`+theme+`"`) {
				t.Errorf("Build output missing data-theme=%q", theme)
			}
		})
	}
}

// TestBuild_InlinesJS verifies that Build inlines external script references.
func TestBuild_InlinesJS(t *testing.T) {
	d, _ := scaffoldMem(t, "JS Inline Test")
	result, err := d.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if strings.Contains(result.HTML, `<script src=`) {
		t.Error("Build output still has external <script src> tags — JS not inlined")
	}
	if !strings.Contains(result.HTML, "showSlide") {
		t.Error("Build output missing inlined slyds.js (showSlide function)")
	}
}

func TestFlattenIncludesMissingFile(t *testing.T) {
	mfs := templar.NewMemFS()
	d := &Deck{FS: mfs}

	html := `{{# include "nonexistent.html" #}}`
	_, err := d.FlattenIncludes(html)
	if err == nil {
		t.Error("expected error for missing include file")
	}
}
