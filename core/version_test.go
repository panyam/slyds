package core

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"
)

// TestContentVersion verifies that ContentVersion returns a deterministic
// 16-character hex string derived from the SHA-256 of the input.
func TestContentVersion(t *testing.T) {
	data := []byte("hello world")
	v := ContentVersion(data)

	// Verify length
	if len(v) != 16 {
		t.Errorf("ContentVersion length = %d, want 16", len(v))
	}

	// Verify deterministic
	if v2 := ContentVersion(data); v2 != v {
		t.Errorf("ContentVersion not deterministic: %q != %q", v, v2)
	}

	// Verify matches manual SHA-256 prefix
	h := sha256.Sum256(data)
	want := fmt.Sprintf("%x", h[:8])
	if v != want {
		t.Errorf("ContentVersion = %q, want %q", v, want)
	}

	// Different content produces different version
	v3 := ContentVersion([]byte("different"))
	if v3 == v {
		t.Error("different content should produce different version")
	}
}

// TestSlideVersion verifies that SlideVersion reads the correct slide
// file and returns its content hash.
func TestSlideVersion(t *testing.T) {
	root := t.TempDir()
	_, err := CreateInDir("Version Test", 3, "default", filepath.Join(root, "deck"), true)
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenDeckDir(filepath.Join(root, "deck"))
	if err != nil {
		t.Fatal(err)
	}

	v1, err := d.SlideVersion(1)
	if err != nil {
		t.Fatalf("SlideVersion(1): %v", err)
	}
	if len(v1) != 16 {
		t.Errorf("SlideVersion length = %d, want 16", len(v1))
	}

	// Different slides should have different versions (different content)
	v2, err := d.SlideVersion(2)
	if err != nil {
		t.Fatalf("SlideVersion(2): %v", err)
	}
	// Note: scaffolded slides may have similar content, so we just check
	// they're valid 16-char hex strings, not that they differ.
	if len(v2) != 16 {
		t.Errorf("SlideVersion(2) length = %d, want 16", len(v2))
	}

	// Out of range
	_, err = d.SlideVersion(99)
	if err == nil {
		t.Error("expected error for out-of-range position")
	}
}

// TestDeckVersion verifies that DeckVersion returns a hash of index.html.
func TestDeckVersion(t *testing.T) {
	root := t.TempDir()
	_, err := CreateInDir("DeckVer Test", 3, "default", filepath.Join(root, "deck"), true)
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenDeckDir(filepath.Join(root, "deck"))
	if err != nil {
		t.Fatal(err)
	}

	v, err := d.DeckVersion()
	if err != nil {
		t.Fatalf("DeckVersion: %v", err)
	}
	if len(v) != 16 {
		t.Errorf("DeckVersion length = %d, want 16", len(v))
	}

	// Deterministic
	v2, err := d.DeckVersion()
	if err != nil {
		t.Fatal(err)
	}
	if v2 != v {
		t.Errorf("DeckVersion not deterministic: %q != %q", v, v2)
	}
}

// TestSlideVersionChangesAfterEdit verifies that editing a slide's content
// changes its version.
func TestSlideVersionChangesAfterEdit(t *testing.T) {
	root := t.TempDir()
	_, err := CreateInDir("Edit Test", 3, "default", filepath.Join(root, "deck"), true)
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenDeckDir(filepath.Join(root, "deck"))
	if err != nil {
		t.Fatal(err)
	}

	before, err := d.SlideVersion(1)
	if err != nil {
		t.Fatal(err)
	}

	if err := d.EditSlideContent(1, `<div class="slide"><h1>Changed</h1></div>`); err != nil {
		t.Fatal(err)
	}

	after, err := d.SlideVersion(1)
	if err != nil {
		t.Fatal(err)
	}

	if after == before {
		t.Error("slide version should change after edit")
	}
}
