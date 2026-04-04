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

func TestFlattenIncludesMissingFile(t *testing.T) {
	mfs := templar.NewMemFS()
	d := &Deck{FS: mfs}

	html := `{{# include "nonexistent.html" #}}`
	_, err := d.FlattenIncludes(html)
	if err == nil {
		t.Error("expected error for missing include file")
	}
}
