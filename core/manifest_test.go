package core

import (
	"strings"
	"testing"

	"github.com/panyam/templar"
)

func TestWriteAndReadManifest(t *testing.T) {
	mfs := templar.NewMemFS()
	m := Manifest{Theme: "dark", Title: "My Talk"}

	if err := WriteManifestFS(mfs, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	got, err := ReadManifestFS(mfs)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if got.Theme != m.Theme || got.Title != m.Title {
		t.Errorf("got %+v, want %+v", got, m)
	}
}

func TestReadManifestNotFound(t *testing.T) {
	mfs := templar.NewMemFS()
	_, err := ReadManifestFS(mfs)
	if err != ErrManifestNotFound {
		t.Errorf("got error %v, want ErrManifestNotFound", err)
	}
}

func TestManifestFileContents(t *testing.T) {
	mfs := templar.NewMemFS()
	m := Manifest{Theme: "corporate", Title: "Q4 Review"}

	if err := WriteManifestFS(mfs, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	data, err := mfs.ReadFile(".slyds.yaml")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "theme: corporate") || !strings.Contains(content, "title: Q4 Review") {
		t.Errorf("unexpected manifest content:\n%s", content)
	}
}

// TestWriteAndReadManifest_WithSlides verifies that the Slides field
// (the id → filename mapping introduced in #83) round-trips correctly
// through yaml.Marshal / yaml.Unmarshal. Every field must survive
// intact for slide_id lookups to work after a deck is closed and
// reopened.
func TestWriteAndReadManifest_WithSlides(t *testing.T) {
	mfs := templar.NewMemFS()
	m := Manifest{
		Theme: "dark",
		Title: "ID Test",
		Slides: []SlideRecord{
			{ID: "sl_a1b2c3d4", File: "01-title.html"},
			{ID: "sl_deadbeef", File: "02-metrics.html"},
			{ID: "sl_cafe0123", File: "03-closing.html"},
		},
	}

	if err := WriteManifestFS(mfs, m); err != nil {
		t.Fatalf("WriteManifestFS: %v", err)
	}

	got, err := ReadManifestFS(mfs)
	if err != nil {
		t.Fatalf("ReadManifestFS: %v", err)
	}
	if len(got.Slides) != 3 {
		t.Fatalf("got %d slides after roundtrip, want 3", len(got.Slides))
	}
	for i, want := range m.Slides {
		if got.Slides[i] != want {
			t.Errorf("slide[%d] = %+v, want %+v", i, got.Slides[i], want)
		}
	}
}

// TestWriteManifest_OmitEmptySlides verifies the `omitempty` tag on the
// Slides field: a manifest with no slides must not emit a `slides:` key
// at all, so legacy decks remain byte-identical after a manifest write
// with an unset Slides field. This keeps git diffs clean during the
// pre-migration transition period.
func TestWriteManifest_OmitEmptySlides(t *testing.T) {
	mfs := templar.NewMemFS()
	m := Manifest{Theme: "default", Title: "Legacy"}

	if err := WriteManifestFS(mfs, m); err != nil {
		t.Fatalf("WriteManifestFS: %v", err)
	}

	data, _ := mfs.ReadFile(".slyds.yaml")
	content := string(data)
	if strings.Contains(content, "slides:") {
		t.Errorf("manifest without Slides field should not emit slides: key, got:\n%s", content)
	}
}
