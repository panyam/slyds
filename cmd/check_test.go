package cmd

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/panyam/slyds/core"
)

// TestCheckDeckJSON verifies that Check() output serializes to valid JSON
// with the expected fields: slide_count (int), in_sync (bool), issues (array),
// and estimated_minutes (float, omitted when zero). This test exercises the
// JSON tags added to CheckResult and Issue in core/check.go and ensures
// agents can reliably parse `slyds check --json` output.
func TestCheckDeckJSON(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Check Test", 3, "default", filepath.Join(root, "deck"), true)

	d, err := core.OpenDeckDir(filepath.Join(root, "deck"))
	if err != nil {
		t.Fatalf("OpenDeckDir: %v", err)
	}

	result, err := d.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
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

	// slide_count must be present and match
	sc, ok := m["slide_count"]
	if !ok {
		t.Error("missing field: slide_count")
	} else if int(sc.(float64)) != result.SlideCount {
		t.Errorf("slide_count = %v, want %d", sc, result.SlideCount)
	}

	// in_sync must be present
	if _, ok := m["in_sync"]; !ok {
		t.Error("missing field: in_sync")
	}

	// issues must be present (array or null for empty)
	issues, ok := m["issues"]
	if !ok {
		t.Error("missing field: issues")
	} else if issues != nil {
		if _, isArr := issues.([]any); !isArr {
			t.Errorf("issues is %T, want array or null", issues)
		}
	}
}

// TestCheckIssueTypeJSON verifies that IssueType marshals to human-readable
// string labels ("error", "warning", "info") rather than numeric values.
// This ensures agents parsing `slyds check --json` output get meaningful
// issue type identifiers.
func TestCheckIssueTypeJSON(t *testing.T) {
	cases := []struct {
		typ  core.IssueType
		want string
	}{
		{core.IssueError, `"error"`},
		{core.IssueWarning, `"warning"`},
		{core.IssueInfo, `"info"`},
	}
	for _, tc := range cases {
		data, err := json.Marshal(tc.typ)
		if err != nil {
			t.Fatalf("Marshal(%v): %v", tc.typ, err)
		}
		if string(data) != tc.want {
			t.Errorf("IssueType %d marshaled to %s, want %s", tc.typ, data, tc.want)
		}
	}
}
