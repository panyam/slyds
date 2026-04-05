package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/mcpkit"
	"github.com/panyam/slyds/core"
)

// TestDiscoverDecks verifies that discoverDecks finds all subdirectories
// containing index.html under the given root.
func TestDiscoverDecks(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Alpha", 2, "default", filepath.Join(root, "alpha"), true)
	core.CreateInDir("Beta", 3, "dark", filepath.Join(root, "beta"), true)

	decks := discoverDecks(root)
	if len(decks) != 2 {
		t.Errorf("expected 2 decks, got %d: %v", len(decks), decks)
	}
}

// TestDiscoverDecksEmpty verifies that an empty directory yields no decks.
func TestDiscoverDecksEmpty(t *testing.T) {
	decks := discoverDecks(t.TempDir())
	if len(decks) != 0 {
		t.Errorf("expected 0 decks, got %d", len(decks))
	}
}

// TestDiscoverDecksRootIsDeck verifies that when the root directory itself
// contains index.html, it is discovered as deck ".".
func TestDiscoverDecksRootIsDeck(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Root Deck", 2, "default", root, true)

	decks := discoverDecks(root)
	found := false
	for _, d := range decks {
		if d == "." {
			found = true
		}
	}
	if !found {
		t.Errorf("root deck not discovered as '.': %v", decks)
	}
}

// TestOpenDeck verifies that openDeck resolves a deck name to a Deck
// instance with the correct title and slide count.
func TestOpenDeck(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Test", 3, "default", filepath.Join(root, "my-deck"), true)

	d, err := openDeck(root, "my-deck")
	if err != nil {
		t.Fatalf("openDeck: %v", err)
	}
	if d.Title() != "Test" {
		t.Errorf("title = %q, want Test", d.Title())
	}
	count, _ := d.SlideCount()
	if count != 3 {
		t.Errorf("slides = %d, want 3", count)
	}
}

// TestResourceRegistration verifies that registerResources succeeds and
// the registered deck is readable through the Deck API.
func TestResourceRegistration(t *testing.T) {
	root := t.TempDir()
	core.CreateInDir("Deck", 2, "default", filepath.Join(root, "test-deck"), true)

	srv := mcpkit.NewServer(mcpkit.ServerInfo{Name: "test", Version: "0.0.1"})
	registerResources(srv, root)

	d, err := openDeck(root, "test-deck")
	if err != nil {
		t.Fatalf("openDeck: %v", err)
	}
	content, err := d.GetSlideContent(1)
	if err != nil {
		t.Fatalf("GetSlideContent: %v", err)
	}
	if !strings.Contains(content, `class="slide`) {
		t.Error("slide 1 missing slide class")
	}
}

// TestJsonResultRoundTrip verifies that jsonResult produces valid JSON
// that round-trips correctly through marshal/unmarshal.
func TestJsonResultRoundTrip(t *testing.T) {
	result, _ := jsonResult(map[string]any{"key": "value"})
	text := toolText(result)
	var m map[string]string
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatalf("jsonResult not valid JSON: %v", err)
	}
	if m["key"] != "value" {
		t.Errorf("key = %q, want value", m["key"])
	}
}
