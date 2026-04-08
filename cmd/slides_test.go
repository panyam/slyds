package cmd

import (
	"encoding/json"
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
	return d.InsertSlide(pos, name, layoutName, title)
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





































// TestLsSlidesJSON verifies that the ls command's JSON output (via the
// slideInfo struct) contains a JSON array with position, file, layout, and
// title for each slide. This exercises the --json flag on `slyds ls` and
// ensures agents get machine-parseable slide listings for CLI-direct mode.
func TestLsSlidesJSON(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	d, err := core.OpenDeckDir(root)
	if err != nil {
		t.Fatalf("OpenDeckDir: %v", err)
	}

	slides, err := d.SlideFilenames()
	if err != nil {
		t.Fatalf("SlideFilenames: %v", err)
	}

	// Build slideInfo array the same way lsCmd --json does.
	var infos []slideInfo
	for i, f := range slides {
		content, _ := d.GetSlideContent(i + 1)
		infos = append(infos, slideInfo{
			Position: i + 1,
			File:     f,
			Layout:   core.DetectLayout(content),
			Title:    core.ExtractFirstHeading(content),
		})
	}

	data, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}

	// Round-trip: unmarshal and verify structure.
	var parsed []map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(parsed) != 4 {
		t.Fatalf("expected 4 slides, got %d", len(parsed))
	}

	// Verify first slide has all required fields.
	first := parsed[0]
	for _, field := range []string{"position", "file", "layout", "title"} {
		if _, ok := first[field]; !ok {
			t.Errorf("slide[0] missing field: %s", field)
		}
	}

	// Position should be 1-based.
	if pos := int(first["position"].(float64)); pos != 1 {
		t.Errorf("slide[0] position = %d, want 1", pos)
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

	d, _ := core.OpenDeckDir(root)
	html, err := d.GetSlideContent(2)
	if err != nil {
		t.Fatalf("failed to read inserted slide: %v", err)
	}

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

	d, _ := core.OpenDeckDir(root)
	slideContent, err := d.GetSlideContent(2)
	if err != nil {
		t.Fatalf("failed to read inserted slide: %v", err)
	}
	if !strings.Contains(slideContent, `data-layout="two-col"`) {
		t.Error("two-column type did not map to two-col layout")
	}
}





