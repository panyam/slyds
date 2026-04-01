package cmd

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestBuildIntrospectDocument(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	doc, err := buildIntrospectDocument(root)
	if err != nil {
		t.Fatalf("buildIntrospectDocument: %v", err)
	}
	if doc.Version == "" {
		t.Error("expected version")
	}
	if doc.SchemaVersion != IntrospectSchemaVersion {
		t.Errorf("schema %q", doc.SchemaVersion)
	}
	if doc.Deck == nil || doc.Deck.Root == "" {
		t.Fatal("expected deck context")
	}
	if len(doc.Layouts) < 4 {
		t.Errorf("expected several layouts, got %d", len(doc.Layouts))
	}
	found := false
	for _, l := range doc.Layouts {
		if l.Name == "two-col" && len(l.Slots) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected two-col layout with slots")
	}
	if len(doc.ThemesBuiltin) < 3 {
		t.Errorf("expected themes_builtin, got %d", len(doc.ThemesBuiltin))
	}
	if len(doc.Commands) < 8 {
		t.Errorf("expected command catalog, got %d", len(doc.Commands))
	}
}

func TestIntrospectOutsideDeck(t *testing.T) {
	tmp := t.TempDir()
	doc, err := buildIntrospectDocument(tmp)
	if err != nil {
		t.Fatalf("buildIntrospectDocument: %v", err)
	}
	if doc.Deck != nil {
		t.Error("expected no deck when index.html missing")
	}
	if len(doc.Layouts) == 0 {
		t.Error("expected global layouts")
	}
}

func TestIntrospectJSONRoundTrip(t *testing.T) {
	root, cleanup := setupTestPresentation(t)
	defer cleanup()

	doc, err := buildIntrospectDocument(filepath.Join(root, "slides"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var out IntrospectDocument
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Version != doc.Version {
		t.Error("version mismatch")
	}
}
