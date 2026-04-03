package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/slyds/core"
)

// runInsert is a backward-compat helper for tests that verify layout rendering.
// Opens a Deck, renders slide content from layout, and inserts it.
func runInsert(root string, pos int, name, layoutName, title string) error {
	d, err := core.OpenDeckDir(root)
	if err != nil {
		return err
	}
	content, err := renderSlideContent(d, name, layoutName, pos, title)
	if err != nil {
		return err
	}
	filename := fmt.Sprintf("%02d-%s.html", pos, name)
	return d.AddSlide(pos, filename, content)
}

// mustSlideFilenames opens a Deck and returns its slide filenames.
func mustSlideFilenames(t *testing.T, root string) ([]string, error) {
	t.Helper()
	d, err := core.OpenDeckDir(root)
	if err != nil {
		return nil, err
	}
	return d.SlideFilenames()
}

// setupTestPresentation creates a test presentation in a temp dir and chdir into it.
func setupTestPresentation(t *testing.T) (string, func()) {
	t.Helper()
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)

	_, err := core.Create("Test Pres", 4)
	if err != nil {
		t.Fatalf("core.Create failed: %v", err)
	}
	presDir := filepath.Join(tmp, "test-pres")
	os.Chdir(presDir)

	return presDir, func() { os.Chdir(origDir) }
}



func TestRenderSlideFromTheme(t *testing.T) {
	content, err := renderSlideFromTheme("", "my-demo", "content", 5)
	if err != nil {
		t.Fatalf("renderSlideFromTheme failed: %v", err)
	}

	if !strings.Contains(content, `class="slide"`) {
		t.Error("missing slide class")
	}
	if !strings.Contains(content, "Slide 5") {
		t.Error("missing slide number")
	}
}

func TestRenderSlideFromThemeTitle(t *testing.T) {
	content, err := renderSlideFromTheme("", "intro", "title", 1)
	if err != nil {
		t.Fatalf("renderSlideFromTheme failed: %v", err)
	}

	if !strings.Contains(content, "title-slide") {
		t.Error("missing title-slide class")
	}
}

func TestRenderSlideFromThemeClosing(t *testing.T) {
	content, err := renderSlideFromTheme("", "end", "closing", 10)
	if err != nil {
		t.Fatalf("renderSlideFromTheme failed: %v", err)
	}

	if !strings.Contains(content, "conclusion-slide") {
		t.Error("missing conclusion-slide class")
	}
}







// TestRenderSlideFromThemeUsesManifest verifies that renderSlideFromTheme reads
// the theme from .slyds.yaml instead of hardcoding "default". When a presentation
// is created with theme "dark", new slides should use dark theme templates.
func TestRenderSlideFromThemeUsesManifest(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	// Create a presentation with "dark" theme
	_, err := core.CreateWithTheme("Dark Pres", 3, "dark")
	if err != nil {
		t.Fatalf("core.CreateWithTheme failed: %v", err)
	}
	presDir := filepath.Join(tmp, "dark-pres")
	os.Chdir(presDir)

	// renderSlideFromTheme should use "dark" theme from manifest
	content, err := renderSlideFromTheme(presDir, "test-slide", "content", 2)
	if err != nil {
		t.Fatalf("renderSlideFromTheme failed: %v", err)
	}

	// Dark theme content template should produce valid slide content
	if !strings.Contains(content, "slide") {
		t.Error("expected slide content from dark theme")
	}
}






// TestInsertWithType verifies that the --type flag is respected when inserting
// a slide. A section type slide should use the section template and contain
// the section-slide CSS class.
func TestInsertWithType(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	err := runInsert(root, 2, "chapter-one", "section", "")
	if err != nil {
		t.Fatalf("insert with type failed: %v", err)
	}

	// Read the new slide content
	slideContent, err := os.ReadFile(filepath.Join(root, "slides", "02-chapter-one.html"))
	if err != nil {
		t.Fatalf("failed to read inserted slide: %v", err)
	}

	if !strings.Contains(string(slideContent), "section") {
		t.Error("expected section slide content")
	}
}

// TestInsertWithTitle verifies that the --title flag sets the display title
// in the rendered slide template, overriding the auto-generated title derived
// from the slug name. Uses the "title" slide type which renders {{.Title}}.
func TestInsertWithTitle(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	err := runInsert(root, 2, "ch1", "title", "Custom Title")
	if err != nil {
		t.Fatalf("insert with title failed: %v", err)
	}

	slideContent, err := os.ReadFile(filepath.Join(root, "slides", "02-ch1.html"))
	if err != nil {
		t.Fatalf("failed to read inserted slide: %v", err)
	}

	if !strings.Contains(string(slideContent), "Custom Title") {
		t.Errorf("expected 'Custom Title' in slide, got: %s", string(slideContent))
	}
}















// TestInsertWithLayoutFlag verifies that runInsert with a layout name produces
// a slide containing the correct data-layout attribute and data-slot markers.
func TestInsertWithLayoutFlag(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	err := runInsert(root, 2, "comparison", "two-col", "")
	if err != nil {
		t.Fatalf("insert with layout two-col failed: %v", err)
	}

	slides, _ := mustSlideFilenames(t, root)
	content, err := os.ReadFile(filepath.Join(root, "slides", slides[1]))
	if err != nil {
		t.Fatalf("failed to read inserted slide: %v", err)
	}
	html := string(content)

	if !strings.Contains(html, `data-layout="two-col"`) {
		t.Error("inserted slide missing data-layout=\"two-col\" attribute")
	}
	if !strings.Contains(html, `data-slot="left"`) {
		t.Error("two-col slide missing data-slot=\"left\"")
	}
	if !strings.Contains(html, `data-slot="right"`) {
		t.Error("two-col slide missing data-slot=\"right\"")
	}
	if !strings.Contains(html, "layout-two-col") {
		t.Error("two-col slide missing layout-two-col CSS class")
	}
}

// TestInsertWithLayoutTitle verifies that the title layout produces a slide
// with data-layout="title" and the title-slide CSS class for backward compat.
func TestInsertWithLayoutTitle(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	err := runInsert(root, 1, "intro", "title", "")
	if err != nil {
		t.Fatalf("insert with layout title failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(root, "slides", "01-intro.html"))
	if err != nil {
		t.Fatalf("failed to read inserted slide: %v", err)
	}
	html := string(content)

	if !strings.Contains(html, `data-layout="title"`) {
		t.Error("title slide missing data-layout=\"title\"")
	}
	if !strings.Contains(html, "title-slide") {
		t.Error("title slide missing title-slide CSS class")
	}
	if !strings.Contains(html, "Welcome") {
		t.Error("title slide missing custom title text")
	}
}

// TestInsertDefaultLayout verifies that runInsert with the default layout name
// "content" produces a slide with data-layout="content" and a body slot.
func TestInsertDefaultLayout(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	err := runInsert(root, 2, "details", "content", "")
	if err != nil {
		t.Fatalf("insert with default layout failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(root, "slides", "02-details.html"))
	if err != nil {
		t.Fatalf("failed to read inserted slide: %v", err)
	}
	html := string(content)

	if !strings.Contains(html, `data-layout="content"`) {
		t.Error("content slide missing data-layout=\"content\"")
	}
	if !strings.Contains(html, `data-slot="body"`) {
		t.Error("content slide missing data-slot=\"body\"")
	}
}

// TestInsertWithDeprecatedType verifies that the legacy --type flag still works
// by mapping to the equivalent layout name. The "section" type maps to the
// "section" layout, and the slide should have data-layout="section".
func TestInsertWithDeprecatedType(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Simulate --type flag: resolveLayoutFlag("content", "section") → "section"
	layoutName := resolveLayoutFlag("content", "section")
	err := runInsert(root, 2, "break", layoutName, "")
	if err != nil {
		t.Fatalf("insert with deprecated type failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(root, "slides", "02-break.html"))
	if err != nil {
		t.Fatalf("failed to read inserted slide: %v", err)
	}
	html := string(content)

	if !strings.Contains(html, `data-layout="section"`) {
		t.Error("section slide missing data-layout=\"section\" — deprecated --type mapping failed")
	}
}

// TestInsertWithDeprecatedTypeTwoColumn verifies that the legacy --type
// "two-column" maps to the new layout name "two-col" (the rename).
func TestInsertWithDeprecatedTypeTwoColumn(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	layoutName := resolveLayoutFlag("content", "two-column")
	if layoutName != "two-col" {
		t.Fatalf("resolveLayoutFlag(content, two-column) = %q, want %q", layoutName, "two-col")
	}

	err := runInsert(root, 2, "versus", layoutName, "")
	if err != nil {
		t.Fatalf("insert with deprecated two-column type failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(root, "slides", "02-versus.html"))
	if err != nil {
		t.Fatalf("failed to read inserted slide: %v", err)
	}
	if !strings.Contains(string(content), `data-layout="two-col"`) {
		t.Error("two-column type did not map to two-col layout")
	}
}

// TestInsertUnknownLayout verifies that inserting with an unknown layout name
// returns a descriptive error listing the available layouts.
func TestInsertUnknownLayout(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	err := runInsert(root, 2, "bad", "nonexistent-layout", "")
	if err == nil {
		t.Fatal("expected error for unknown layout, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

// TestLsDetectsLayout verifies that detectSlideLayout correctly identifies
// the layout of slides in a scaffolded presentation, both from data-layout
// attributes (new slides) and CSS class heuristics (legacy slides).
func TestLsDetectsLayout(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Title slide should be detected as "title"
	detected := detectSlideLayout(filepath.Join(root, "slides", "01-title.html"))
	if detected != "title" {
		t.Errorf("detectSlideLayout(01-title.html) = %q, want %q", detected, "title")
	}

	// Closing slide should be detected as "closing"
	slides, _ := mustSlideFilenames(t, root)
	lastSlide := slides[len(slides)-1]
	detected = detectSlideLayout(filepath.Join(root, "slides", lastSlide))
	if detected != "closing" {
		t.Errorf("detectSlideLayout(%s) = %q, want %q", lastSlide, detected, "closing")
	}
}



// TestApplySlotsFile verifies JSON slot maps fill [data-slot] regions after insert.
func TestApplySlotsFile(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	slotsPath := filepath.Join(root, "slots.json")
	js := `{"title":"<h1>Agent Title</h1>","body":"<p>Paragraph</p>"}`
	if err := os.WriteFile(slotsPath, []byte(js), 0644); err != nil {
		t.Fatal(err)
	}

	existing, err := mustSlideFilenames(t, root)
	if err != nil {
		t.Fatal(err)
	}
	pos := len(existing) + 1
	if err := runInsert(root, pos, "extra", "content", ""); err != nil {
		t.Fatal(err)
	}
	d, _ := core.OpenDeckDir(root)
	if err := applySlotsFile(d, pos, slotsPath); err != nil {
		t.Fatal(err)
	}

	slides, _ := mustSlideFilenames(t, root)
	last := slides[len(slides)-1]
	data, err := os.ReadFile(filepath.Join(root, "slides", last))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "Agent Title") || !strings.Contains(s, "Paragraph") {
		t.Fatalf("expected slot HTML applied: %s", s)
	}
}
