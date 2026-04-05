package cmd

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/mcpkit"
	"github.com/panyam/slyds/core"
)

// scaffoldTestDeck creates a deck in a temp dir and returns the root and deck name.
func scaffoldTestDeck(t *testing.T, name, title, theme string, slides int) string {
	t.Helper()
	root := t.TempDir()
	outDir := filepath.Join(root, name)
	_, err := core.CreateInDir(title, slides, theme, outDir, true)
	if err != nil {
		t.Fatalf("CreateInDir(%s) failed: %v", name, err)
	}
	return root
}

func callTool(t *testing.T, handler mcpkit.ToolHandler, args any) mcpkit.ToolResult {
	t.Helper()
	data, _ := json.Marshal(args)
	result, err := handler(context.Background(), mcpkit.ToolRequest{
		Arguments: data,
	})
	if err != nil {
		t.Fatalf("tool handler error: %v", err)
	}
	return result
}

func toolText(result mcpkit.ToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}
	return result.Content[0].Text
}

func TestDescribeDeckTool(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Test Deck", "default", 3)
	_, handler := describeDeckTool(root)

	result := callTool(t, handler, map[string]string{"deck": "test-deck"})
	if result.IsError {
		t.Fatalf("describe_deck error: %s", toolText(result))
	}

	text := toolText(result)
	if !strings.Contains(text, "Test Deck") {
		t.Error("describe_deck missing title")
	}
	if !strings.Contains(text, `"slide_count": 3`) {
		t.Error("describe_deck missing slide count")
	}
}

func TestListSlidesTool(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Test", "default", 4)
	_, handler := listSlidesTool(root)

	result := callTool(t, handler, map[string]string{"deck": "test-deck"})
	if result.IsError {
		t.Fatalf("list_slides error: %s", toolText(result))
	}

	var slides []map[string]any
	json.Unmarshal([]byte(toolText(result)), &slides)
	if len(slides) != 4 {
		t.Errorf("expected 4 slides, got %d", len(slides))
	}
}

func TestReadSlideTool(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Read Test", "default", 3)
	_, handler := readSlideTool(root)

	result := callTool(t, handler, map[string]any{"deck": "test-deck", "position": 1})
	if result.IsError {
		t.Fatalf("read_slide error: %s", toolText(result))
	}

	text := toolText(result)
	if !strings.Contains(text, `class="slide`) {
		t.Error("read_slide missing slide class")
	}
	if !strings.Contains(text, "Read Test") {
		t.Error("read_slide missing title content")
	}
}

func TestEditSlideTool(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Edit Test", "default", 3)
	_, editHandler := editSlideTool(root)
	_, readHandler := readSlideTool(root)

	// Edit slide 2
	newContent := `<div class="slide" data-layout="content"><h1>Updated</h1></div>`
	result := callTool(t, editHandler, map[string]any{
		"deck": "test-deck", "position": 2, "content": newContent,
	})
	if result.IsError {
		t.Fatalf("edit_slide error: %s", toolText(result))
	}

	// Read it back
	result = callTool(t, readHandler, map[string]any{"deck": "test-deck", "position": 2})
	if !strings.Contains(toolText(result), "Updated") {
		t.Error("edit_slide content not persisted")
	}
}

func TestQuerySlideTool(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Query Test", "default", 3)
	_, handler := querySlideTool(root)

	result := callTool(t, handler, map[string]any{
		"deck": "test-deck", "slide": "1", "selector": "h1",
	})
	if result.IsError {
		t.Fatalf("query_slide error: %s", toolText(result))
	}

	text := toolText(result)
	if !strings.Contains(text, "Query Test") {
		t.Error("query_slide didn't find h1 text")
	}
}

func TestAddAndRemoveSlideTool(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "CRUD Test", "default", 3)
	_, addHandler := addSlideTool(root)
	_, removeHandler := removeSlideTool(root)
	_, listHandler := listSlidesTool(root)

	// Add a slide at position 2
	result := callTool(t, addHandler, map[string]any{
		"deck": "test-deck", "position": 2, "name": "new-slide", "layout": "content", "title": "New",
	})
	if result.IsError {
		t.Fatalf("add_slide error: %s", toolText(result))
	}

	// List should now have 4 slides
	result = callTool(t, listHandler, map[string]string{"deck": "test-deck"})
	var slides []map[string]any
	json.Unmarshal([]byte(toolText(result)), &slides)
	if len(slides) != 4 {
		t.Errorf("after add: expected 4 slides, got %d", len(slides))
	}

	// Remove it
	result = callTool(t, removeHandler, map[string]any{"deck": "test-deck", "slide": "2"})
	if result.IsError {
		t.Fatalf("remove_slide error: %s", toolText(result))
	}

	// List should be back to 3
	result = callTool(t, listHandler, map[string]string{"deck": "test-deck"})
	json.Unmarshal([]byte(toolText(result)), &slides)
	if len(slides) != 3 {
		t.Errorf("after remove: expected 3 slides, got %d", len(slides))
	}
}

func TestCheckDeckTool(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Check Test", "default", 3)
	_, handler := checkDeckTool(root)

	result := callTool(t, handler, map[string]string{"deck": "test-deck"})
	if result.IsError {
		t.Fatalf("check_deck error: %s", toolText(result))
	}
	// Should return valid JSON (issues array)
	text := toolText(result)
	if !strings.HasPrefix(strings.TrimSpace(text), "{") && !strings.HasPrefix(strings.TrimSpace(text), "[") {
		t.Errorf("check_deck didn't return JSON: %s", text[:50])
	}
}

func TestBuildDeckTool(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Build Test", "default", 3)
	_, handler := buildDeckTool(root)

	result := callTool(t, handler, map[string]string{"deck": "test-deck"})
	if result.IsError {
		t.Fatalf("build_deck error: %s", toolText(result))
	}

	text := toolText(result)
	if !strings.Contains(text, "<style>") {
		t.Error("build_deck missing inlined CSS")
	}
	if !strings.Contains(text, "Build Test") {
		t.Error("build_deck missing title")
	}
}

func TestCreateDeckTool(t *testing.T) {
	root := t.TempDir()
	_, handler := createDeckTool(root)

	result := callTool(t, handler, map[string]any{
		"name": "new-deck", "title": "Created Deck", "theme": "dark", "slides": 2,
	})
	if result.IsError {
		t.Fatalf("create_deck error: %s", toolText(result))
	}

	text := toolText(result)
	if !strings.Contains(text, "Created Deck") {
		t.Error("create_deck missing title in response")
	}
	if !strings.Contains(text, `"theme": "dark"`) {
		t.Error("create_deck missing theme in response")
	}

	// Verify deck exists and is readable
	d, err := core.OpenDeckDir(filepath.Join(root, "new-deck"))
	if err != nil {
		t.Fatalf("can't open created deck: %v", err)
	}
	count, _ := d.SlideCount()
	if count != 2 {
		t.Errorf("created deck has %d slides, want 2", count)
	}
}

func TestToolDeckNotFound(t *testing.T) {
	root := t.TempDir()
	_, handler := describeDeckTool(root)

	result := callTool(t, handler, map[string]string{"deck": "nonexistent"})
	if !result.IsError {
		t.Error("expected error for nonexistent deck")
	}
}
