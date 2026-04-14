package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/mcpkit/testutil"
	"github.com/panyam/slyds/core"
	slydsv1 "github.com/panyam/slyds/gen/go/slyds/v1"
)

// newProtoMCPClient creates a TestClient connected to a slyds MCP server
// using the proto-generated tool and resource registration. This is the
// proto equivalent of newSlydsMCPClient.
func newProtoMCPClient(t *testing.T, root string) *testutil.TestClient {
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
	return testutil.NewTestClient(t, srv)
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
