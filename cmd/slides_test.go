package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/slyds/internal/scaffold"
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

// TestExtractNamePart verifies that extractNamePart correctly strips numeric
// prefixes from slide filenames while preserving the full name for files
// without a numeric prefix. This is critical for handling user-created files
// that don't follow the NN-name.html convention (e.g., "blah.html" or
// "my-intro.html" should not be mangled during renumbering).
func TestExtractNamePart(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"01-title.html", "title.html"},
		{"010-blah.html", "blah.html"},
		{"2-x.html", "x.html"},
		{"blah.html", "blah.html"},
		{"my-intro.html", "my-intro.html"},
		{"03-my-great-slide.html", "my-great-slide.html"},
	}
	for _, tt := range tests {
		got := extractNamePart(tt.input)
		if got != tt.want {
			t.Errorf("extractNamePart(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestListSlidesFromIndex verifies that slide ordering is derived from
// index.html include directives rather than filesystem sort. This ensures
// that manual reordering in index.html is respected and that orphan files
// on disk (not referenced in index.html) are not included in the list.
func TestListSlidesFromIndex(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	slides, err := listSlidesFromIndex(root)
	if err != nil {
		t.Fatalf("listSlidesFromIndex failed: %v", err)
	}

	// Scaffolded presentation has 4 slides
	if len(slides) != 4 {
		t.Fatalf("expected 4 slides, got %d: %v", len(slides), slides)
	}

	// First should be title, last should be closing
	if !strings.Contains(slides[0], "title") {
		t.Errorf("expected first slide to contain 'title', got %s", slides[0])
	}
	if !strings.Contains(slides[3], "closing") {
		t.Errorf("expected last slide to contain 'closing', got %s", slides[3])
	}

	// Add an orphan file not in index.html — it should NOT appear
	orphan := filepath.Join(root, "slides", "orphan.html")
	os.WriteFile(orphan, []byte("<div>orphan</div>"), 0644)

	slides, _ = listSlidesFromIndex(root)
	for _, s := range slides {
		if s == "orphan.html" {
			t.Error("orphan file should not appear in listSlidesFromIndex")
		}
	}
}

// TestRewriteWithNonPrefixedFiles verifies that rewriteSlidesAndIndex correctly
// handles files without numeric prefixes (e.g., "my-intro.html") by preserving
// the full filename as the name part and adding a numeric prefix. This catches
// the bug where strings.SplitN("-", 2) would mangle "my-intro.html" into
// "intro.html" by splitting on the first hyphen.
func TestRewriteWithNonPrefixedFiles(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	slidesDir := filepath.Join(root, "slides")

	// Rename 02-slide.html to my-intro.html (no numeric prefix, has hyphens)
	os.Rename(filepath.Join(slidesDir, "02-slide.html"), filepath.Join(slidesDir, "my-intro.html"))

	// Update index.html to reference the new name
	indexPath := filepath.Join(root, "index.html")
	indexHTML, _ := os.ReadFile(indexPath)
	newIndex := strings.Replace(string(indexHTML), "02-slide.html", "my-intro.html", 1)
	os.WriteFile(indexPath, []byte(newIndex), 0644)

	// Get ordering from index and rewrite
	slides, _ := listSlidesFromIndex(root)
	err := rewriteSlidesAndIndex(root, slides)
	if err != nil {
		t.Fatalf("rewriteSlidesAndIndex failed: %v", err)
	}

	// Verify my-intro.html was renumbered to 02-my-intro.html (NOT 02-intro.html)
	newFiles := listSlideFiles(root)
	found := false
	for _, f := range newFiles {
		if strings.Contains(f, "my-intro") {
			found = true
			if f != "02-my-intro.html" {
				t.Errorf("expected 02-my-intro.html, got %s", f)
			}
		}
	}
	if !found {
		t.Errorf("my-intro file not found after rewrite; files: %v", newFiles)
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
	_, err := scaffold.CreateWithTheme("Dark Pres", 3, "dark")
	if err != nil {
		t.Fatalf("scaffold.CreateWithTheme failed: %v", err)
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

// TestInsertAtBeginning verifies that inserting a slide at position 1 shifts all
// existing slides by +1 and places the new slide first. The new slide should be
// 01-<name>.html and all subsequent slides should be renumbered.
func TestInsertAtBeginning(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Presentation has 4 slides: 01-title, 02-slide, 03-slide, 04-closing
	err := runInsert(root, 1, "opening", "content", "")
	if err != nil {
		t.Fatalf("insert at beginning failed: %v", err)
	}

	slides, _ := listSlidesFromIndex(root)
	if len(slides) != 5 {
		t.Fatalf("expected 5 slides after insert, got %d: %v", len(slides), slides)
	}

	// New slide should be first
	if slides[0] != "01-opening.html" {
		t.Errorf("expected 01-opening.html at position 1, got %s", slides[0])
	}

	// Original title slide should now be 02
	if !strings.Contains(slides[1], "title") {
		t.Errorf("expected title slide at position 2, got %s", slides[1])
	}

	// Closing should still be last
	if !strings.Contains(slides[4], "closing") {
		t.Errorf("expected closing slide last, got %s", slides[4])
	}
}

// TestInsertAtMiddle verifies that inserting a slide at position 3 (between
// existing slides 2 and 3) correctly shifts slides from position 3 onward
// and places the new slide at the correct position in both the filesystem
// and index.html.
func TestInsertAtMiddle(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	err := runInsert(root, 3, "interlude", "content", "")
	if err != nil {
		t.Fatalf("insert at middle failed: %v", err)
	}

	slides, _ := listSlidesFromIndex(root)
	if len(slides) != 5 {
		t.Fatalf("expected 5 slides after insert, got %d: %v", len(slides), slides)
	}

	if slides[2] != "03-interlude.html" {
		t.Errorf("expected 03-interlude.html at position 3, got %s", slides[2])
	}

	// Verify index.html has correct order
	indexHTML, _ := os.ReadFile(filepath.Join(root, "index.html"))
	indexStr := string(indexHTML)
	for _, s := range slides {
		if !strings.Contains(indexStr, s) {
			t.Errorf("index.html missing include for %s", s)
		}
	}
}

// TestInsertAtEnd verifies that inserting at position len(slides)+1 appends
// the new slide after the last existing slide (including after the closing slide).
func TestInsertAtEnd(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Insert at position 5 (after all 4 existing slides)
	err := runInsert(root, 5, "bonus", "content", "")
	if err != nil {
		t.Fatalf("insert at end failed: %v", err)
	}

	slides, _ := listSlidesFromIndex(root)
	if len(slides) != 5 {
		t.Fatalf("expected 5 slides after insert, got %d: %v", len(slides), slides)
	}

	if slides[4] != "05-bonus.html" {
		t.Errorf("expected 05-bonus.html at last position, got %s", slides[4])
	}
}

// TestInsertWithNonPrefixedFiles verifies that insert works correctly when the
// presentation contains slides without standard numeric prefixes. The non-prefixed
// file should be renumbered without losing its name, and the new slide should be
// inserted at the correct position.
func TestInsertWithNonPrefixedFiles(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	slidesDir := filepath.Join(root, "slides")

	// Rename 02-slide.html to just "blah.html"
	os.Rename(filepath.Join(slidesDir, "02-slide.html"), filepath.Join(slidesDir, "blah.html"))
	indexPath := filepath.Join(root, "index.html")
	indexHTML, _ := os.ReadFile(indexPath)
	os.WriteFile(indexPath, []byte(strings.Replace(string(indexHTML), "02-slide.html", "blah.html", 1)), 0644)

	// Insert at position 2
	err := runInsert(root, 2, "new-slide", "content", "")
	if err != nil {
		t.Fatalf("insert with non-prefixed files failed: %v", err)
	}

	slides, _ := listSlidesFromIndex(root)
	if len(slides) != 5 {
		t.Fatalf("expected 5 slides, got %d: %v", len(slides), slides)
	}

	// "blah.html" should have been renumbered to 03-blah.html
	found := false
	for _, s := range slides {
		if strings.Contains(s, "blah") {
			found = true
			if s != "03-blah.html" {
				t.Errorf("expected 03-blah.html, got %s", s)
			}
		}
	}
	if !found {
		t.Errorf("blah file lost after insert; slides: %v", slides)
	}
}

// TestInsertOutOfRange verifies that inserting at position 0 or beyond len+1
// returns an error rather than silently corrupting the slide ordering.
func TestInsertOutOfRange(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Position 0 is invalid (1-based)
	err := runInsert(root, 0, "bad", "content", "")
	if err == nil {
		t.Error("expected error for position 0")
	}

	// Position 6 is out of range (only 4 slides, max insert at 5)
	err = runInsert(root, 6, "bad", "content", "")
	if err == nil {
		t.Error("expected error for position > len+1")
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

	err := runInsert(root, 2, "ch1", "title", "Chapter One: The Beginning")
	if err != nil {
		t.Fatalf("insert with title failed: %v", err)
	}

	slideContent, err := os.ReadFile(filepath.Join(root, "slides", "02-ch1.html"))
	if err != nil {
		t.Fatalf("failed to read inserted slide: %v", err)
	}

	if !strings.Contains(string(slideContent), "Chapter One: The Beginning") {
		t.Errorf("expected custom title in slide, got: %s", string(slideContent))
	}
}

// TestInsertPreservesSlideContent verifies that inserting a slide does not
// corrupt or lose the content of existing slides during renumbering.
func TestInsertPreservesSlideContent(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Read original content of slide 2
	origContent, _ := os.ReadFile(filepath.Join(root, "slides", "02-slide.html"))

	// Insert at position 2, pushing original slide 2 to position 3
	err := runInsert(root, 2, "new", "content", "")
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	// Original slide 2 is now slide 3
	newContent, err := os.ReadFile(filepath.Join(root, "slides", "03-slide.html"))
	if err != nil {
		t.Fatalf("failed to read renumbered slide: %v", err)
	}

	if string(origContent) != string(newContent) {
		t.Error("slide content was modified during renumbering")
	}
}

// TestMultipleInserts verifies that multiple consecutive inserts at different
// positions produce the correct final ordering with no file conflicts or
// numbering gaps.
func TestMultipleInserts(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Start: 4 slides
	// Insert at position 1, then at position 3, then at end
	if err := runInsert(root, 1, "first", "content", ""); err != nil {
		t.Fatalf("first insert failed: %v", err)
	}
	if err := runInsert(root, 3, "middle", "content", ""); err != nil {
		t.Fatalf("second insert failed: %v", err)
	}
	if err := runInsert(root, 7, "last", "content", ""); err != nil {
		t.Fatalf("third insert failed: %v", err)
	}

	slides, _ := listSlidesFromIndex(root)
	if len(slides) != 7 {
		t.Fatalf("expected 7 slides after 3 inserts, got %d: %v", len(slides), slides)
	}

	// Verify sequential numbering with no gaps
	for i, s := range slides {
		expected := fmt.Sprintf("%02d-", i+1)
		if !strings.HasPrefix(s, expected) {
			t.Errorf("slide %d has wrong prefix: %s (expected %s...)", i+1, s, expected)
		}
	}
}
