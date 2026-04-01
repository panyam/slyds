package builder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/slyds/internal/scaffold"
)

func TestFlattenIncludes(t *testing.T) {
	tmp := t.TempDir()

	// Create a slide file
	slidesDir := filepath.Join(tmp, "slides")
	os.MkdirAll(slidesDir, 0755)
	os.WriteFile(filepath.Join(slidesDir, "01-title.html"), []byte(`<div class="slide"><h1>Hello</h1></div>`), 0644)

	html := `<html>
{{# include "slides/01-title.html" #}}
</html>`

	result, err := FlattenIncludes(html, tmp)
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
	tmp := t.TempDir()
	slidesDir := filepath.Join(tmp, "slides")
	os.MkdirAll(slidesDir, 0755)

	os.WriteFile(filepath.Join(slidesDir, "01.html"), []byte(`<div>Slide 1</div>`), 0644)
	os.WriteFile(filepath.Join(slidesDir, "02.html"), []byte(`<div>Slide 2</div>`), 0644)

	html := `<html>
{{# include "slides/01.html" #}}
{{# include "slides/02.html" #}}
</html>`

	result, err := FlattenIncludes(html, tmp)
	if err != nil {
		t.Fatalf("FlattenIncludes failed: %v", err)
	}

	if !strings.Contains(result, "Slide 1") || !strings.Contains(result, "Slide 2") {
		t.Error("not all includes resolved")
	}
}

// TestBuildIncludesExportJS verifies the full build pipeline: scaffold a presentation,
// build it, and confirm the output HTML contains the inlined export JS (exportPresentation
// function) and the export button markup (class="export-btn"). This is an end-to-end test
// ensuring the export feature survives the complete init → build workflow.
func TestBuildIncludesExportJS(t *testing.T) {
	tmp := t.TempDir()

	// Scaffold a presentation
	_, err := scaffold.CreateInDir("Build Export Test", 3, "default", filepath.Join(tmp, "deck"), true)
	if err != nil {
		t.Fatalf("CreateInDir failed: %v", err)
	}

	// Build it
	result, err := Build(filepath.Join(tmp, "deck"))
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
	tmp := t.TempDir()

	html := `{{# include "nonexistent.html" #}}`
	_, err := FlattenIncludes(html, tmp)
	if err == nil {
		t.Error("expected error for missing include file")
	}
}
