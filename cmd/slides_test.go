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

// TestRenameSlugs verifies that renameToSlugs reads each slide's <h1> content,
// slugifies it, and renames files + updates index.html accordingly. Slides
// whose slug already matches their current name should be left unchanged.
func TestRenameSlugs(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Scaffolded slides: 01-title.html (<h1>Test Pres</h1>),
	// 02-slide.html (<h1>Slide 2</h1>), 03-slide.html (<h1>Slide 3</h1>),
	// 04-closing.html (<h1>Thank You</h1>)
	renamed, err := renameToSlugs(root)
	if err != nil {
		t.Fatalf("renameToSlugs failed: %v", err)
	}

	if renamed != 4 {
		t.Errorf("expected 4 renames, got %d", renamed)
	}

	slides, _ := listSlidesFromIndex(root)

	// 01-title.html → 01-test-pres.html (title slide has <h1>Test Pres</h1>)
	if slides[0] != "01-test-pres.html" {
		t.Errorf("expected 01-test-pres.html, got %s", slides[0])
	}
	// 02-slide.html → 02-slide-2.html
	if slides[1] != "02-slide-2.html" {
		t.Errorf("expected 02-slide-2.html, got %s", slides[1])
	}
	// 03-slide.html → 03-slide-3.html
	if slides[2] != "03-slide-3.html" {
		t.Errorf("expected 03-slide-3.html, got %s", slides[2])
	}
	// 04-closing.html → 04-thank-you.html
	if slides[3] != "04-thank-you.html" {
		t.Errorf("expected 04-thank-you.html, got %s", slides[3])
	}

	// Verify index.html references match
	indexHTML, _ := os.ReadFile(filepath.Join(root, "index.html"))
	indexStr := string(indexHTML)
	for _, s := range slides {
		if !strings.Contains(indexStr, fmt.Sprintf(`"slides/%s"`, s)) {
			t.Errorf("index.html missing include for %s", s)
		}
	}
}

// TestRenameSlugsIdempotent verifies that running renameToSlugs twice produces
// the same result — slides already matching their h1 slug are not renamed again.
func TestRenameSlugsIdempotent(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	renameToSlugs(root)
	renamed, err := renameToSlugs(root)
	if err != nil {
		t.Fatalf("second renameToSlugs failed: %v", err)
	}
	if renamed != 0 {
		t.Errorf("expected 0 renames on second run, got %d", renamed)
	}
}

// TestRenameSlugsNoHeading verifies that slides without an <h1> tag keep
// their existing name rather than being renamed to an empty slug.
func TestRenameSlugsNoHeading(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Replace slide 2 content with no heading
	slidePath := filepath.Join(root, "slides", "02-slide.html")
	os.WriteFile(slidePath, []byte(`<div class="slide"><p>No heading here</p></div>`), 0644)

	_, err := renameToSlugs(root)
	if err != nil {
		t.Fatalf("renameToSlugs failed: %v", err)
	}

	// Slide 2 should keep its existing name part since there's no h1
	slides, _ := listSlidesFromIndex(root)
	if slides[1] != "02-slide.html" {
		t.Errorf("expected 02-slide.html (no h1 to slug from), got %s", slides[1])
	}
}

// TestRenameSlugsPreservesContent verifies that renaming does not modify
// the actual HTML content of any slide file.
func TestRenameSlugsPreservesContent(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Read original content
	origContent, _ := os.ReadFile(filepath.Join(root, "slides", "02-slide.html"))

	renameToSlugs(root)

	// Content should be identical, just in a new file
	newContent, err := os.ReadFile(filepath.Join(root, "slides", "02-slide-2.html"))
	if err != nil {
		t.Fatalf("failed to read renamed slide: %v", err)
	}
	if string(origContent) != string(newContent) {
		t.Error("slide content was modified during rename")
	}
}

// TestRenameSlugsDeduplicates verifies that when two slides have the same <h1>
// text, the rename produces unique filenames by appending a suffix.
func TestRenameSlugsDeduplicates(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Set slides 2 and 3 to have the same heading
	for _, f := range []string{"02-slide.html", "03-slide.html"} {
		path := filepath.Join(root, "slides", f)
		os.WriteFile(path, []byte(`<div class="slide"><h1>Same Title</h1></div>`), 0644)
	}

	_, err := renameToSlugs(root)
	if err != nil {
		t.Fatalf("renameToSlugs failed: %v", err)
	}

	slides, _ := listSlidesFromIndex(root)
	// Both should have "same-title" but one needs a suffix
	if slides[1] != "02-same-title.html" {
		t.Errorf("expected 02-same-title.html, got %s", slides[1])
	}
	if slides[2] != "03-same-title-2.html" {
		t.Errorf("expected 03-same-title-2.html, got %s", slides[2])
	}
}

// TestCheckCleanDeck verifies that checkDeck returns no warnings and no errors
// for a freshly scaffolded presentation where everything is in sync.
func TestCheckCleanDeck(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	result, err := checkDeck(root)
	if err != nil {
		t.Fatalf("checkDeck failed: %v", err)
	}

	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got: %v", result.Errors)
	}
	if result.SlideCount != 4 {
		t.Errorf("expected 4 slides, got %d", result.SlideCount)
	}
	if !result.InSync {
		t.Error("expected index.html to be in sync with slide files")
	}
}

// TestCheckOrphanFiles verifies that checkDeck detects slide files on disk
// that are not referenced in index.html.
func TestCheckOrphanFiles(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Add an orphan file not in index.html
	os.WriteFile(filepath.Join(root, "slides", "orphan.html"), []byte("<div>orphan</div>"), 0644)

	result, _ := checkDeck(root)
	if result.InSync {
		t.Error("expected out of sync when orphan file exists")
	}

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "orphan.html") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about orphan.html, got: %v", result.Warnings)
	}
}

// TestCheckMissingFiles verifies that checkDeck detects slides referenced in
// index.html that don't exist on disk.
func TestCheckMissingFiles(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Delete a slide file but leave index.html reference
	os.Remove(filepath.Join(root, "slides", "02-slide.html"))

	result, _ := checkDeck(root)
	if result.InSync {
		t.Error("expected out of sync when file is missing")
	}

	found := false
	for _, e := range result.Errors {
		if strings.Contains(e, "02-slide.html") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about missing 02-slide.html, got: %v", result.Errors)
	}
}

// TestCheckMissingSpeakerNotes verifies that checkDeck warns about slides
// that have no speaker-notes div or have an empty one.
func TestCheckMissingSpeakerNotes(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Replace slide 2 with content that has no speaker notes
	os.WriteFile(filepath.Join(root, "slides", "02-slide.html"),
		[]byte(`<div class="slide"><h1>No Notes</h1><p>Content</p></div>`), 0644)

	result, _ := checkDeck(root)

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "02-slide.html") && strings.Contains(w, "speaker notes") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about missing speaker notes, got: %v", result.Warnings)
	}
}

// TestCheckBrokenAssetRef verifies that checkDeck detects local asset
// references (src="...", href="...") that point to files that don't exist.
func TestCheckBrokenAssetRef(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Add a slide with a broken image reference
	os.WriteFile(filepath.Join(root, "slides", "02-slide.html"),
		[]byte(`<div class="slide"><h1>Demo</h1><img src="images/missing.png"><div class="speaker-notes"><p>notes</p></div></div>`), 0644)

	result, _ := checkDeck(root)

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "missing.png") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about missing.png, got: %v", result.Warnings)
	}
}

// TestCheckTalkTime verifies that checkDeck estimates talk time from
// speaker notes word count.
func TestCheckTalkTime(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	result, _ := checkDeck(root)

	// Scaffolded slides have some speaker notes, so time should be > 0
	if result.EstimatedMinutes <= 0 {
		t.Error("expected positive estimated talk time")
	}
}

// TestCheckRemoteAssetIgnored verifies that checkDeck does not flag
// remote URLs (http/https) as broken asset references.
func TestCheckRemoteAssetIgnored(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	os.WriteFile(filepath.Join(root, "slides", "02-slide.html"),
		[]byte(`<div class="slide"><h1>Demo</h1><img src="https://example.com/img.png"><div class="speaker-notes"><p>notes</p></div></div>`), 0644)

	result, _ := checkDeck(root)

	for _, w := range result.Warnings {
		if strings.Contains(w, "example.com") {
			t.Errorf("should not warn about remote URLs, got: %s", w)
		}
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

	content, err := os.ReadFile(filepath.Join(root, "slides", "02-comparison.html"))
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

	err := runInsert(root, 1, "intro", "title", "Welcome")
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

	err := runInsert(root, 2, "bad", "nonexistent", "")
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
	slides, _ := listSlidesFromIndex(root)
	lastSlide := slides[len(slides)-1]
	detected = detectSlideLayout(filepath.Join(root, "slides", lastSlide))
	if detected != "closing" {
		t.Errorf("detectSlideLayout(%s) = %q, want %q", lastSlide, detected, "closing")
	}
}

// TestCheckMissingDataLayout verifies that slyds check warns about slides
// that lack a data-layout attribute (legacy slides from before Phase 2).
func TestCheckMissingDataLayout(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Create a legacy slide without data-layout
	legacySlide := `<div class="slide"><h1>Legacy</h1></div>`
	os.WriteFile(filepath.Join(root, "slides", "02-slide.html"), []byte(legacySlide), 0644)

	result, err := checkDeck(root)
	if err != nil {
		t.Fatalf("checkDeck failed: %v", err)
	}

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "no data-layout") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning about missing data-layout attribute on legacy slide")
	}
}

// TestCheckUnknownLayout verifies that slyds check warns about slides with
// an unrecognized data-layout value.
func TestCheckUnknownLayout(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Create a slide with an unknown layout
	badSlide := `<div class="slide" data-layout="nonexistent"><h1>Bad</h1></div>`
	os.WriteFile(filepath.Join(root, "slides", "02-slide.html"), []byte(badSlide), 0644)

	result, err := checkDeck(root)
	if err != nil {
		t.Fatalf("checkDeck failed: %v", err)
	}

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "unknown layout") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning about unknown layout \"nonexistent\"")
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

	existing, err := listSlidesFromIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	pos := len(existing) + 1
	if err := runInsert(root, pos, "extra", "content", ""); err != nil {
		t.Fatal(err)
	}
	if err := applySlotsFile(root, pos, slotsPath); err != nil {
		t.Fatal(err)
	}

	slides, _ := listSlidesFromIndex(root)
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
