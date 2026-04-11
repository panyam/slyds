package cmd

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/slyds/core"
)

// scaffoldTestDeck creates a deck in a temp dir and returns the root path.
// The deck is created at root/<name>/ using core.CreateInDir.
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

// workspaceCtx returns a context with a LocalWorkspace rooted at the given
// path pre-installed. Used by unit tests that invoke tool handlers directly
// without going through the full middleware chain.
func workspaceCtx(t *testing.T, root string) context.Context {
	t.Helper()
	ws, err := NewLocalWorkspace(root)
	if err != nil {
		t.Fatalf("NewLocalWorkspace: %v", err)
	}
	return withWorkspace(context.Background(), ws)
}

// callTool invokes a tool handler directly with JSON-marshalled arguments
// and a context carrying a LocalWorkspace rooted at the given path. Fails
// the test on handler errors.
func callTool(t *testing.T, root string, handler mcpcore.ToolHandler, args any) mcpcore.ToolResult {
	t.Helper()
	data, _ := json.Marshal(args)
	result, err := handler(workspaceCtx(t, root), mcpcore.ToolRequest{
		Arguments: data,
	})
	if err != nil {
		t.Fatalf("tool handler error: %v", err)
	}
	return result
}

// toolText extracts the first text content from a ToolResult.
func toolText(result mcpcore.ToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}
	return result.Content[0].Text
}

// TestDescribeDeckTool verifies that describe_deck returns structured JSON
// with the deck title, slide count, and slide metadata.
func TestDescribeDeckTool(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Test Deck", "default", 3)
	tool := describeDeckTool()

	result := callTool(t, root, tool.Handler, map[string]string{"deck": "test-deck"})
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

// TestListSlidesTool verifies that list_slides returns a JSON array with
// the correct number of slides and their metadata.
func TestListSlidesTool(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Test", "default", 4)
	tool := listSlidesTool()

	result := callTool(t, root, tool.Handler, map[string]string{"deck": "test-deck"})
	if result.IsError {
		t.Fatalf("list_slides error: %s", toolText(result))
	}

	var slides []map[string]any
	json.Unmarshal([]byte(toolText(result)), &slides)
	if len(slides) != 4 {
		t.Errorf("expected 4 slides, got %d", len(slides))
	}
}

// TestReadSlideTool verifies that read_slide returns the raw HTML content
// of a slide at a given position, including the slide class and title.
func TestReadSlideTool(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Read Test", "default", 3)
	tool := readSlideTool()

	result := callTool(t, root, tool.Handler, map[string]any{"deck": "test-deck", "position": 1})
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

// TestEditSlideTool verifies that edit_slide replaces a slide's HTML content
// and that the change persists when read back via read_slide.
func TestEditSlideTool(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Edit Test", "default", 3)
	editTool := editSlideTool()
	readTool := readSlideTool()

	newContent := `<div class="slide" data-layout="content"><h1>Updated</h1></div>`
	result := callTool(t, root, editTool.Handler, map[string]any{
		"deck": "test-deck", "position": 2, "content": newContent,
	})
	if result.IsError {
		t.Fatalf("edit_slide error: %s", toolText(result))
	}

	result = callTool(t, root, readTool.Handler, map[string]any{"deck": "test-deck", "position": 2})
	if !strings.Contains(toolText(result), "Updated") {
		t.Error("edit_slide content not persisted")
	}
}

// TestQuerySlideTool verifies that query_slide uses CSS selectors to extract
// content from slides — here, reading the h1 text from the title slide.
func TestQuerySlideTool(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Query Test", "default", 3)
	tool := querySlideTool()

	result := callTool(t, root, tool.Handler, map[string]any{
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

// TestAddAndRemoveSlideTool verifies the full add/remove cycle: inserts a
// slide at position 2, confirms the count increased, removes it, and
// confirms the count returned to the original.
func TestAddAndRemoveSlideTool(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "CRUD Test", "default", 3)
	addTool := addSlideTool()
	removeTool := removeSlideTool()
	listTool := listSlidesTool()

	result := callTool(t, root, addTool.Handler, map[string]any{
		"deck": "test-deck", "position": 2, "name": "new-slide", "layout": "content", "title": "New",
	})
	if result.IsError {
		t.Fatalf("add_slide error: %s", toolText(result))
	}

	result = callTool(t, root, listTool.Handler, map[string]string{"deck": "test-deck"})
	var slides []map[string]any
	json.Unmarshal([]byte(toolText(result)), &slides)
	if len(slides) != 4 {
		t.Errorf("after add: expected 4 slides, got %d", len(slides))
	}

	result = callTool(t, root, removeTool.Handler, map[string]any{"deck": "test-deck", "slide": "2"})
	if result.IsError {
		t.Fatalf("remove_slide error: %s", toolText(result))
	}

	result = callTool(t, root, listTool.Handler, map[string]string{"deck": "test-deck"})
	json.Unmarshal([]byte(toolText(result)), &slides)
	if len(slides) != 3 {
		t.Errorf("after remove: expected 3 slides, got %d", len(slides))
	}
}

// TestCheckDeckTool verifies that check_deck returns valid JSON with the
// InSync field indicating deck validation status.
func TestCheckDeckTool(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Check Test", "default", 3)
	tool := checkDeckTool()

	result := callTool(t, root, tool.Handler, map[string]string{"deck": "test-deck"})
	if result.IsError {
		t.Fatalf("check_deck error: %s", toolText(result))
	}
	text := toolText(result)
	if !strings.HasPrefix(strings.TrimSpace(text), "{") && !strings.HasPrefix(strings.TrimSpace(text), "[") {
		t.Errorf("check_deck didn't return JSON: %s", text[:50])
	}
}

// TestBuildDeckTool verifies that build_deck produces a self-contained HTML
// file with inlined CSS and the presentation title.
func TestBuildDeckTool(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Build Test", "default", 3)
	tool := buildDeckTool()

	result := callTool(t, root, tool.Handler, map[string]string{"deck": "test-deck"})
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

// TestCreateDeckTool verifies that create_deck scaffolds a new deck in the
// workspace, returns its metadata, and the deck is readable via the Deck API.
func TestCreateDeckTool(t *testing.T) {
	root := t.TempDir()
	tool := createDeckTool()

	result := callTool(t, root, tool.Handler, map[string]any{
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

	d, err := core.OpenDeckDir(filepath.Join(root, "new-deck"))
	if err != nil {
		t.Fatalf("can't open created deck: %v", err)
	}
	count, _ := d.SlideCount()
	if count != 2 {
		t.Errorf("created deck has %d slides, want 2", count)
	}
}

// TestToolDeckNotFound verifies that tools return isError:true when the
// specified deck directory doesn't exist.
func TestToolDeckNotFound(t *testing.T) {
	root := t.TempDir()
	tool := describeDeckTool()

	result := callTool(t, root, tool.Handler, map[string]string{"deck": "nonexistent"})
	if !result.IsError {
		t.Error("expected error for nonexistent deck")
	}
}

// TestReadSlideTool_BySlideParam verifies that read_slide accepts a
// slug-based reference via the new `slide` string parameter in addition
// to the legacy `position` int. Uses a scaffolded 3-slide deck (slugs:
// title, slide, closing) and reads slide 2 by slug.
func TestReadSlideTool_BySlideParam(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Slide Test", "default", 3)
	tool := readSlideTool()

	result := callTool(t, root, tool.Handler, map[string]any{
		"deck":  "test-deck",
		"slide": "slide", // slug of the middle content slide
	})
	if result.IsError {
		t.Fatalf("read_slide by slide: %s", toolText(result))
	}
	if !strings.Contains(toolText(result), `data-layout="content"`) {
		t.Error("read_slide by slug didn't return the content slide")
	}
}

// TestEditSlideTool_BySlideParam verifies that edit_slide accepts a
// slug-based reference and lands the mutation on the right slide.
func TestEditSlideTool_BySlideParam(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Edit by Slug", "default", 3)
	editTool := editSlideTool()
	readTool := readSlideTool()

	newContent := `<div class="slide" data-layout="content"><h1>Edited by Slug</h1></div>`
	result := callTool(t, root, editTool.Handler, map[string]any{
		"deck":    "test-deck",
		"slide":   "slide",
		"content": newContent,
	})
	if result.IsError {
		t.Fatalf("edit_slide by slide: %s", toolText(result))
	}

	result = callTool(t, root, readTool.Handler, map[string]any{
		"deck":     "test-deck",
		"position": 2,
	})
	if !strings.Contains(toolText(result), "Edited by Slug") {
		t.Error("edit via slide ref didn't persist")
	}
}

// TestEditSlideTool_RequiresPositionOrSlide verifies that edit_slide
// returns a clear error when the caller supplies neither `slide` nor
// `position`. Guards against agents accidentally sending empty refs.
func TestEditSlideTool_RequiresPositionOrSlide(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Req Test", "default", 3)
	tool := editSlideTool()

	result := callTool(t, root, tool.Handler, map[string]any{
		"deck":    "test-deck",
		"content": "<div>x</div>",
	})
	if !result.IsError {
		t.Error("expected error when neither slide nor position provided")
	}
	if !strings.Contains(toolText(result), "required") {
		t.Errorf("error should say 'required'; got: %s", toolText(result))
	}
}

// TestAddSlideTool_SlugCollision verifies that add_slide auto-suffixes a
// colliding slug and surfaces the actual slug used in the response text.
// Two consecutive inserts with name="intro" produce intro and intro-2;
// the second call's response must mention the auto-suffix.
func TestAddSlideTool_SlugCollision(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Collision", "default", 3)
	addTool := addSlideTool()

	// First insert: slug "intro" is free.
	r1 := callTool(t, root, addTool.Handler, map[string]any{
		"deck": "test-deck", "position": 2, "name": "intro", "layout": "content",
	})
	if r1.IsError {
		t.Fatalf("first add: %s", toolText(r1))
	}

	// Second insert: slug "intro" now collides, should become intro-2.
	r2 := callTool(t, root, addTool.Handler, map[string]any{
		"deck": "test-deck", "position": 3, "name": "intro", "layout": "content",
	})
	if r2.IsError {
		t.Fatalf("second add: %s", toolText(r2))
	}
	msg := toolText(r2)
	if !strings.Contains(msg, "intro-2") {
		t.Errorf("second add response should mention intro-2; got: %s", msg)
	}
	if !strings.Contains(msg, "auto-suffixed") {
		t.Errorf("second add response should mention auto-suffix; got: %s", msg)
	}
}

// TestMCPTools_NoWorkspaceReturnsError verifies that every tool handler
// returns a clean error (instead of panicking) when invoked with a context
// that has no workspace installed. This prevents silent nil-pointer bugs
// if the middleware is accidentally removed from the server wiring.
func TestMCPTools_NoWorkspaceReturnsError(t *testing.T) {
	tools := map[string]mcpcore.ToolHandler{
		"list_decks":    listDecksTool().Handler,
		"describe_deck": describeDeckTool().Handler,
		"list_slides":   listSlidesTool().Handler,
		"read_slide":    readSlideTool().Handler,
		"edit_slide":    editSlideTool().Handler,
		"query_slide":   querySlideTool().Handler,
		"add_slide":     addSlideTool().Handler,
		"remove_slide":  removeSlideTool().Handler,
		"check_deck":    checkDeckTool().Handler,
		"build_deck":    buildDeckTool().Handler,
		"create_deck":   createDeckTool().Handler,
	}
	for name, handler := range tools {
		t.Run(name, func(t *testing.T) {
			// Bare context, no workspace installed.
			result, err := handler(context.Background(), mcpcore.ToolRequest{
				Arguments: json.RawMessage(`{"deck":"whatever"}`),
			})
			if err != nil {
				t.Fatalf("handler returned err: %v", err)
			}
			if !result.IsError {
				t.Errorf("expected IsError=true without workspace")
			}
			if !strings.Contains(toolText(result), "workspace") {
				t.Errorf("error should mention workspace; got: %s", toolText(result))
			}
		})
	}
}
