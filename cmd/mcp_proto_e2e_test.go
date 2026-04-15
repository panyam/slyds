package cmd

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/mcpkit/client"
	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/mcpkit/testutil"
	"github.com/panyam/slyds/core"
	slydsv1 "github.com/panyam/slyds/gen/go/slyds/v1"
)

// newProtoMCPClient creates a TestClient connected to a slyds MCP server
// using the proto-generated tool and resource registration. This is the
// proto equivalent of newSlydsMCPClient.
func newProtoMCPClient(t *testing.T, root string, opts ...client.ClientOption) *testutil.TestClient {
	t.Helper()
	ws, err := NewLocalWorkspace(root)
	if err != nil {
		t.Fatalf("NewLocalWorkspace: %v", err)
	}
	srv := server.NewServer(
		mcpcore.ServerInfo{Name: "slyds-proto-test", Version: "0.0.1"},
		server.WithMiddleware(workspaceMiddleware(ws)),
	)
	impl := &SlydsServiceImpl{}
	slydsv1.RegisterSlydsServiceMCP(srv, impl)
	slydsv1.RegisterSlydsServiceMCPResources(srv, impl)
	slydsv1.RegisterSlydsServiceMCPCompletions(srv, impl)
	slydsv1.RegisterSlydsServiceMCPPrompts(srv, impl)
	return testutil.NewTestClient(t, srv, opts...)
}

// --- Proto E2E tests: verify parity with hand-written handlers ---

// TestProtoE2E_ListDecks verifies that list_decks via proto-generated
// handler returns the same deck names as the hand-written handler.
func TestProtoE2E_ListDecks(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Alpha", 2, "default", filepath.Join(root, "alpha"), true)
	core.CreateInDir("Beta", 2, "dark", filepath.Join(root, "beta"), true)

	pc := newProtoMCPClient(t, root)
	hc := newSlydsMCPClient(t, root)

	protoResult := pc.ToolCall("list_decks", map[string]any{})
	handResult := hc.ToolCall("list_decks", map[string]any{})

	// Both should contain the same deck names.
	for _, name := range []string{"alpha", "beta"} {
		if !strings.Contains(protoResult, name) {
			t.Errorf("proto list_decks missing %q: %s", name, protoResult)
		}
		if !strings.Contains(handResult, name) {
			t.Errorf("hand list_decks missing %q: %s", name, handResult)
		}
	}
}

// TestProtoE2E_DescribeDeck verifies that describe_deck returns matching
// metadata from both paths.
func TestProtoE2E_DescribeDeck(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Describe Test", 3, "dark", filepath.Join(root, "deck"), true)

	pc := newProtoMCPClient(t, root)
	hc := newSlydsMCPClient(t, root)

	protoRaw := pc.ToolCall("describe_deck", map[string]any{"deck": "deck"})
	handRaw := hc.ToolCall("describe_deck", map[string]any{"deck": "deck"})

	// Parse both as generic JSON and compare key fields.
	var protoDesc, handDesc map[string]any
	json.Unmarshal([]byte(protoRaw), &protoDesc)
	json.Unmarshal([]byte(handRaw), &handDesc)

	if protoDesc["title"] != handDesc["title"] {
		t.Errorf("title mismatch: proto=%v hand=%v", protoDesc["title"], handDesc["title"])
	}
	if protoDesc["theme"] != handDesc["theme"] {
		t.Errorf("theme mismatch: proto=%v hand=%v", protoDesc["theme"], handDesc["theme"])
	}
	if protoDesc["slide_count"] != handDesc["slide_count"] {
		t.Errorf("slide_count mismatch: proto=%v hand=%v", protoDesc["slide_count"], handDesc["slide_count"])
	}
}

// TestProtoE2E_ReadSlide verifies that read_slide returns the same content
// and version fields from both paths.
func TestProtoE2E_ReadSlide(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Read Test", 3, "default", filepath.Join(root, "deck"), true)

	pc := newProtoMCPClient(t, root)
	hc := newSlydsMCPClient(t, root)

	protoRaw := pc.ToolCall("read_slide", map[string]any{"deck": "deck", "position": 1})
	handRaw := hc.ToolCall("read_slide", map[string]any{"deck": "deck", "position": 1})

	var protoRes, handRes slideReadResult
	json.Unmarshal([]byte(protoRaw), &protoRes)
	json.Unmarshal([]byte(handRaw), &handRes)

	if protoRes.Content != handRes.Content {
		t.Error("read_slide content mismatch between proto and hand-written")
	}
	if protoRes.Version != handRes.Version {
		t.Errorf("version mismatch: proto=%q hand=%q", protoRes.Version, handRes.Version)
	}
	if protoRes.DeckVersion != handRes.DeckVersion {
		t.Errorf("deck_version mismatch: proto=%q hand=%q", protoRes.DeckVersion, handRes.DeckVersion)
	}
}

// TestProtoE2E_EditSlide verifies that edit_slide works through the proto
// path and returns version info.
func TestProtoE2E_EditSlide(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Edit Test", 3, "default", filepath.Join(root, "deck"), true)

	pc := newProtoMCPClient(t, root)

	result := pc.ToolCall("edit_slide", map[string]any{
		"deck":     "deck",
		"position": 1,
		"content":  `<div class="slide"><h1>Proto Edited</h1></div>`,
	})
	if strings.Contains(result, "error") && !strings.Contains(result, "version") {
		t.Fatalf("edit_slide failed: %s", result)
	}

	// Verify the edit persisted by reading back.
	readRaw := pc.ToolCall("read_slide", map[string]any{"deck": "deck", "position": 1})
	var readRes slideReadResult
	json.Unmarshal([]byte(readRaw), &readRes)
	if !strings.Contains(readRes.Content, "Proto Edited") {
		t.Error("edit not persisted through proto path")
	}
}

// TestProtoE2E_EditSlideVersionConflict verifies that the proto path
// returns ABORTED (via version_conflict) for stale expected_version.
func TestProtoE2E_EditSlideVersionConflict(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Conflict Test", 3, "default", filepath.Join(root, "deck"), true)

	pc := newProtoMCPClient(t, root)

	result, err := pc.Client.ToolCallFull("edit_slide", map[string]any{
		"deck":             "deck",
		"position":         1,
		"content":          `<div class="slide"><h1>Stale</h1></div>`,
		"expected_version": "0000000000000000",
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for version conflict")
	}
	// The proto path returns gRPC ABORTED which maps to an MCP error.
	errorText := result.Content[0].Text
	if !strings.Contains(strings.ToLower(errorText), "version") && !strings.Contains(strings.ToLower(errorText), "aborted") {
		t.Errorf("error should mention version conflict or aborted: %s", errorText)
	}
}

// TestProtoE2E_AddSlide verifies that add_slide works through the proto path.
func TestProtoE2E_AddSlide(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Add Test", 3, "default", filepath.Join(root, "deck"), true)

	pc := newProtoMCPClient(t, root)

	result := pc.ToolCall("add_slide", map[string]any{
		"deck":     "deck",
		"position": 2,
		"name":     "inserted",
		"layout":   "content",
	})
	if strings.Contains(result, "error") {
		t.Fatalf("add_slide failed: %s", result)
	}

	// Verify slide count increased.
	descRaw := pc.ToolCall("describe_deck", map[string]any{"deck": "deck"})
	var desc map[string]any
	json.Unmarshal([]byte(descRaw), &desc)
	if desc["slide_count"] != float64(4) {
		t.Errorf("slide_count after add = %v, want 4", desc["slide_count"])
	}
}

// TestProtoE2E_RemoveSlide verifies that remove_slide works through proto.
func TestProtoE2E_RemoveSlide(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Remove Test", 3, "default", filepath.Join(root, "deck"), true)

	pc := newProtoMCPClient(t, root)

	result := pc.ToolCall("remove_slide", map[string]any{
		"deck":  "deck",
		"slide": "1",
	})
	if strings.Contains(result, "error") {
		t.Fatalf("remove_slide failed: %s", result)
	}

	descRaw := pc.ToolCall("describe_deck", map[string]any{"deck": "deck"})
	var desc map[string]any
	json.Unmarshal([]byte(descRaw), &desc)
	if desc["slide_count"] != float64(2) {
		t.Errorf("slide_count after remove = %v, want 2", desc["slide_count"])
	}
}

// TestProtoE2E_CheckDeck verifies that check_deck works through proto.
func TestProtoE2E_CheckDeck(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Check Test", 3, "default", filepath.Join(root, "deck"), true)

	pc := newProtoMCPClient(t, root)

	result := pc.ToolCall("check_deck", map[string]any{"deck": "deck"})
	// Should return JSON with issues array.
	if !strings.Contains(result, "issues") {
		t.Errorf("check_deck result missing 'issues': %s", result)
	}
}

// TestProtoE2E_BuildDeck verifies that build_deck returns HTML.
func TestProtoE2E_BuildDeck(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Build Test", 3, "default", filepath.Join(root, "deck"), true)

	pc := newProtoMCPClient(t, root)

	result := pc.ToolCall("build_deck", map[string]any{"deck": "deck"})
	if !strings.Contains(result, "<html") {
		t.Errorf("build_deck missing HTML: %s", result[:min(200, len(result))])
	}
}

// TestProtoE2E_CreateDeck verifies that create_deck works through proto.
func TestProtoE2E_CreateDeck(t *testing.T) {
	root := t.TempDir()

	pc := newProtoMCPClient(t, root)

	result := pc.ToolCall("create_deck", map[string]any{
		"name":  "new-deck",
		"title": "Proto Created",
		"theme": "dark",
	})
	if !strings.Contains(result, "Proto Created") {
		t.Errorf("create_deck result missing title: %s", result)
	}

	// Verify it's listable.
	listResult := pc.ToolCall("list_decks", map[string]any{})
	if !strings.Contains(listResult, "new-deck") {
		t.Error("created deck not in list_decks")
	}
}

// --- Resource parity tests ---

// TestProtoE2E_ResourceDeckMetadata verifies that the proto resource
// handler for slyds://decks/{name} returns the same content as hand-written.
func TestProtoE2E_ResourceDeckMetadata(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Resource Test", 3, "default", filepath.Join(root, "deck"), true)

	pc := newProtoMCPClient(t, root)
	hc := newSlydsMCPClient(t, root)

	protoRes := pc.ReadResource("slyds://decks/deck")
	handRes := hc.ReadResource("slyds://decks/deck")

	// Both should contain the deck title.
	if !strings.Contains(protoRes, "Resource Test") {
		t.Error("proto resource missing title")
	}
	if !strings.Contains(handRes, "Resource Test") {
		t.Error("hand resource missing title")
	}
}

// TestProtoE2E_ResourceSlideContent verifies that the proto resource
// handler for slyds://decks/{name}/slides/{n} returns slide HTML.
func TestProtoE2E_ResourceSlideContent(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Slide Resource", 3, "default", filepath.Join(root, "deck"), true)

	pc := newProtoMCPClient(t, root)

	content := pc.ReadResource("slyds://decks/deck/slides/1")
	if !strings.Contains(content, "slide") {
		t.Errorf("proto slide resource missing slide content: %s", content)
	}
}

// TestProtoE2E_ResourceServerInfo verifies the server info resource.
func TestProtoE2E_ResourceServerInfo(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Info Test", 2, "default", filepath.Join(root, "deck"), true)

	pc := newProtoMCPClient(t, root)

	info := pc.ReadResource("slyds://server/info")
	if !strings.Contains(info, "slyds") {
		t.Error("server info missing 'slyds' name")
	}
	if !strings.Contains(info, "default") {
		t.Error("server info missing 'default' theme")
	}
}

// TestProtoE2E_ResourceDeckConfig verifies the config resource.
func TestProtoE2E_ResourceDeckConfig(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Config Test", 2, "dark", filepath.Join(root, "deck"), true)

	pc := newProtoMCPClient(t, root)

	config := pc.ReadResource("slyds://decks/deck/config")
	if !strings.Contains(config, "dark") {
		t.Error("config resource missing theme")
	}
}

// TestProtoE2E_ResourceAgentGuide verifies the AGENT.md resource.
func TestProtoE2E_ResourceAgentGuide(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Agent Test", 2, "default", filepath.Join(root, "deck"), true)

	pc := newProtoMCPClient(t, root)

	guide := pc.ReadResource("slyds://decks/deck/agent")
	if !strings.Contains(guide, "slyds") {
		t.Error("agent guide missing slyds reference")
	}
}

// TestProtoE2E_FullWorkflow exercises a complete agent workflow through
// the proto path: create → describe → edit → read → build.
func TestProtoE2E_FullWorkflow(t *testing.T) {
	root := t.TempDir()

	pc := newProtoMCPClient(t, root)

	// 1. Create deck
	pc.ToolCall("create_deck", map[string]any{
		"name": "workflow", "title": "Workflow Test", "theme": "minimal", "slides": 3,
	})

	// 2. Describe
	descRaw := pc.ToolCall("describe_deck", map[string]any{"deck": "workflow"})
	if !strings.Contains(descRaw, "Workflow Test") {
		t.Fatal("describe missing title")
	}

	// 3. Edit slide 1
	pc.ToolCall("edit_slide", map[string]any{
		"deck": "workflow", "position": 1,
		"content": `<div class="slide"><h1>Proto Workflow</h1></div>`,
	})

	// 4. Read back
	readRaw := pc.ToolCall("read_slide", map[string]any{"deck": "workflow", "position": 1})
	var readRes slideReadResult
	json.Unmarshal([]byte(readRaw), &readRes)
	if !strings.Contains(readRes.Content, "Proto Workflow") {
		t.Error("edit not persisted in workflow")
	}

	// 5. Build
	buildResult := pc.ToolCall("build_deck", map[string]any{"deck": "workflow"})
	if !strings.Contains(buildResult, "Proto Workflow") {
		t.Error("build output missing edited content")
	}
}

// --- Proto completion tests ---

// TestProtoE2E_CompleteDeckName verifies that proto-generated completions
// return deck names matching the prefix.
func TestProtoE2E_CompleteDeckName(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Alpha", 2, "default", filepath.Join(root, "alpha"), true)
	core.CreateInDir("Beta", 2, "dark", filepath.Join(root, "beta"), true)

	pc := newProtoMCPClient(t, root)

	values := completeResource(t, pc, "slyds://decks/{name}", "name", "")
	if len(values) < 2 {
		t.Fatalf("expected at least 2 completions, got %d: %v", len(values), values)
	}
	has := func(name string) bool {
		for _, v := range values {
			if v == name {
				return true
			}
		}
		return false
	}
	if !has("alpha") {
		t.Error("completion missing 'alpha'")
	}
	if !has("beta") {
		t.Error("completion missing 'beta'")
	}
}

// TestProtoE2E_CompleteDeckNamePrefix verifies prefix filtering.
func TestProtoE2E_CompleteDeckNamePrefix(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Alpha", 2, "default", filepath.Join(root, "alpha"), true)
	core.CreateInDir("Beta", 2, "dark", filepath.Join(root, "beta"), true)

	pc := newProtoMCPClient(t, root)

	values := completeResource(t, pc, "slyds://decks/{name}", "name", "al")
	if len(values) != 1 || values[0] != "alpha" {
		t.Errorf("expected [alpha], got %v", values)
	}
}

// TestProtoE2E_CompleteSlidePosition verifies slide position completions.
func TestProtoE2E_CompleteSlidePosition(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Slides", 5, "default", filepath.Join(root, "deck"), true)

	pc := newProtoMCPClient(t, root)

	values := completeResource(t, pc, "slyds://decks/{name}/slides/{n}", "n", "")
	if len(values) != 5 {
		t.Fatalf("expected 5 positions, got %d: %v", len(values), values)
	}
}

// TestProtoE2E_CompletionParity verifies proto completions match hand-written.
func TestProtoE2E_CompletionParity(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Parity", 3, "default", filepath.Join(root, "deck"), true)

	pc := newProtoMCPClient(t, root)
	hc := newSlydsMCPClientForCompletions(t, root)

	protoValues := completeResource(t, pc, "slyds://decks/{name}", "name", "")
	handValues := completeResource(t, hc, "slyds://decks/{name}", "name", "")

	if len(protoValues) != len(handValues) {
		t.Errorf("completion count mismatch: proto=%d hand=%d", len(protoValues), len(handValues))
	}
	for i := range protoValues {
		if i < len(handValues) && protoValues[i] != handValues[i] {
			t.Errorf("completion[%d] mismatch: proto=%q hand=%q", i, protoValues[i], handValues[i])
		}
	}
}

// --- Deep parity tests: compare JSON field-by-field ---

// assertJSONKeysMatch verifies that two JSON objects have the same top-level
// keys. Reports which keys are missing from each side.
func assertJSONKeysMatch(t *testing.T, label string, protoJSON, handJSON string) {
	t.Helper()
	var protoMap, handMap map[string]any
	if err := json.Unmarshal([]byte(protoJSON), &protoMap); err != nil {
		t.Fatalf("%s: proto result not a JSON object: %v\nraw: %s", label, err, protoJSON[:min(200, len(protoJSON))])
	}
	if err := json.Unmarshal([]byte(handJSON), &handMap); err != nil {
		t.Fatalf("%s: hand result not a JSON object: %v\nraw: %s", label, err, handJSON[:min(200, len(handJSON))])
	}
	for k := range handMap {
		if _, ok := protoMap[k]; !ok {
			t.Errorf("%s: proto missing key %q (present in hand-written)", label, k)
		}
	}
	for k := range protoMap {
		if _, ok := handMap[k]; !ok {
			t.Errorf("%s: proto has extra key %q (not in hand-written)", label, k)
		}
	}
}

// setupParityDecks creates a test workspace with consistent decks for
// parity testing. Returns the root and a cleanup function.
func setupParityDecks(t *testing.T) (string, *testutil.TestClient, *testutil.TestClient) {
	t.Helper()
	root := t.TempDir()
	core.CreateInDir("Parity Test", 3, "dark", filepath.Join(root, "deck"), true)
	return root, newProtoMCPClient(t, root), newSlydsMCPClient(t, root)
}

// TestParity_ListDecks verifies that list_decks returns the same JSON
// structure from both proto and hand-written paths.
func TestParity_ListDecks(t *testing.T) {
	_, pc, hc := setupParityDecks(t)

	protoRaw := pc.ToolCall("list_decks", map[string]any{})
	handRaw := hc.ToolCall("list_decks", map[string]any{})

	assertJSONKeysMatch(t, "list_decks", protoRaw, handRaw)

	// Parse and compare deck entries.
	var protoList, handList struct {
		Decks []map[string]any `json:"decks"`
	}
	json.Unmarshal([]byte(protoRaw), &protoList)
	json.Unmarshal([]byte(handRaw), &handList)

	if len(protoList.Decks) != len(handList.Decks) {
		t.Fatalf("deck count: proto=%d hand=%d", len(protoList.Decks), len(handList.Decks))
	}
	for i := range protoList.Decks {
		for _, key := range []string{"name", "title", "theme", "slides"} {
			pv, hv := protoList.Decks[i][key], handList.Decks[i][key]
			if pv != hv {
				t.Errorf("list_decks[%d].%s: proto=%v hand=%v", i, key, pv, hv)
			}
		}
	}
}

// TestParity_DescribeDeck verifies describe_deck JSON field parity.
func TestParity_DescribeDeck(t *testing.T) {
	_, pc, hc := setupParityDecks(t)

	protoRaw := pc.ToolCall("describe_deck", map[string]any{"deck": "deck"})
	handRaw := hc.ToolCall("describe_deck", map[string]any{"deck": "deck"})

	assertJSONKeysMatch(t, "describe_deck", protoRaw, handRaw)

	var protoDesc, handDesc map[string]any
	json.Unmarshal([]byte(protoRaw), &protoDesc)
	json.Unmarshal([]byte(handRaw), &handDesc)

	for _, key := range []string{"title", "theme", "slide_count", "deck_version"} {
		if protoDesc[key] != handDesc[key] {
			t.Errorf("describe_deck.%s: proto=%v hand=%v", key, protoDesc[key], handDesc[key])
		}
	}

	// Verify slide array has same length and per-slide keys match.
	protoSlides, _ := protoDesc["slides"].([]any)
	handSlides, _ := handDesc["slides"].([]any)
	if len(protoSlides) != len(handSlides) {
		t.Fatalf("slides count: proto=%d hand=%d", len(protoSlides), len(handSlides))
	}
	for i := range protoSlides {
		ps, _ := protoSlides[i].(map[string]any)
		hs, _ := handSlides[i].(map[string]any)
		for _, key := range []string{"position", "file", "slug", "layout", "title", "words", "has_notes", "images", "version"} {
			if ps[key] != hs[key] {
				t.Errorf("slides[%d].%s: proto=%v hand=%v", i, key, ps[key], hs[key])
			}
		}
	}
}

// TestParity_ReadSlide verifies read_slide returns identical content,
// version, and deck_version.
func TestParity_ReadSlide(t *testing.T) {
	_, pc, hc := setupParityDecks(t)

	protoRaw := pc.ToolCall("read_slide", map[string]any{"deck": "deck", "position": 1})
	handRaw := hc.ToolCall("read_slide", map[string]any{"deck": "deck", "position": 1})

	assertJSONKeysMatch(t, "read_slide", protoRaw, handRaw)

	var pr, hr slideReadResult
	json.Unmarshal([]byte(protoRaw), &pr)
	json.Unmarshal([]byte(handRaw), &hr)

	if pr.Content != hr.Content {
		t.Error("read_slide content differs")
	}
	if pr.Version != hr.Version {
		t.Errorf("read_slide version: proto=%q hand=%q", pr.Version, hr.Version)
	}
	if pr.DeckVersion != hr.DeckVersion {
		t.Errorf("read_slide deck_version: proto=%q hand=%q", pr.DeckVersion, hr.DeckVersion)
	}
}

// TestParity_ListSlides verifies list_slides returns same slide metadata.
func TestParity_ListSlides(t *testing.T) {
	_, pc, hc := setupParityDecks(t)

	protoRaw := pc.ToolCall("list_slides", map[string]any{"deck": "deck"})
	handRaw := hc.ToolCall("list_slides", map[string]any{"deck": "deck"})

	assertJSONKeysMatch(t, "list_slides", protoRaw, handRaw)

	var protoList, handList struct {
		Slides []map[string]any `json:"slides"`
	}
	json.Unmarshal([]byte(protoRaw), &protoList)
	json.Unmarshal([]byte(handRaw), &handList)

	if len(protoList.Slides) != len(handList.Slides) {
		t.Fatalf("slide count: proto=%d hand=%d", len(protoList.Slides), len(handList.Slides))
	}
	for i := range protoList.Slides {
		for _, key := range []string{"position", "file", "slug", "layout", "version"} {
			pv, hv := protoList.Slides[i][key], handList.Slides[i][key]
			if pv != hv {
				t.Errorf("slides[%d].%s: proto=%v hand=%v", i, key, pv, hv)
			}
		}
	}
}

// TestParity_CheckDeck verifies check_deck returns same issue structure.
func TestParity_CheckDeck(t *testing.T) {
	_, pc, hc := setupParityDecks(t)

	protoRaw := pc.ToolCall("check_deck", map[string]any{"deck": "deck"})
	handRaw := hc.ToolCall("check_deck", map[string]any{"deck": "deck"})

	var protoMap, handMap map[string]any
	if err := json.Unmarshal([]byte(protoRaw), &protoMap); err != nil {
		t.Fatalf("proto check_deck not JSON: %v\nraw: %s", err, protoRaw[:min(200, len(protoRaw))])
	}
	if err := json.Unmarshal([]byte(handRaw), &handMap); err != nil {
		t.Fatalf("hand check_deck not JSON: %v\nraw: %s", err, handRaw[:min(200, len(handRaw))])
	}

	// Compare key fields.
	for _, key := range []string{"slide_count", "in_sync"} {
		if protoMap[key] != handMap[key] {
			t.Errorf("check_deck.%s: proto=%v hand=%v", key, protoMap[key], handMap[key])
		}
	}
}

// TestParity_BuildDeck verifies build_deck returns the same HTML content.
// Both paths should return {"html": "...", "warnings": [...]}.
func TestParity_BuildDeck(t *testing.T) {
	_, pc, hc := setupParityDecks(t)

	protoRaw := pc.ToolCall("build_deck", map[string]any{"deck": "deck"})
	handRaw := hc.ToolCall("build_deck", map[string]any{"deck": "deck"})

	var protoRes, handRes map[string]any
	if err := json.Unmarshal([]byte(protoRaw), &protoRes); err != nil {
		t.Fatalf("proto build not JSON: %v\nraw: %s", err, protoRaw[:min(200, len(protoRaw))])
	}
	if err := json.Unmarshal([]byte(handRaw), &handRes); err != nil {
		t.Fatalf("hand build not JSON: %v\nraw: %s", err, handRaw[:min(200, len(handRaw))])
	}

	// Both should have an "html" key with the built content.
	protoHTML, _ := protoRes["html"].(string)
	handHTML, _ := handRes["html"].(string)

	for _, marker := range []string{"<html", "Parity Test", `class="slide`, "<style>"} {
		if !strings.Contains(protoHTML, marker) {
			t.Errorf("proto build html missing %q", marker)
		}
		if !strings.Contains(handHTML, marker) {
			t.Errorf("hand build html missing %q", marker)
		}
	}
}

// TestParity_EditSlide verifies edit_slide response has matching version fields.
func TestParity_EditSlide(t *testing.T) {
	root := t.TempDir()
	// Create two identical decks — one for each path.
	core.CreateInDir("Edit Parity", 3, "default", filepath.Join(root, "proto-deck"), true)
	core.CreateInDir("Edit Parity", 3, "default", filepath.Join(root, "hand-deck"), true)

	pc := newProtoMCPClient(t, root)
	hc := newSlydsMCPClient(t, root)

	newContent := `<div class="slide"><h1>Parity Edit</h1></div>`

	protoRaw := pc.ToolCall("edit_slide", map[string]any{
		"deck": "proto-deck", "position": 1, "content": newContent,
	})
	handRaw := hc.ToolCall("edit_slide", map[string]any{
		"deck": "hand-deck", "position": 1, "content": newContent,
	})

	// Both should return version fields.
	var protoRes, handRes map[string]any
	json.Unmarshal([]byte(protoRaw), &protoRes)
	json.Unmarshal([]byte(handRaw), &handRes)

	// Version should be the same — same content written to identical decks.
	if protoRes["version"] != handRes["version"] {
		t.Errorf("edit version: proto=%v hand=%v", protoRes["version"], handRes["version"])
	}

	// Verify edit persisted identically in both paths.
	protoRead := pc.ToolCall("read_slide", map[string]any{"deck": "proto-deck", "position": 1})
	handRead := hc.ToolCall("read_slide", map[string]any{"deck": "hand-deck", "position": 1})

	var protoSlide, handSlide slideReadResult
	json.Unmarshal([]byte(protoRead), &protoSlide)
	json.Unmarshal([]byte(handRead), &handSlide)

	if protoSlide.Content != handSlide.Content {
		t.Error("edited content differs between proto and hand paths")
	}
}

// TestParity_ResourceServerInfo verifies server info resource has same fields.
func TestParity_ResourceServerInfo(t *testing.T) {
	_, pc, hc := setupParityDecks(t)

	protoInfo := pc.ReadResource("slyds://server/info")
	handInfo := hc.ReadResource("slyds://server/info")

	var protoMap, handMap map[string]any
	json.Unmarshal([]byte(protoInfo), &protoMap)
	json.Unmarshal([]byte(handInfo), &handMap)

	for _, key := range []string{"name", "version", "themes", "layouts"} {
		if protoMap[key] == nil {
			t.Errorf("proto server/info missing %q", key)
		}
		if handMap[key] == nil {
			t.Errorf("hand server/info missing %q", key)
		}
	}
}

// TestParity_ResourceSlideContent verifies slide resource returns same HTML.
func TestParity_ResourceSlideContent(t *testing.T) {
	_, pc, hc := setupParityDecks(t)

	protoHTML := pc.ReadResource("slyds://decks/deck/slides/1")
	handHTML := hc.ReadResource("slyds://decks/deck/slides/1")

	if protoHTML != handHTML {
		t.Errorf("slide resource content differs:\nproto: %s\nhand:  %s",
			protoHTML[:min(100, len(protoHTML))],
			handHTML[:min(100, len(handHTML))])
	}
}

// TestParity_ResourceDeckConfig verifies config resource returns same YAML.
func TestParity_ResourceDeckConfig(t *testing.T) {
	_, pc, hc := setupParityDecks(t)

	protoConfig := pc.ReadResource("slyds://decks/deck/config")
	handConfig := hc.ReadResource("slyds://decks/deck/config")

	if protoConfig != handConfig {
		t.Errorf("config resource differs:\nproto: %s\nhand:  %s",
			protoConfig[:min(100, len(protoConfig))],
			handConfig[:min(100, len(handConfig))])
	}
}

// --- Proto path: prompts parity ---

// TestProtoE2E_PromptsList verifies the proto server registers the same
// prompts as the hand-written server.
func TestProtoE2E_PromptsList(t *testing.T) {
	root := t.TempDir()
	pc := newProtoMCPClient(t, root)

	result := pc.Call("prompts/list", nil)
	var parsed struct {
		Prompts []mcpcore.PromptDef `json:"prompts"`
	}
	if err := result.Unmarshal(&parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.Prompts) != 3 {
		t.Fatalf("expected 3 prompts, got %d", len(parsed.Prompts))
	}
}

// --- Proto path: elicitation on remove_slide ---

// TestProtoE2E_RemoveSlide_Declined verifies elicitation decline prevents
// slide removal in the proto path.
func TestProtoE2E_RemoveSlide_Declined(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Proto Elicit", 3, "default", filepath.Join(root, "deck"), true)

	pc := newProtoMCPClient(t, root,
		client.WithElicitationHandler(func(_ context.Context, _ mcpcore.ElicitationRequest) (mcpcore.ElicitationResult, error) {
			return mcpcore.ElicitationResult{Action: "decline"}, nil
		}),
	)

	// Proto RemoveSlide returns a response even on decline (empty fields).
	pc.ToolCall("remove_slide", map[string]any{"deck": "deck", "slide": "2"})

	// Slide should NOT be removed.
	descResult := pc.ToolCall("describe_deck", map[string]any{"deck": "deck"})
	var desc testDeckDescription
	json.Unmarshal([]byte(descResult), &desc)
	if desc.SlideCount != 3 {
		t.Errorf("expected 3 slides (unchanged), got %d", desc.SlideCount)
	}
}

// TestProtoE2E_RemoveSlide_NoElicitation verifies backward compatibility:
// no elicitation handler means slide is still removed.
func TestProtoE2E_RemoveSlide_NoElicitation(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Proto Compat", 3, "default", filepath.Join(root, "deck"), true)

	pc := newProtoMCPClient(t, root)

	pc.ToolCall("remove_slide", map[string]any{"deck": "deck", "slide": "2"})

	descResult := pc.ToolCall("describe_deck", map[string]any{"deck": "deck"})
	var desc testDeckDescription
	json.Unmarshal([]byte(descResult), &desc)
	if desc.SlideCount != 2 {
		t.Errorf("expected 2 slides after removal, got %d", desc.SlideCount)
	}
}

// --- Proto path: elicitation on create_deck ---

// TestProtoE2E_CreateDeck_ElicitTheme verifies theme elicitation in proto path.
func TestProtoE2E_CreateDeck_ElicitTheme(t *testing.T) {
	root := t.TempDir()

	pc := newProtoMCPClient(t, root,
		client.WithElicitationHandler(func(_ context.Context, _ mcpcore.ElicitationRequest) (mcpcore.ElicitationResult, error) {
			return mcpcore.ElicitationResult{
				Action:  "accept",
				Content: map[string]any{"theme": "dark"},
			}, nil
		}),
	)

	pc.ToolCall("create_deck", map[string]any{
		"name":  "elicit-proto",
		"title": "Elicited Proto",
	})

	descResult := pc.ToolCall("describe_deck", map[string]any{"deck": "elicit-proto"})
	var desc testDeckDescription
	json.Unmarshal([]byte(descResult), &desc)
	if desc.Theme != "dark" {
		t.Errorf("expected theme 'dark', got %q", desc.Theme)
	}
}
