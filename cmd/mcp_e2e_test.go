package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panyam/mcpkit/client"
	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/mcpkit/testutil"
	"github.com/panyam/slyds/core"
)

// --- Test-only result structs for ToolCallTyped ---

// testDeckSummary matches the JSON shape returned by list_decks.
type testDeckSummary struct {
	Name   string `json:"name"`
	Title  string `json:"title"`
	Theme  string `json:"theme"`
	Slides int    `json:"slides"`
}

// testDeckList is the wrapper object returned by list_decks structuredContent.
type testDeckList struct {
	Decks []testDeckSummary `json:"decks"`
}

// testSlideList is the wrapper object returned by list_slides structuredContent.
type testSlideList struct {
	Slides []testSlideDescription `json:"slides"`
}

// testDeckDescription matches the JSON shape returned by describe_deck,
// mirroring core.DeckDescription but limited to the fields we verify.
type testDeckDescription struct {
	Title      string                 `json:"title"`
	Theme      string                 `json:"theme"`
	SlideCount int                    `json:"slide_count"`
	Slides     []testSlideDescription `json:"slides"`
}

// testSlideDescription matches the slide entries in a describe_deck response.
type testSlideDescription struct {
	Position int    `json:"position"`
	File     string `json:"file"`
	Layout   string `json:"layout"`
	Title    string `json:"title"`
}

// testCheckResult matches the JSON shape returned by check_deck,
// mirroring core.CheckResult but limited to the fields we verify.
type testCheckResult struct {
	SlideCount int  `json:"slide_count"`
	InSync     bool `json:"in_sync"`
}

// toolCallTyped is a test helper that calls client.ToolCallTyped and fails
// the test on error. Requires tools to return mcpcore.StructuredResult.
func toolCallTyped[T any](t *testing.T, tc *testutil.TestClient, name string, args any) T {
	t.Helper()
	result, err := client.ToolCallTyped[T](tc.Client, name, args)
	if err != nil {
		t.Fatalf("ToolCallTyped(%s): %v", name, err)
	}
	return result
}

// readSlideContent calls read_slide via the TestClient and extracts the
// HTML content from the JSON response. read_slide now returns structured
// JSON with {content, version, deck_version} for optimistic concurrency.
func readSlideContent(t *testing.T, c *testutil.TestClient, args map[string]any) string {
	t.Helper()
	raw := c.ToolCall("read_slide", args)
	var parsed slideReadResult
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("read_slide response not JSON: %v\nraw: %s", err, raw)
	}
	return parsed.Content
}

// newSlydsMCPClient creates a TestClient connected to a slyds MCP server
// with the given deck root. Uses mcpkit/testutil for automatic httptest
// server lifecycle, session management, and t.Fatal on errors.
func newSlydsMCPClient(t *testing.T, root string) *testutil.TestClient {
	t.Helper()
	ws, err := NewLocalWorkspace(root)
	if err != nil {
		t.Fatalf("NewLocalWorkspace: %v", err)
	}
	srv := server.NewServer(
		mcpcore.ServerInfo{Name: "slyds-test", Version: "0.0.1"},
		server.WithMiddleware(workspaceMiddleware(ws)),
	)
	registerResources(srv)
	registerTools(srv)
	return testutil.NewTestClient(t, srv)
}

// TestE2E_FullAgentWorkflow exercises the complete MCP agent lifecycle:
// discover decks → read metadata → read slide → create deck → query →
// edit → verify edit → add slide → check → build. All via HTTP against
// a real mcpkit server with slyds tools and resources registered.
func TestE2E_FullAgentWorkflow(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Existing Deck", 3, "default", fmt.Sprintf("%s/existing", root), true)

	c := newSlydsMCPClient(t, root)

	// 1. Discover decks via resource
	decksJSON := c.ReadResource("slyds://decks")
	var decks []map[string]any
	json.Unmarshal([]byte(decksJSON), &decks)
	if len(decks) != 1 || decks[0]["name"] != "existing" {
		t.Fatalf("expected 1 deck 'existing', got: %v", decks)
	}

	// 2. Read deck metadata via resource
	metaJSON := c.ReadResource("slyds://decks/existing")
	var meta map[string]any
	json.Unmarshal([]byte(metaJSON), &meta)
	if meta["title"] != "Existing Deck" {
		t.Errorf("title = %v, want 'Existing Deck'", meta["title"])
	}

	// 3. Read slide content via resource
	slideHTML := c.ReadResource("slyds://decks/existing/slides/1")
	if !strings.Contains(slideHTML, `class="slide`) {
		t.Error("slide 1 missing slide class")
	}

	// 4. Create a new deck via tool
	createResult := c.ToolCall("create_deck", map[string]any{
		"name": "new-deck", "title": "Agent Created", "theme": "dark", "slides": 2,
	})
	if !strings.Contains(createResult, "Agent Created") {
		t.Error("create_deck didn't return title")
	}

	// 5. Verify new deck appears in resource list
	decksJSON = c.ReadResource("slyds://decks")
	json.Unmarshal([]byte(decksJSON), &decks)
	if len(decks) != 2 {
		t.Errorf("expected 2 decks after create, got %d", len(decks))
	}

	// 6. Query slide h1 via tool
	queryResult := c.ToolCall("query_slide", map[string]any{
		"deck": "new-deck", "slide": "1", "selector": "h1",
	})
	if !strings.Contains(queryResult, "Agent Created") {
		t.Errorf("query h1 = %s, want 'Agent Created'", queryResult)
	}

	// 7. Edit slide via tool
	c.ToolCall("edit_slide", map[string]any{
		"deck": "new-deck", "position": 1,
		"content": `<div class="slide"><h1>Modified</h1></div>`,
	})

	// 8. Verify edit via read_slide tool
	readContent := readSlideContent(t, c, map[string]any{
		"deck": "new-deck", "position": 1,
	})
	if !strings.Contains(readContent, "Modified") {
		t.Error("edit not persisted")
	}

	// 9. Add slide via tool
	c.ToolCall("add_slide", map[string]any{
		"deck": "new-deck", "position": 2, "name": "extra", "layout": "content", "title": "Extra",
	})
	listResult := c.ToolCall("list_slides", map[string]any{"deck": "new-deck"})
	var slideWrapper struct {
		Slides []map[string]any `json:"slides"`
	}
	json.Unmarshal([]byte(listResult), &slideWrapper)
	if len(slideWrapper.Slides) != 3 {
		t.Errorf("after add: expected 3 slides, got %d", len(slideWrapper.Slides))
	}

	// 10. Check deck via tool
	checkResult := c.ToolCall("check_deck", map[string]any{"deck": "new-deck"})
	if !strings.Contains(checkResult, "in_sync") {
		t.Error("check_deck missing in_sync field")
	}

	// 11. Build deck via tool — returns {"html": "...", "warnings": [...]}
	buildRaw := c.ToolCall("build_deck", map[string]any{"deck": "new-deck"})
	var buildRes struct {
		HTML string `json:"html"`
	}
	json.Unmarshal([]byte(buildRaw), &buildRes)
	if !strings.Contains(buildRes.HTML, "<style>") {
		t.Error("build missing inlined CSS")
	}
	if strings.Contains(buildRes.HTML, "{{#") {
		t.Error("build has unresolved includes")
	}
}

// TestE2E_ResourceTemplatesList verifies that all 6 resource templates
// are registered and discoverable via resources/templates/list.
func TestE2E_ResourceTemplatesList(t *testing.T) {
	c := newSlydsMCPClient(t, t.TempDir())

	templates := c.ListResourceTemplates()
	names := make(map[string]bool)
	for _, tmpl := range templates {
		names[tmpl.URITemplate] = true
	}

	expected := []string{
		"slyds://decks",
		"slyds://decks/{name}",
		"slyds://decks/{name}/slides",
		"slyds://decks/{name}/slides/{n}",
		"slyds://decks/{name}/config",
		"slyds://decks/{name}/agent",
	}
	for _, uri := range expected {
		if !names[uri] {
			t.Errorf("missing resource template: %s", uri)
		}
	}
}

// TestE2E_ToolsList verifies that all 10 semantic tools are registered
// and discoverable via tools/list.
func TestE2E_ToolsList(t *testing.T) {
	c := newSlydsMCPClient(t, t.TempDir())

	tools := c.ListTools()
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}

	expected := []string{
		"list_decks", "create_deck", "describe_deck", "list_slides", "read_slide",
		"edit_slide", "query_slide", "add_slide", "remove_slide",
		"check_deck", "build_deck",
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing tool: %s", name)
		}
	}
}

// TestE2E_ServerInfo verifies the slyds://server/info static resource
// returns server version, available themes, and available layouts.
func TestE2E_ServerInfo(t *testing.T) {
	c := newSlydsMCPClient(t, t.TempDir())

	text := c.ReadResource("slyds://server/info")
	var info map[string]any
	json.Unmarshal([]byte(text), &info)

	if info["name"] != "slyds" {
		t.Errorf("server name = %v", info["name"])
	}
	themes, _ := info["themes"].([]any)
	if len(themes) < 3 {
		t.Errorf("expected >=3 themes, got %d", len(themes))
	}
}

// TestE2E_ListDecks verifies that the list_decks tool returns typed structured
// content with name, title, theme, and slide count for each deck. Uses
// ToolCallTyped to verify structured result unmarshaling end-to-end.
func TestE2E_ListDecks(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Alpha Talk", 3, "default", fmt.Sprintf("%s/alpha", root), true)
	core.CreateInDir("Beta Talk", 5, "dark", fmt.Sprintf("%s/beta", root), true)

	c := newSlydsMCPClient(t, root)

	result := toolCallTyped[testDeckList](t, c, "list_decks", map[string]any{})

	if len(result.Decks) != 2 {
		t.Fatalf("expected 2 decks, got %d", len(result.Decks))
	}

	names := make(map[string]bool)
	for _, d := range result.Decks {
		names[d.Name] = true
		if d.Title == "" {
			t.Errorf("deck %s has empty title", d.Name)
		}
		if d.Theme == "" {
			t.Errorf("deck %s has empty theme", d.Name)
		}
	}
	if !names["alpha"] || !names["beta"] {
		t.Errorf("expected decks alpha and beta, got: %v", names)
	}
}

// TestE2E_ToolCallTyped verifies that tools returning JSON produce structured
// content that can be unmarshaled via client.ToolCallTyped into typed Go structs.
// Exercises list_decks, describe_deck, list_slides, and check_deck — the four
// tools that return structured JSON via jsonResult. This test validates the
// StructuredResult integration end-to-end: tool handler → wire format →
// client deserialization.
func TestE2E_ToolCallTyped(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Typed Test", 3, "default", fmt.Sprintf("%s/typed", root), true)

	c := newSlydsMCPClient(t, root)

	// 1. list_decks → testDeckList
	deckList := toolCallTyped[testDeckList](t, c, "list_decks", map[string]any{})
	if len(deckList.Decks) != 1 {
		t.Fatalf("list_decks: expected 1 deck, got %d", len(deckList.Decks))
	}
	if deckList.Decks[0].Name != "typed" {
		t.Errorf("list_decks: name = %q, want 'typed'", deckList.Decks[0].Name)
	}
	if deckList.Decks[0].Slides != 3 {
		t.Errorf("list_decks: slides = %d, want 3", deckList.Decks[0].Slides)
	}

	// 2. describe_deck → testDeckDescription
	desc := toolCallTyped[testDeckDescription](t, c, "describe_deck", map[string]any{"deck": "typed"})
	if desc.Title != "Typed Test" {
		t.Errorf("describe_deck: title = %q, want 'Typed Test'", desc.Title)
	}
	if desc.Theme != "default" {
		t.Errorf("describe_deck: theme = %q, want 'default'", desc.Theme)
	}
	if desc.SlideCount != 3 {
		t.Errorf("describe_deck: slide_count = %d, want 3", desc.SlideCount)
	}
	if len(desc.Slides) != 3 {
		t.Errorf("describe_deck: len(slides) = %d, want 3", len(desc.Slides))
	}

	// 3. list_slides → testSlideList
	slideList := toolCallTyped[testSlideList](t, c, "list_slides", map[string]any{"deck": "typed"})
	if len(slideList.Slides) != 3 {
		t.Fatalf("list_slides: expected 3 slides, got %d", len(slideList.Slides))
	}
	if slideList.Slides[0].Position != 1 {
		t.Errorf("list_slides: first slide position = %d, want 1", slideList.Slides[0].Position)
	}

	// 4. check_deck → testCheckResult
	check := toolCallTyped[testCheckResult](t, c, "check_deck", map[string]any{"deck": "typed"})
	if !check.InSync {
		t.Error("check_deck: expected in_sync = true")
	}
	if check.SlideCount != 3 {
		t.Errorf("check_deck: slide_count = %d, want 3", check.SlideCount)
	}
}

// TestPerToolTimeout verifies that per-tool timeout values are set on the
// ToolDef returned by tool factory functions. ToolDef.Timeout is server-side
// only (json:"-"), so this is a unit test inspecting the struct directly
// rather than an E2E test via tools/list.
func TestPerToolTimeout(t *testing.T) {
	build := buildDeckTool()
	if build.Timeout != 30*time.Second {
		t.Errorf("build_deck timeout = %v, want 30s", build.Timeout)
	}

	check := checkDeckTool()
	if check.Timeout != 10*time.Second {
		t.Errorf("check_deck timeout = %v, want 10s", check.Timeout)
	}

	// Other tools should have no per-tool timeout (use server default)
	list := listDecksTool()
	if list.Timeout != 0 {
		t.Errorf("list_decks timeout = %v, want 0 (server default)", list.Timeout)
	}
}

// TestE2E_SlugOnlyDeckWorkflow exercises the full MCP agent lifecycle on a
// deck scaffolded with filename_style: slug-only. This proves that the
// entire MCP stack — workspace resolution, tool dispatch, slide identity
// (slug + slide_id), build, and describe — works correctly when slide files
// have no NN- numeric prefix. The test mirrors TestE2E_FullAgentWorkflow
// but on a slug-only deck.
func TestE2E_SlugOnlyDeckWorkflow(t *testing.T) {
	root := t.TempDir()
	// Scaffold a slug-only deck via CreateInDirWithOpts.
	core.CreateInDirWithOpts(
		fmt.Sprintf("%s/slugonly", root),
		core.ScaffoldOpts{
			Title:           "Slug Only E2E",
			SlideCount:      3,
			ThemeName:       "default",
			IncludeMCPAgent: true,
			FilenameStyle:   "slug-only",
		},
	)

	c := newSlydsMCPClient(t, root)

	// 1. List decks — slug-only deck should appear.
	listResult := c.ToolCall("list_decks", map[string]any{})
	if !strings.Contains(listResult, "slugonly") {
		t.Fatalf("list_decks missing slug-only deck: %s", listResult)
	}

	// 2. Describe — slide filenames should have no NN- prefix.
	descResult := c.ToolCall("describe_deck", map[string]any{"deck": "slugonly"})
	if !strings.Contains(descResult, "title.html") {
		t.Errorf("describe_deck should show title.html (no prefix): %s", descResult[:min(200, len(descResult))])
	}
	if strings.Contains(descResult, "01-title.html") {
		t.Error("describe_deck shows numbered filename on a slug-only deck")
	}

	// 3. Read slide by slug — works the same as numbered.
	readContent := readSlideContent(t, c, map[string]any{
		"deck": "slugonly", "slide": "slide",
	})
	if !strings.Contains(readContent, `class="slide`) {
		t.Error("read_slide by slug failed on slug-only deck")
	}

	// 4. Edit slide by slug.
	c.ToolCall("edit_slide", map[string]any{
		"deck": "slugonly", "slide": "slide",
		"content": `<div class="slide" data-layout="content"><h1>Edited Slug Only</h1></div>`,
	})

	// 5. Verify edit persisted.
	readContent = readSlideContent(t, c, map[string]any{
		"deck": "slugonly", "slide": "slide",
	})
	if !strings.Contains(readContent, "Edited Slug Only") {
		t.Error("edit not persisted on slug-only deck")
	}

	// 6. Add a slide — should get a slug-only filename (no prefix).
	addResult := c.ToolCall("add_slide", map[string]any{
		"deck": "slugonly", "position": 2, "name": "bonus", "layout": "content",
	})
	if !strings.Contains(addResult, "inserted at position 2") {
		t.Fatalf("add_slide: %s", addResult)
	}

	// 7. Describe again — the new slide should have slug-only filename.
	descResult = c.ToolCall("describe_deck", map[string]any{"deck": "slugonly"})
	if !strings.Contains(descResult, "bonus.html") {
		t.Errorf("new slide should be bonus.html (no prefix): %s", descResult[:min(300, len(descResult))])
	}
	if strings.Contains(descResult, "02-bonus.html") {
		t.Error("new slide got a numbered filename on a slug-only deck")
	}

	// 8. Build — should produce valid self-contained HTML.
	buildRaw := c.ToolCall("build_deck", map[string]any{"deck": "slugonly"})
	var slugBuild struct{ HTML string `json:"html"` }
	json.Unmarshal([]byte(buildRaw), &slugBuild)
	if !strings.Contains(slugBuild.HTML, "<style>") {
		t.Error("build missing inlined CSS on slug-only deck")
	}
	if !strings.Contains(slugBuild.HTML, "Edited Slug Only") {
		t.Error("build missing edited content on slug-only deck")
	}
}

// TestE2E_SchemaValidation_RejectsInvalidArgs verifies that mcpkit's
// server-side JSON Schema validation (active by default since v0.1.24)
// rejects malformed tool arguments with a -32602 error before the
// handler runs. This is the contract that lets agents rely on structured
// error data (field path, keyword) to fix their arguments without a
// round-trip through the handler's error handling.
func TestE2E_SchemaValidation_RejectsInvalidArgs(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Schema Test", 2, "default", fmt.Sprintf("%s/test-deck", root), true)

	c := newSlydsMCPClient(t, root)

	// Call read_slide with position as a string instead of the required int.
	// The schema declares position as "type": "integer"; a string should be
	// rejected by the schema validator, not by the handler.
	_, err := c.Client.ToolCall("read_slide", map[string]any{
		"deck":     "test-deck",
		"position": "not-a-number",
	})
	if err == nil {
		t.Fatal("expected error for invalid position type, got nil")
	}
	errMsg := err.Error()
	// mcpkit returns -32602 for schema validation failures.
	if !strings.Contains(errMsg, "-32602") && !strings.Contains(errMsg, "validation") {
		t.Errorf("expected -32602 schema validation error; got: %s", errMsg)
	}
}

// TestBuildDeckTool_WithEmitContent verifies that adding EmitContent
// progress calls to the build_deck handler doesn't change the final
// result. EmitContent is silently dropped on the JSON path used by
// testutil.TestClient, so this is a regression guard for the return
// value — not a test of streaming delivery (which is an mcpkit
// transport concern tested upstream).
func TestBuildDeckTool_WithEmitContent(t *testing.T) {
	root := scaffoldTestDeck(t, "test-deck", "Progress Test", "default", 3)
	tool := buildDeckTool()

	result := callTool(t, root, tool.Handler, map[string]string{"deck": "test-deck"})
	if result.IsError {
		t.Fatalf("build_deck error: %s", toolText(result))
	}

	var parsed struct{ HTML string `json:"html"` }
	json.Unmarshal([]byte(toolText(result)), &parsed)
	if !strings.Contains(parsed.HTML, "<style>") {
		t.Error("build_deck missing inlined CSS after EmitContent addition")
	}
	if !strings.Contains(parsed.HTML, "Progress Test") {
		t.Error("build_deck missing title after EmitContent addition")
	}
}

// TestE2E_SlideIDSurvivesRename is the canonical integration test for
// slide_id (#83): it proves that a slide_id reference survives a rename
// (slugify) that changes the slide's slug and filename — the exact
// scenario that slug alone cannot handle.
//
// Flow:
//  1. Scaffold a 3-slide deck (IDs assigned at scaffold time).
//  2. Describe the deck; capture slide 2's slide_id.
//  3. Edit slide 2's content to change its <h1> heading.
//  4. Call SlugifySlides (via the CLI path) to rename slides based on
//     headings — slide 2's slug changes.
//  5. Read the slide by its ORIGINAL slide_id — should resolve to the
//     renamed file and return the new content.
func TestE2E_SlideIDSurvivesRename(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("ID Rename Test", 3, "default", fmt.Sprintf("%s/test-deck", root), true)

	c := newSlydsMCPClient(t, root)

	// 1. Get slide 2's slide_id from describe_deck
	descResult := c.ToolCall("describe_deck", map[string]any{"deck": "test-deck"})
	type slideInfo struct {
		SlideID string `json:"slide_id"`
		Slug    string `json:"slug"`
	}
	type deckDesc struct {
		Slides []slideInfo `json:"slides"`
	}
	var desc deckDesc
	json.Unmarshal([]byte(descResult), &desc)
	if len(desc.Slides) < 2 {
		t.Fatalf("expected >=2 slides, got %d", len(desc.Slides))
	}
	slideID := desc.Slides[1].SlideID
	if slideID == "" {
		t.Fatal("slide 2 has no slide_id")
	}

	// 2. Edit slide 2's content to give it a distinctive heading
	newContent := `<div class="slide" data-layout="content"><h1>Renamed Heading</h1><p>marker-rename</p></div>`
	c.ToolCall("edit_slide", map[string]any{
		"deck": "test-deck", "slide": slideID, "content": newContent,
	})

	// 3. Run slugify to rename slides based on headings — this changes
	//    slide 2's slug and filename.
	d, _ := core.OpenDeckDir(fmt.Sprintf("%s/test-deck", root))
	renamed, err := d.SlugifySlides(core.Slugify)
	if err != nil {
		t.Fatalf("SlugifySlides: %v", err)
	}
	if renamed == 0 {
		t.Fatal("slugify should have renamed at least 1 slide")
	}

	// 4. Reconnect — the deck state changed on disk outside MCP
	c2 := newSlydsMCPClient(t, root)

	// 5. Read the slide by its ORIGINAL slide_id — should still work!
	readContent := readSlideContent(t, c2, map[string]any{
		"deck": "test-deck", "slide": slideID,
	})
	if !strings.Contains(readContent, "marker-rename") {
		t.Errorf("read by slide_id after rename didn't return the edited content: %s", readContent[:min(200, len(readContent))])
	}
}

// TestE2E_SlugRefSurvivesInsert is the canonical integration test for
// slug-as-ID (#78): it proves that a slug-based `slide` reference remains
// stable across a structural mutation that shifts position numbers.
//
// The flow:
//
//  1. Scaffold a 5-slide deck (slugs: title, slide, slide-2, slide-3, closing).
//  2. Edit the slide at slug "slide-2" via edit_slide — originally at position 3.
//  3. Insert a new slide at position 2 via add_slide, shifting every later
//     slide's position number up by one. "slide-2" is now at position 4.
//  4. Edit again by slug "slide-2". If slug is truly stable, the second edit
//     lands on the same file as the first — not on whatever is at position 3
//     now. Content from both edits should be present in the final read.
//
// This test exists to prove that the whole PR's motivation works end-to-end.
// If it fails, slug identity is broken.
func TestE2E_SlugRefSurvivesInsert(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Slug Stability", 5, "default", fmt.Sprintf("%s/test-deck", root), true)

	c := newSlydsMCPClient(t, root)

	// 1. First edit by slug — lands at position 3 in the 5-slide deck.
	firstEdit := `<div class="slide" data-layout="content"><h1>First Edit</h1><p class="marker-1"></p></div>`
	r := c.ToolCall("edit_slide", map[string]any{
		"deck":    "test-deck",
		"slide":   "slide-2",
		"content": firstEdit,
	})
	if !strings.Contains(r, "updated") {
		t.Fatalf("first edit_slide: %s", r)
	}

	// Read by slug and verify the first edit landed.
	slideContent := readSlideContent(t, c, map[string]any{
		"deck":  "test-deck",
		"slide": "slide-2",
	})
	if !strings.Contains(slideContent, "First Edit") {
		t.Fatalf("first edit content not found: %s", slideContent)
	}

	// 2. Insert a new slide at position 2. The original "slide-2" shifts
	//    to position 4; its filename becomes 04-slide-2.html.
	r = c.ToolCall("add_slide", map[string]any{
		"deck": "test-deck", "position": 2, "name": "inserted", "layout": "content",
	})
	if !strings.Contains(r, "inserted at position 2") {
		t.Fatalf("add_slide: %s", r)
	}

	// 3. Edit by slug "slide-2" AGAIN — should still land on the same
	//    original slide, now at position 4. The first edit's content
	//    must still be present (we didn't clobber it), and the second
	//    edit's content must be appended over.
	secondEdit := `<div class="slide" data-layout="content"><h1>Second Edit</h1><p class="marker-2"></p></div>`
	r = c.ToolCall("edit_slide", map[string]any{
		"deck":    "test-deck",
		"slide":   "slide-2",
		"content": secondEdit,
	})
	if !strings.Contains(r, "updated") {
		t.Fatalf("second edit_slide: %s", r)
	}

	// 4. Read by slug — must return the second edit's content, proving
	//    the slug reference stayed pointed at the same file across the
	//    position shift.
	slideContent = readSlideContent(t, c, map[string]any{
		"deck":  "test-deck",
		"slide": "slide-2",
	})
	if !strings.Contains(slideContent, "Second Edit") {
		t.Errorf("second edit by slug didn't land on the original slide: %s", slideContent)
	}

	// Sanity: position 2 now holds the newly inserted slide, NOT slide-2.
	slideContent = readSlideContent(t, c, map[string]any{
		"deck":     "test-deck",
		"position": 2,
	})
	if strings.Contains(slideContent, "Second Edit") || strings.Contains(slideContent, "marker-2") {
		t.Error("the second edit accidentally landed at position 2 (the inserted slide)")
	}
}

// TestE2E_ReadSlideReturnsVersion verifies that read_slide returns a
// structured JSON response with content, version, and deck_version fields.
func TestE2E_ReadSlideReturnsVersion(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Version Test", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClient(t, root)

	raw := c.ToolCall("read_slide", map[string]any{"deck": "deck", "position": 1})
	var parsed slideReadResult
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("read_slide not JSON: %v", err)
	}
	if parsed.Content == "" {
		t.Error("read_slide content is empty")
	}
	if len(parsed.Version) != 16 {
		t.Errorf("version length = %d, want 16", len(parsed.Version))
	}
	if len(parsed.DeckVersion) != 16 {
		t.Errorf("deck_version length = %d, want 16", len(parsed.DeckVersion))
	}
}

// TestE2E_EditSlideVersionConflict verifies that edit_slide rejects a write
// when expected_version doesn't match the current slide version.
func TestE2E_EditSlideVersionConflict(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Conflict Test", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClient(t, root)

	// Read the current version.
	// Edit with the WRONG version → should fail with isError result.
	result, err := c.Client.ToolCallFull("edit_slide", map[string]any{
		"deck":             "deck",
		"position":         1,
		"content":          `<div class="slide"><h1>Conflict</h1></div>`,
		"expected_version": "0000000000000000",
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected version_conflict error result")
	}
	errorText := result.Content[0].Text
	if !strings.Contains(errorText, "version_conflict") {
		t.Errorf("error should contain version_conflict: %s", errorText)
	}
	// Verify the conflict includes current_version for recovery.
	var conflict versionConflict
	if err := json.Unmarshal([]byte(errorText), &conflict); err != nil {
		t.Fatalf("conflict error not parseable JSON: %v", err)
	}
	if len(conflict.CurrentVersion) != 16 {
		t.Errorf("current_version length = %d, want 16", len(conflict.CurrentVersion))
	}
	if conflict.CurrentContent == "" {
		t.Error("conflict should include current_content for recovery")
	}
}

// TestE2E_EditSlideVersionMatch verifies that edit_slide succeeds when
// expected_version matches, and returns the new version.
func TestE2E_EditSlideVersionMatch(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Match Test", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClient(t, root)

	// Read current version.
	raw := c.ToolCall("read_slide", map[string]any{"deck": "deck", "position": 1})
	var readRes slideReadResult
	json.Unmarshal([]byte(raw), &readRes)

	// Edit with correct version → should succeed.
	editRaw := c.ToolCall("edit_slide", map[string]any{
		"deck":             "deck",
		"position":         1,
		"content":          `<div class="slide"><h1>Updated</h1></div>`,
		"expected_version": readRes.Version,
	})
	var editRes slideEditResult
	if err := json.Unmarshal([]byte(editRaw), &editRes); err != nil {
		t.Fatalf("edit result not JSON: %v\nraw: %s", err, editRaw)
	}
	if editRes.Version == readRes.Version {
		t.Error("version should change after edit")
	}
	if len(editRes.Version) != 16 {
		t.Errorf("new version length = %d, want 16", len(editRes.Version))
	}
}

// TestE2E_EditSlideLatestBypass verifies that expected_version="latest"
// always succeeds regardless of the current version.
func TestE2E_EditSlideLatestBypass(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Latest Test", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClient(t, root)

	result := c.ToolCall("edit_slide", map[string]any{
		"deck":             "deck",
		"position":         1,
		"content":          `<div class="slide"><h1>Latest</h1></div>`,
		"expected_version": "latest",
	})
	if strings.Contains(result, "version_conflict") {
		t.Error("'latest' should bypass version check")
	}
}

// TestE2E_EditSlideEmptyVersionBackwardCompat verifies that omitting
// expected_version succeeds (backward compatibility).
func TestE2E_EditSlideEmptyVersionBackwardCompat(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Compat Test", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClient(t, root)

	result := c.ToolCall("edit_slide", map[string]any{
		"deck":     "deck",
		"position": 1,
		"content":  `<div class="slide"><h1>No Version</h1></div>`,
	})
	if strings.Contains(result, "version_conflict") {
		t.Error("omitting expected_version should skip version check")
	}
}

// TestE2E_AddSlideDeckVersionConflict verifies that add_slide rejects
// when expected_deck_version doesn't match.
func TestE2E_AddSlideDeckVersionConflict(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Add Conflict", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClient(t, root)

	result, err := c.Client.ToolCallFull("add_slide", map[string]any{
		"deck":                  "deck",
		"position":              2,
		"name":                  "extra",
		"expected_deck_version": "0000000000000000",
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected version_conflict error result")
	}
	if !strings.Contains(result.Content[0].Text, "version_conflict") {
		t.Errorf("error should contain version_conflict: %s", result.Content[0].Text)
	}
}

// TestE2E_RemoveSlideDeckVersionConflict verifies that remove_slide rejects
// when expected_deck_version doesn't match.
func TestE2E_RemoveSlideDeckVersionConflict(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Remove Conflict", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClient(t, root)

	result, err := c.Client.ToolCallFull("remove_slide", map[string]any{
		"deck":                  "deck",
		"slide":                 "1",
		"expected_deck_version": "0000000000000000",
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected version_conflict error result")
	}
	if !strings.Contains(result.Content[0].Text, "version_conflict") {
		t.Errorf("error should contain version_conflict: %s", result.Content[0].Text)
	}
}

// TestE2E_DescribeIncludesVersions verifies that describe_deck returns
// deck_version and per-slide version fields.
func TestE2E_DescribeIncludesVersions(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Describe Version", 3, "default", filepath.Join(root, "deck"), true)

	c := newSlydsMCPClient(t, root)

	raw := c.ToolCall("describe_deck", map[string]any{"deck": "deck"})
	var desc core.DeckDescription
	if err := json.Unmarshal([]byte(raw), &desc); err != nil {
		t.Fatalf("describe_deck not JSON: %v", err)
	}
	if len(desc.DeckVersion) != 16 {
		t.Errorf("deck_version length = %d, want 16", len(desc.DeckVersion))
	}
	for i, s := range desc.Slides {
		if len(s.Version) != 16 {
			t.Errorf("slide %d version length = %d, want 16", i+1, len(s.Version))
		}
	}
}

// TestE2E_StdioTransport verifies that the slyds MCP server works over the
// stdio transport (Content-Length framed JSON-RPC over stdin/stdout). This test
// creates a server with all tools and resources registered, wires it to a pair
// of io.Pipes simulating stdin/stdout, performs the MCP initialize handshake,
// then calls tools/list and verifies all 10 slyds tools are returned.
//
// This test exercises the full dispatch pipeline over stdio — the same code path
// used when an editor (Cursor, Claude Desktop) spawns `slyds mcp --stdio`.
func TestE2E_StdioTransport(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Stdio Test", 2, "default", fmt.Sprintf("%s/test-deck", root), true)

	ws, err := NewLocalWorkspace(root)
	if err != nil {
		t.Fatalf("NewLocalWorkspace: %v", err)
	}
	srv := server.NewServer(
		mcpcore.ServerInfo{Name: "slyds-stdio", Version: "0.0.1"},
		server.WithMiddleware(workspaceMiddleware(ws)),
	)
	registerResources(srv)
	registerTools(srv)

	// Create pipe pairs: server reads from sr, client writes to cw;
	// client reads from cr, server writes to sw.
	sr, cw := io.Pipe()
	cr, sw := io.Pipe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- srv.RunStdio(ctx, server.WithStdioInput(sr), server.WithStdioOutput(sw))
	}()

	reader := bufio.NewReader(cr)

	// 1. Initialize handshake.
	writeStdioFrame(t, cw, `{"jsonrpc":"2.0","id":"1","method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`)
	initResp := readStdioFrame(t, reader)
	var resp mcpcore.Response
	if err := json.Unmarshal(initResp, &resp); err != nil {
		t.Fatalf("unmarshal init response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("initialize error: %v", resp.Error)
	}

	// 2. Send initialized notification.
	writeStdioFrame(t, cw, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	// 3. List tools.
	writeStdioFrame(t, cw, `{"jsonrpc":"2.0","id":"2","method":"tools/list","params":{}}`)
	toolsResp := readStdioFrame(t, reader)
	if err := json.Unmarshal(toolsResp, &resp); err != nil {
		t.Fatalf("unmarshal tools/list response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("tools/list error: %v", resp.Error)
	}

	var toolsResult struct {
		Tools []struct{ Name string } `json:"tools"`
	}
	json.Unmarshal(resp.Result, &toolsResult)
	names := make(map[string]bool)
	for _, tool := range toolsResult.Tools {
		names[tool.Name] = true
	}
	expected := []string{
		"list_decks", "create_deck", "describe_deck", "list_slides", "read_slide",
		"edit_slide", "query_slide", "add_slide", "remove_slide",
		"check_deck", "build_deck",
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("stdio: missing tool %q", name)
		}
	}

	// 4. Call describe_deck via stdio to verify tool dispatch.
	writeStdioFrame(t, cw, `{"jsonrpc":"2.0","id":"3","method":"tools/call","params":{"name":"describe_deck","arguments":{"deck":"test-deck"}}}`)
	descResp := readStdioFrame(t, reader)
	if err := json.Unmarshal(descResp, &resp); err != nil {
		t.Fatalf("unmarshal describe_deck response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("describe_deck error: %v", resp.Error)
	}
	var toolResult struct {
		Content []struct{ Text string } `json:"content"`
	}
	json.Unmarshal(resp.Result, &toolResult)
	if len(toolResult.Content) == 0 || !strings.Contains(toolResult.Content[0].Text, "Stdio Test") {
		t.Error("stdio describe_deck didn't return deck title")
	}

	// Clean shutdown.
	cw.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunStdio: %v", err)
		}
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("RunStdio did not exit")
	}
}

// writeStdioFrame writes a Content-Length framed JSON-RPC message to a writer.
func writeStdioFrame(t *testing.T, w io.Writer, msg string) {
	t.Helper()
	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(msg), msg)
	if _, err := io.WriteString(w, frame); err != nil {
		t.Fatalf("writeStdioFrame: %v", err)
	}
}

// readStdioFrame reads a Content-Length framed JSON-RPC message from a reader.
// Parses the Content-Length header and reads exactly that many bytes of body.
func readStdioFrame(t *testing.T, r *bufio.Reader) []byte {
	t.Helper()
	// Read headers until blank line.
	contentLength := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("readStdioFrame header: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == "Content-Length" {
			fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &contentLength)
		}
	}
	if contentLength < 0 {
		t.Fatal("readStdioFrame: missing Content-Length")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		t.Fatalf("readStdioFrame body: %v", err)
	}
	return body
}
