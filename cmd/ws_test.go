package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/slyds/core"
)

// runWsCmd executes the `ws` subcommand with the given args, capturing
// stdout. Uses the global `wsCmd` cobra tree with a fresh stdout buffer
// so each test is isolated.
func runWsCmd(t *testing.T, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	wsCmd.SetOut(&buf)
	wsCmd.SetErr(&buf)
	wsCmd.SetArgs(args)
	if err := wsCmd.Execute(); err != nil {
		t.Fatalf("ws cmd: %v", err)
	}
	return buf.String()
}

// TestWsInfoHumanReadable verifies that `slyds ws info` prints the workspace
// root and deck count in human-readable form. This is the smoke-test path
// that Makefile demo-smoke target invokes.
func TestWsInfoHumanReadable(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Info Test", 2, "default", filepath.Join(root, "alpha"), true)

	// Note: we call the info subcommand directly instead of via the wsCmd
	// parent to avoid interference from persistent flag state across tests.
	// The flag values are set by the cobra flag parser on the subcommand.
	wsDeckRoot = root
	wsJSON = false
	var buf bytes.Buffer
	wsInfoCmd.SetOut(&buf)
	wsInfoCmd.SetErr(&buf)
	if err := wsInfoCmd.RunE(wsInfoCmd, nil); err != nil {
		t.Fatalf("ws info: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Workspace: local") {
		t.Errorf("ws info missing 'Workspace: local': %q", out)
	}
	if !strings.Contains(out, "Decks:     1") {
		t.Errorf("ws info missing 'Decks:     1': %q", out)
	}
}

// TestWsInfoJSON verifies the --json output of `slyds ws info` has the
// expected shape: root, deck_count, kind. Locks the schema so scripts
// (including the Makefile smoke test) can parse it reliably.
func TestWsInfoJSON(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("JSON Test", 2, "default", filepath.Join(root, "alpha"), true)
	core.CreateInDir("JSON Test 2", 3, "dark", filepath.Join(root, "beta"), true)

	wsDeckRoot = root
	wsJSON = true
	defer func() { wsJSON = false }()
	var buf bytes.Buffer
	wsInfoCmd.SetOut(&buf)
	wsInfoCmd.SetErr(&buf)
	if err := wsInfoCmd.RunE(wsInfoCmd, nil); err != nil {
		t.Fatalf("ws info --json: %v", err)
	}

	var info map[string]any
	if err := json.Unmarshal(buf.Bytes(), &info); err != nil {
		t.Fatalf("ws info --json is not valid JSON: %v\n%s", err, buf.String())
	}
	if info["kind"] != "local" {
		t.Errorf("kind = %v, want \"local\"", info["kind"])
	}
	if info["deck_count"].(float64) != 2 {
		t.Errorf("deck_count = %v, want 2", info["deck_count"])
	}
	if info["root"] == "" {
		t.Error("root should be set")
	}
}

// TestWsListJSON verifies that `slyds ws list --json` returns the same
// shape as the MCP list_decks tool. This is the contract that makes the
// CLI and MCP tool output interchangeable for scripting.
func TestWsListJSON(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Alpha", 3, "default", filepath.Join(root, "alpha"), true)
	core.CreateInDir("Beta", 5, "dark", filepath.Join(root, "beta"), true)

	wsDeckRoot = root
	wsJSON = true
	defer func() { wsJSON = false }()
	var buf bytes.Buffer
	wsListCmd.SetOut(&buf)
	wsListCmd.SetErr(&buf)
	if err := wsListCmd.RunE(wsListCmd, nil); err != nil {
		t.Fatalf("ws list --json: %v", err)
	}

	var summaries []deckSummary
	if err := json.Unmarshal(buf.Bytes(), &summaries); err != nil {
		t.Fatalf("ws list --json is not valid JSON: %v\n%s", err, buf.String())
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 decks, got %d", len(summaries))
	}

	got := map[string]deckSummary{}
	for _, s := range summaries {
		got[s.Name] = s
	}
	if got["alpha"].Title != "Alpha" || got["alpha"].Slides != 3 || got["alpha"].Theme != "default" {
		t.Errorf("alpha mismatch: %+v", got["alpha"])
	}
	if got["beta"].Title != "Beta" || got["beta"].Slides != 5 || got["beta"].Theme != "dark" {
		t.Errorf("beta mismatch: %+v", got["beta"])
	}
}

// TestWsListEmpty verifies the human-readable empty-workspace output is
// "(no decks)" — so a human running `slyds ws list` in an empty dir gets
// a clear message instead of silent output.
func TestWsListEmpty(t *testing.T) {
	wsDeckRoot = t.TempDir()
	wsJSON = false
	var buf bytes.Buffer
	wsListCmd.SetOut(&buf)
	wsListCmd.SetErr(&buf)
	if err := wsListCmd.RunE(wsListCmd, nil); err != nil {
		t.Fatalf("ws list: %v", err)
	}
	if !strings.Contains(buf.String(), "(no decks)") {
		t.Errorf("empty workspace should say '(no decks)': %q", buf.String())
	}
}
