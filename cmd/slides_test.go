package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/slyds/internal/scaffold"
)

// setupTestPresentation creates a test presentation in a temp dir and chdir into it.
func setupTestPresentation(t *testing.T) (string, func()) {
	t.Helper()
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)

	_, err := scaffold.Create("Test Pres", 4)
	if err != nil {
		t.Fatalf("scaffold.Create failed: %v", err)
	}
	presDir := filepath.Join(tmp, "test-pres")
	os.Chdir(presDir)

	return presDir, func() { os.Chdir(origDir) }
}

func TestListSlideFiles(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	files := listSlideFiles(root)
	if len(files) != 4 {
		t.Errorf("expected 4 slides, got %d: %v", len(files), files)
	}

	// Should be sorted
	for i := 1; i < len(files); i++ {
		if files[i] < files[i-1] {
			t.Errorf("slides not sorted: %v", files)
			break
		}
	}
}

func TestExtractFirstHeading(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.html")

	os.WriteFile(path, []byte(`<div class="slide"><h1>My Heading</h1></div>`), 0644)
	heading := extractFirstHeading(path)
	if heading != "My Heading" {
		t.Errorf("extractFirstHeading = %q, want %q", heading, "My Heading")
	}

	// No heading
	os.WriteFile(path, []byte(`<div class="slide"><p>No heading</p></div>`), 0644)
	heading = extractFirstHeading(path)
	if heading != "" {
		t.Errorf("expected empty heading, got %q", heading)
	}
}

func TestRenderSlideFromTheme(t *testing.T) {
	content, err := renderSlideFromTheme("my-demo", "content", 5)
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
	content, err := renderSlideFromTheme("intro", "title", 1)
	if err != nil {
		t.Fatalf("renderSlideFromTheme failed: %v", err)
	}

	if !strings.Contains(content, "title-slide") {
		t.Error("missing title-slide class")
	}
}

func TestRenderSlideFromThemeClosing(t *testing.T) {
	content, err := renderSlideFromTheme("end", "closing", 10)
	if err != nil {
		t.Fatalf("renderSlideFromTheme failed: %v", err)
	}

	if !strings.Contains(content, "conclusion-slide") {
		t.Error("missing conclusion-slide class")
	}
}

func TestRewriteSlidesAndIndex(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Reverse the order
	files := listSlideFiles(root)
	reversed := make([]string, len(files))
	for i, f := range files {
		reversed[len(files)-1-i] = f
	}

	err := rewriteSlidesAndIndex(root, reversed)
	if err != nil {
		t.Fatalf("rewriteSlidesAndIndex failed: %v", err)
	}

	// Check files were renumbered
	newFiles := listSlideFiles(root)
	if len(newFiles) != 4 {
		t.Errorf("expected 4 slides after rewrite, got %d", len(newFiles))
	}

	// The closing slide should now be first
	if !strings.Contains(newFiles[0], "closing") {
		t.Errorf("expected closing slide first, got %s", newFiles[0])
	}

	// Check index.html was updated
	indexHTML, _ := os.ReadFile(filepath.Join(root, "index.html"))
	indexStr := string(indexHTML)
	for _, f := range newFiles {
		if !strings.Contains(indexStr, f) {
			t.Errorf("index.html missing include for %s", f)
		}
	}
}

func TestFindRoot(t *testing.T) {
	_, cleanup := setupTestPresentation(t)
	defer cleanup()

	found, err := findRoot()
	if err != nil {
		t.Fatalf("findRoot failed: %v", err)
	}
	// Check index.html exists at the found root (avoid symlink path comparison on macOS)
	if _, err := os.Stat(filepath.Join(found, "index.html")); os.IsNotExist(err) {
		t.Errorf("findRoot returned %q but index.html not found there", found)
	}
}

func TestFindRootNoIndex(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	_, err := findRoot()
	if err == nil {
		t.Error("expected error when no index.html exists")
	}
}
