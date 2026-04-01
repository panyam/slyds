package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestQueryReadH1 verifies that querying "h1" returns the text content of
// the first heading in a slide. Uses a scaffolded presentation where
// slide 1 has <h1>Test Pres</h1>.
func TestQueryReadH1(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	result, err := runQuery(root, "1", "h1", QueryOpts{})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(result) != 1 || strings.TrimSpace(result[0]) != "Test Pres" {
		t.Errorf("expected [\"Test Pres\"], got %v", result)
	}
}

// TestQueryReadHTML verifies that --html returns the inner HTML of matched
// elements rather than just text content.
func TestQueryReadHTML(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	result, err := runQuery(root, "1", ".speaker-notes", QueryOpts{HTML: true})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}
	// Speaker notes contain nested HTML (h3, p tags)
	if !strings.Contains(result[0], "<h3>") || !strings.Contains(result[0], "<p>") {
		t.Errorf("expected HTML with nested tags, got: %s", result[0])
	}
}

// TestQueryReadAttr verifies that --attr returns the value of a specific
// attribute on matched elements.
func TestQueryReadAttr(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	result, err := runQuery(root, "1", ".slide", QueryOpts{Attr: "class"})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result[0], "slide") {
		t.Errorf("expected class containing 'slide', got: %s", result[0])
	}
}

// TestQueryReadMultiple verifies that when a selector matches multiple nodes,
// all matches are returned (one per entry in the result slice).
func TestQueryReadMultiple(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Slide 2 has <li> elements
	result, err := runQuery(root, "2", "li", QueryOpts{})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(result) < 2 {
		t.Errorf("expected multiple li matches, got %d: %v", len(result), result)
	}
}

// TestQueryReadCount verifies that --count returns the number of matching
// elements rather than their content.
func TestQueryReadCount(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	result, err := runQuery(root, "2", "li", QueryOpts{Count: true})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(result) != 1 || result[0] != "2" {
		t.Errorf("expected [\"2\"], got %v", result)
	}
}

// TestQueryReadNoMatch verifies that a selector matching nothing returns
// an empty result without error.
func TestQueryReadNoMatch(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	result, err := runQuery(root, "1", "table", QueryOpts{})
	if err != nil {
		t.Fatalf("expected no error on no match, got: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

// TestQuerySetText verifies that --set changes the text content of the first
// matched element and writes the result back to the file.
func TestQuerySetText(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	_, err := runQuery(root, "1", "h1", QueryOpts{Set: strPtr("New Title")})
	if err != nil {
		t.Fatalf("set failed: %v", err)
	}

	// Read back
	result, _ := runQuery(root, "1", "h1", QueryOpts{})
	if len(result) != 1 || strings.TrimSpace(result[0]) != "New Title" {
		t.Errorf("expected [\"New Title\"], got %v", result)
	}
}

// TestQuerySetHTML verifies that --set-html replaces the inner HTML of the
// first matched element, allowing insertion of rich content.
func TestQuerySetHTML(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	_, err := runQuery(root, "2", ".speaker-notes", QueryOpts{SetHTML: strPtr("<p>Custom notes</p>")})
	if err != nil {
		t.Fatalf("set-html failed: %v", err)
	}

	result, _ := runQuery(root, "2", ".speaker-notes", QueryOpts{HTML: true})
	if len(result) == 0 || !strings.Contains(result[0], "Custom notes") {
		t.Errorf("expected custom notes in HTML, got: %v", result)
	}
}

// TestQueryAppend verifies that --append adds a child element to the first
// matched node without disturbing existing content.
func TestQueryAppend(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	_, err := runQuery(root, "2", ".slide", QueryOpts{Append: strPtr(`<img src="demo.png">`)})
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	// Verify the img was added
	result, _ := runQuery(root, "2", "img", QueryOpts{Attr: "src"})
	if len(result) != 1 || result[0] != "demo.png" {
		t.Errorf("expected [\"demo.png\"], got %v", result)
	}

	// Verify existing content preserved
	result, _ = runQuery(root, "2", "h1", QueryOpts{})
	if len(result) == 0 {
		t.Error("existing h1 was lost after append")
	}
}

// TestQuerySetAttr verifies that --set-attr modifies an attribute on the
// first matched element.
func TestQuerySetAttr(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	_, err := runQuery(root, "1", ".slide", QueryOpts{SetAttr: strPtr("id=intro")})
	if err != nil {
		t.Fatalf("set-attr failed: %v", err)
	}

	result, _ := runQuery(root, "1", ".slide", QueryOpts{Attr: "id"})
	if len(result) != 1 || result[0] != "intro" {
		t.Errorf("expected [\"intro\"], got %v", result)
	}
}

// TestQueryRemove verifies that --remove deletes matched elements from the
// slide HTML.
func TestQueryRemove(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	_, err := runQuery(root, "2", ".speaker-notes", QueryOpts{Remove: true})
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	result, _ := runQuery(root, "2", ".speaker-notes", QueryOpts{})
	if len(result) != 0 {
		t.Errorf("expected speaker-notes removed, still found: %v", result)
	}
}

// TestQueryWriteFirstOnly verifies that write operations without --all only
// affect the first matched element when multiple nodes match.
func TestQueryWriteFirstOnly(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	// Slide 2 has 2 li elements
	_, err := runQuery(root, "2", "li", QueryOpts{Set: strPtr("CHANGED")})
	if err != nil {
		t.Fatalf("set failed: %v", err)
	}

	result, _ := runQuery(root, "2", "li", QueryOpts{})
	if len(result) < 2 {
		t.Fatalf("expected 2 li elements, got %d", len(result))
	}
	if strings.TrimSpace(result[0]) != "CHANGED" {
		t.Errorf("expected first li to be CHANGED, got: %s", result[0])
	}
	if strings.TrimSpace(result[1]) == "CHANGED" {
		t.Error("second li should NOT be changed without --all")
	}
}

// TestQueryWriteAll verifies that --all applies write operations to every
// matched element, not just the first.
func TestQueryWriteAll(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	_, err := runQuery(root, "2", "li", QueryOpts{Set: strPtr("ALL CHANGED"), All: true})
	if err != nil {
		t.Fatalf("set --all failed: %v", err)
	}

	result, _ := runQuery(root, "2", "li", QueryOpts{})
	for i, r := range result {
		if strings.TrimSpace(r) != "ALL CHANGED" {
			t.Errorf("li[%d] expected ALL CHANGED, got: %s", i, r)
		}
	}
}

// TestQueryWriteNoMatch verifies that write operations return an error when
// the selector matches no elements — preventing silent failures.
func TestQueryWriteNoMatch(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	_, err := runQuery(root, "1", "table", QueryOpts{Set: strPtr("x")})
	if err == nil {
		t.Error("expected error when writing to non-matching selector")
	}
}

// TestQueryByName verifies that slides can be addressed by name substring
// rather than position number.
func TestQueryByName(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	result, err := runQuery(root, "closing", "h1", QueryOpts{})
	if err != nil {
		t.Fatalf("query by name failed: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("expected result for closing slide")
	}
	if !strings.Contains(result[0], "Thank You") {
		t.Errorf("expected 'Thank You' from closing slide, got: %s", result[0])
	}
}

// TestQueryPreservesFormatting verifies that a write roundtrip through the
// DOM parser does not mangle unrelated parts of the slide HTML. This is
// critical because goquery normalizes HTML — we must avoid adding <html>,
// <head>, <body> wrappers or altering whitespace/structure outside the
// targeted element.
func TestQueryPreservesFormatting(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	slidePath := filepath.Join(root, "slides", "02-slide.html")
	origContent, _ := os.ReadFile(slidePath)

	// Set h1 then read back the full file
	_, err := runQuery(root, "2", "h1", QueryOpts{Set: strPtr("Modified")})
	if err != nil {
		t.Fatalf("set failed: %v", err)
	}

	newContent, _ := os.ReadFile(slidePath)

	// File should still be an HTML fragment (no <html>, <head>, <body>)
	if strings.Contains(string(newContent), "<html>") {
		t.Error("goquery added <html> wrapper — fragment roundtrip broken")
	}
	if strings.Contains(string(newContent), "<head>") {
		t.Error("goquery added <head> wrapper — fragment roundtrip broken")
	}
	if strings.Contains(string(newContent), "<body>") {
		t.Error("goquery added <body> wrapper — fragment roundtrip broken")
	}

	// The slide class div should still be present
	if !strings.Contains(string(newContent), `class="slide"`) {
		t.Error("slide class div was lost during roundtrip")
	}

	// Speaker notes should still be present
	if !strings.Contains(string(newContent), "speaker-notes") {
		t.Error("speaker-notes were lost during roundtrip")
	}

	// Only the h1 should have changed
	if !strings.Contains(string(newContent), "Modified") {
		t.Error("h1 was not modified")
	}

	// The original h1 text should be gone
	if strings.Contains(string(newContent), string(origContent)) {
		t.Error("file was not modified at all")
	}
}

// strPtr is a helper to create a *string for optional flag values in tests.
func strPtr(s string) *string { return &s }

func TestBatchQueryAtomicMultiSlide(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	batch := BatchFile{
		Operations: []BatchOperation{
			{Slide: "1", Selector: "h1", Op: "set", Value: "BatchTitle"},
			{Slide: "2", Selector: "h1", Op: "set", Value: "SecondSlide"},
		},
	}
	data, err := json.Marshal(batch)
	if err != nil {
		t.Fatal(err)
	}
	if err := runBatchQuery(root, data, true, false); err != nil {
		t.Fatal(err)
	}
	r1, _ := runQuery(root, "1", "h1", QueryOpts{})
	if len(r1) != 1 || strings.TrimSpace(r1[0]) != "BatchTitle" {
		t.Errorf("slide1 h1: %v", r1)
	}
	r2, _ := runQuery(root, "2", "h1", QueryOpts{})
	if len(r2) != 1 || strings.TrimSpace(r2[0]) != "SecondSlide" {
		t.Errorf("slide2 h1: %v", r2)
	}
}

func TestBatchQueryAtomicRollback(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	before, _ := os.ReadFile(filepath.Join(root, "slides", "01-title.html"))

	batch := BatchFile{
		Operations: []BatchOperation{
			{Slide: "1", Selector: "h1", Op: "set", Value: "OK"},
			{Slide: "2", Selector: "nonexistent-zq", Op: "set", Value: "bad"},
		},
	}
	data, _ := json.Marshal(batch)
	if err := runBatchQuery(root, data, true, false); err == nil {
		t.Fatal("expected error from bad selector")
	}
	after, _ := os.ReadFile(filepath.Join(root, "slides", "01-title.html"))
	if string(after) != string(before) {
		t.Error("atomic batch should not modify any file on failure")
	}
}
