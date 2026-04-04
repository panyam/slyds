package core

import (
	"strings"
	"testing"
)

// TestListLayouts verifies that all expected layout names are returned
// from the embedded layouts.yaml registry.
func TestListLayouts(t *testing.T) {
	names, err := ListLayouts()
	if err != nil {
		t.Fatalf("ListLayouts() failed: %v", err)
	}

	expected := []string{"blank", "closing", "content", "section", "title", "two-col"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d layouts, got %d: %v", len(expected), len(names), names)
	}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("expected layout %d to be %q, got %q", i, name, names[i])
		}
	}
}

// TestLayoutExists verifies that LayoutExists returns true for known layouts
// and false for unknown layout names.
func TestLayoutExists(t *testing.T) {
	for _, name := range []string{"title", "content", "two-col", "section", "blank", "closing"} {
		if !LayoutExists(name) {
			t.Errorf("LayoutExists(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"nonexistent", "two-column", ""} {
		if LayoutExists(name) {
			t.Errorf("LayoutExists(%q) = true, want false", name)
		}
	}
}

// TestRenderLayoutContent verifies that the content layout produces HTML with
// the correct data-layout attribute and a data-slot="body" container.
func TestRenderLayoutContent(t *testing.T) {
	html, err := Render("content", map[string]any{"Title": "Test Slide", "Number": 2})
	if err != nil {
		t.Fatalf("Render(content) failed: %v", err)
	}
	if !strings.Contains(html, `data-layout="content"`) {
		t.Error("content layout missing data-layout attribute")
	}
	if !strings.Contains(html, `data-slot="body"`) {
		t.Error("content layout missing data-slot=\"body\"")
	}
	if !strings.Contains(html, "Test Slide") {
		t.Error("content layout missing rendered title")
	}
}

// TestRenderLayoutTwoCol verifies that the two-col layout produces HTML with
// the correct data-layout attribute and left/right data-slot containers.
func TestRenderLayoutTwoCol(t *testing.T) {
	html, err := Render("two-col", map[string]any{"Title": "Comparison", "Number": 3})
	if err != nil {
		t.Fatalf("Render(two-col) failed: %v", err)
	}
	if !strings.Contains(html, `data-layout="two-col"`) {
		t.Error("two-col layout missing data-layout attribute")
	}
	if !strings.Contains(html, `data-slot="left"`) {
		t.Error("two-col layout missing data-slot=\"left\"")
	}
	if !strings.Contains(html, `data-slot="right"`) {
		t.Error("two-col layout missing data-slot=\"right\"")
	}
	if !strings.Contains(html, "layout-two-col") {
		t.Error("two-col layout missing layout-two-col CSS class")
	}
}

// TestRenderLayoutTitle verifies that the title layout produces HTML with
// the correct data-layout attribute and title-slide CSS class.
func TestRenderLayoutTitle(t *testing.T) {
	html, err := Render("title", map[string]any{"Title": "My Presentation", "Number": 1})
	if err != nil {
		t.Fatalf("Render(title) failed: %v", err)
	}
	if !strings.Contains(html, `data-layout="title"`) {
		t.Error("title layout missing data-layout attribute")
	}
	if !strings.Contains(html, "title-slide") {
		t.Error("title layout missing title-slide CSS class for backward compat")
	}
	if !strings.Contains(html, `data-slot="title"`) {
		t.Error("title layout missing data-slot=\"title\"")
	}
}

// TestRenderLayoutBlank verifies that the blank layout produces minimal HTML
// with just a data-layout attribute and a body slot.
func TestRenderLayoutBlank(t *testing.T) {
	html, err := Render("blank", map[string]any{"Title": "", "Number": 5})
	if err != nil {
		t.Fatalf("Render(blank) failed: %v", err)
	}
	if !strings.Contains(html, `data-layout="blank"`) {
		t.Error("blank layout missing data-layout attribute")
	}
	if !strings.Contains(html, `data-slot="body"`) {
		t.Error("blank layout missing data-slot=\"body\"")
	}
}

// TestRenderLayoutClosing verifies that the closing layout produces HTML with
// the correct data-layout attribute and conclusion-slide class for backward compat.
func TestRenderLayoutClosing(t *testing.T) {
	html, err := Render("closing", map[string]any{"Title": "Thanks", "Number": 10})
	if err != nil {
		t.Fatalf("Render(closing) failed: %v", err)
	}
	if !strings.Contains(html, `data-layout="closing"`) {
		t.Error("closing layout missing data-layout attribute")
	}
	if !strings.Contains(html, "conclusion-slide") {
		t.Error("closing layout missing conclusion-slide CSS class for backward compat")
	}
}

// TestRenderUnknownLayout verifies that rendering an unknown layout returns
// a descriptive error listing available layouts.
func TestRenderUnknownLayout(t *testing.T) {
	_, err := Render("nonexistent", map[string]any{"Title": "X", "Number": 1})
	if err == nil {
		t.Fatal("expected error for unknown layout, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "content") {
		t.Errorf("expected available layouts listed in error, got: %v", err)
	}
}

// TestDetectLayout verifies that DetectLayout correctly parses the data-layout
// attribute from slide HTML.
func TestDetectLayout(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{"data-layout attribute", `<div class="slide" data-layout="two-col">`, "two-col"},
		{"data-layout title", `<div class="slide title-slide" data-layout="title">`, "title"},
		{"legacy title-slide class", `<div class="slide active title-slide">`, "title"},
		{"legacy layout-two-column class", `<div class="slide layout-two-column">`, "two-col"},
		{"legacy section-slide class", `<div class="slide section-slide">`, "section"},
		{"legacy conclusion-slide class", `<div class="slide conclusion-slide">`, "closing"},
		{"no layout defaults to content", `<div class="slide">`, "content"},
		{"empty HTML defaults to content", ``, "content"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectLayout(tt.html)
			if got != tt.expected {
				t.Errorf("DetectLayout(%q) = %q, want %q", tt.html, got, tt.expected)
			}
		})
	}
}

// TestResolveType verifies that legacy --type values are correctly mapped
// to layout names, including the two-column → two-col rename.
func TestResolveType(t *testing.T) {
	tests := []struct {
		typeName string
		layout   string
		ok       bool
	}{
		{"title", "title", true},
		{"content", "content", true},
		{"closing", "closing", true},
		{"two-column", "two-col", true},
		{"section", "section", true},
		{"two-col", "two-col", true},   // layout name directly
		{"blank", "blank", true},       // layout name directly
		{"nonexistent", "nonexistent", false},
	}
	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			layout, ok := ResolveType(tt.typeName)
			if layout != tt.layout || ok != tt.ok {
				t.Errorf("ResolveType(%q) = (%q, %v), want (%q, %v)", tt.typeName, layout, ok, tt.layout, tt.ok)
			}
		})
	}
}

// TestLayoutRegistryHasSlots verifies that every layout in the registry
// declares at least one named slot.
func TestLayoutRegistryHasSlots(t *testing.T) {
	reg, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry() failed: %v", err)
	}
	for name, entry := range reg.Layouts {
		if len(entry.Slots) == 0 {
			t.Errorf("layout %q has no slots defined", name)
		}
		if entry.Template == "" {
			t.Errorf("layout %q has no template defined", name)
		}
	}
}
