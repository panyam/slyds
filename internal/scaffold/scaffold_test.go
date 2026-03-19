package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"My Talk", "my-talk"},
		{"Hello World 2024", "hello-world-2024"},
		{"  Spaces  Everywhere  ", "spaces-everywhere"},
		{"Special!@#Characters$%^", "special-characters"},
		{"already-slugged", "already-slugged"},
		{"UPPERCASE", "uppercase"},
		{"a", "a"},
	}
	for _, tt := range tests {
		got := Slugify(tt.input)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCreate(t *testing.T) {
	// Work in a temp directory
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	slug, err := Create("Test Talk", 3)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if slug != "test-talk" {
		t.Errorf("slug = %q, want %q", slug, "test-talk")
	}

	dir := filepath.Join(tmp, "test-talk")

	// Check required files exist
	requiredFiles := []string{
		"index.html",
		"slyds.css",
		"slyds.js",
		"theme.css",
		"slides/01-title.html",
		"slides/02-slide.html",
		"slides/03-closing.html",
	}
	for _, f := range requiredFiles {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("missing file: %s", f)
		}
	}

	// Check index.html has templar include directives
	indexHTML, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("failed to read index.html: %v", err)
	}
	indexStr := string(indexHTML)

	if !strings.Contains(indexStr, `{{# include "slides/01-title.html" #}}`) {
		t.Error("index.html missing include for 01-title.html")
	}
	if !strings.Contains(indexStr, `{{# include "slides/03-closing.html" #}}`) {
		t.Error("index.html missing include for 03-closing.html")
	}
	if !strings.Contains(indexStr, "<title>Test Talk</title>") {
		t.Error("index.html missing title")
	}

	// Check slide content
	titleSlide, _ := os.ReadFile(filepath.Join(dir, "slides", "01-title.html"))
	if !strings.Contains(string(titleSlide), "Test Talk") {
		t.Error("title slide missing presentation title")
	}
	if !strings.Contains(string(titleSlide), `class="slide`) {
		t.Error("title slide missing slide class")
	}
}

func TestCreateMinSlides(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	slug, err := Create("Min Slides", 2)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	dir := filepath.Join(tmp, slug)

	// Should have exactly title + closing
	slides, err := os.ReadDir(filepath.Join(dir, "slides"))
	if err != nil {
		t.Fatalf("failed to read slides dir: %v", err)
	}
	if len(slides) != 2 {
		t.Errorf("expected 2 slides, got %d", len(slides))
	}
}

func TestCreateMoreSlides(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	_, err := Create("Many Slides", 6)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	dir := filepath.Join(tmp, "many-slides")
	slides, err := os.ReadDir(filepath.Join(dir, "slides"))
	if err != nil {
		t.Fatalf("failed to read slides dir: %v", err)
	}
	if len(slides) != 6 {
		t.Errorf("expected 6 slides, got %d", len(slides))
	}
}

func TestCreateDuplicateDir(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	_, err := Create("Dup Test", 3)
	if err != nil {
		t.Fatalf("first Create failed: %v", err)
	}

	_, err = Create("Dup Test", 3)
	if err == nil {
		t.Error("expected error for duplicate directory, got nil")
	}
}
