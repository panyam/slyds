package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/slyds/core"
)

// TestBuildDeckJSON verifies that Build() output serializes to valid JSON
// with "html" containing the built presentation HTML and "warnings" as a
// string array. This test exercises the JSON tags on core.Result and ensures
// agents can reliably parse `slyds build --json` output for programmatic
// access to built presentations without writing to dist/.
func TestBuildDeckJSON(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Build Test", 3, "default", filepath.Join(root, "deck"), true)

	d, err := core.OpenDeckDir(filepath.Join(root, "deck"))
	if err != nil {
		t.Fatalf("OpenDeckDir: %v", err)
	}

	result, err := d.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}

	// Round-trip: unmarshal into a generic map to verify field names.
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// html must be present and contain actual HTML
	html, ok := m["html"]
	if !ok {
		t.Fatal("missing field: html")
	}
	htmlStr, _ := html.(string)
	if !strings.Contains(htmlStr, "<html") {
		t.Error("html field doesn't contain <html")
	}
	if !strings.Contains(htmlStr, "Build Test") {
		t.Error("html field doesn't contain deck title")
	}

	// warnings must be present as an array (may be null or empty)
	if _, ok := m["warnings"]; !ok {
		t.Error("missing field: warnings")
	}
}
