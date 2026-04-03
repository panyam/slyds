package core

import (
	"github.com/panyam/templar"
	"testing"
)

// TestOpenDeckMemFS verifies that OpenDeck works with an in-memory filesystem,
// proving the DeckFS abstraction is backend-agnostic.
func TestOpenDeckMemFS(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`<html>
{{# include "slides/01-title.html" #}}
{{# include "slides/02-content.html" #}}
</html>`))
	mfs.SetFile("slides/01-title.html", []byte(`<section><h1>Hello World</h1></section>`))
	mfs.SetFile("slides/02-content.html", []byte(`<section><h1>Details</h1><p>Some content</p></section>`))

	d, err := OpenDeck(mfs)
	if err != nil {
		t.Fatal(err)
	}

	// Test SlideFilenames
	files, err := d.SlideFilenames()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d slides, want 2", len(files))
	}
	if files[0] != "01-title.html" {
		t.Errorf("slide[0] = %q, want 01-title.html", files[0])
	}

	// Test SlideCount
	count, err := d.SlideCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	// Test GetSlideContent
	content, err := d.GetSlideContent(1)
	if err != nil {
		t.Fatal(err)
	}
	if content != `<section><h1>Hello World</h1></section>` {
		t.Errorf("slide 1 content = %q", content)
	}

	// Test out of range
	_, err = d.GetSlideContent(3)
	if err == nil {
		t.Error("expected error for out-of-range slide")
	}
}

// TestEditSlideMemFS verifies that EditSlideContent writes through the DeckFS.
func TestEditSlideMemFS(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-intro.html" #}}`))
	mfs.SetFile("slides/01-intro.html", []byte(`<section><h1>Old</h1></section>`))

	d, err := OpenDeck(mfs)
	if err != nil {
		t.Fatal(err)
	}

	if err := d.EditSlideContent(1, `<section><h1>New</h1></section>`); err != nil {
		t.Fatal(err)
	}

	// Verify the write went through the FS
	got, _ := mfs.ReadFile("slides/01-intro.html")
	if string(got) != `<section><h1>New</h1></section>` {
		t.Errorf("after edit: %q", got)
	}
}

// TestOpenDeckNoIndex verifies that OpenDeck fails cleanly when index.html is missing.
func TestOpenDeckNoIndex(t *testing.T) {
	mfs := templar.NewMemFS()
	_, err := OpenDeck(mfs)
	if err == nil {
		t.Error("expected error for missing index.html")
	}
}

// TestExtractFirstHeading verifies heading extraction from HTML strings.
func TestExtractFirstHeading(t *testing.T) {
	tests := []struct {
		html, want string
	}{
		{`<section><h1>Hello World</h1></section>`, "Hello World"},
		{`<section><h1 class="big">Title</h1></section>`, "Title"},
		{`<section><p>No heading</p></section>`, ""},
		{``, ""},
	}
	for _, tt := range tests {
		got := ExtractFirstHeading(tt.html)
		if got != tt.want {
			t.Errorf("ExtractFirstHeading(%q) = %q, want %q", tt.html, got, tt.want)
		}
	}
}

// TestDeckTitleAndTheme verifies defaults and manifest-based values.
func TestDeckTitleAndTheme(t *testing.T) {
	// No manifest
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`<html></html>`))
	d, _ := OpenDeck(mfs)
	if d.Title() != "" {
		t.Errorf("title without manifest = %q, want empty", d.Title())
	}
	if d.Theme() != "default" {
		t.Errorf("theme without manifest = %q, want default", d.Theme())
	}

	// With manifest
	mfs.SetFile(".slyds.yaml", []byte("title: My Deck\ntheme: corporate\n"))
	d2, _ := OpenDeck(mfs)
	if d2.Title() != "My Deck" {
		t.Errorf("title = %q, want My Deck", d2.Title())
	}
	if d2.Theme() != "corporate" {
		t.Errorf("theme = %q, want corporate", d2.Theme())
	}
}

// TestAddSlideMemFS verifies that AddSlide creates a file, inserts into
// the ordering, and renumbers via DeckFS.
func TestAddSlideMemFS(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-intro.html" #}}
{{# include "slides/02-outro.html" #}}`))
	mfs.SetFile("slides/01-intro.html", []byte(`<h1>Intro</h1>`))
	mfs.SetFile("slides/02-outro.html", []byte(`<h1>Outro</h1>`))

	d, _ := OpenDeck(mfs)

	// Insert at position 2 (between intro and outro)
	err := d.AddSlide(2, "new-middle.html", "<h1>Middle</h1>")
	if err != nil {
		t.Fatal(err)
	}

	slides, _ := d.SlideFilenames()
	if len(slides) != 3 {
		t.Fatalf("got %d slides, want 3", len(slides))
	}

	// Verify content of the middle slide
	content, _ := d.GetSlideContent(2)
	if content != "<h1>Middle</h1>" {
		t.Errorf("middle slide content = %q", content)
	}
}

// TestRemoveSlideMemFS verifies that RemoveSlide deletes the file and renumbers.
func TestRemoveSlideMemFS(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-a.html" #}}
{{# include "slides/02-b.html" #}}
{{# include "slides/03-c.html" #}}`))
	mfs.SetFile("slides/01-a.html", []byte(`A`))
	mfs.SetFile("slides/02-b.html", []byte(`B`))
	mfs.SetFile("slides/03-c.html", []byte(`C`))

	d, _ := OpenDeck(mfs)
	err := d.RemoveSlide("02-b.html")
	if err != nil {
		t.Fatal(err)
	}

	slides, _ := d.SlideFilenames()
	if len(slides) != 2 {
		t.Fatalf("got %d slides, want 2", len(slides))
	}

	// Verify the remaining slides
	c1, _ := d.GetSlideContent(1)
	c2, _ := d.GetSlideContent(2)
	if c1 != "A" {
		t.Errorf("slide 1 = %q, want A", c1)
	}
	if c2 != "C" {
		t.Errorf("slide 2 = %q, want C", c2)
	}
}

// TestMoveSlideMemFS verifies that MoveSlide reorders and renumbers.
func TestMoveSlideMemFS(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-a.html" #}}
{{# include "slides/02-b.html" #}}
{{# include "slides/03-c.html" #}}`))
	mfs.SetFile("slides/01-a.html", []byte(`A`))
	mfs.SetFile("slides/02-b.html", []byte(`B`))
	mfs.SetFile("slides/03-c.html", []byte(`C`))

	d, _ := OpenDeck(mfs)
	err := d.MoveSlide(3, 1) // move C to position 1
	if err != nil {
		t.Fatal(err)
	}

	c1, _ := d.GetSlideContent(1)
	c2, _ := d.GetSlideContent(2)
	c3, _ := d.GetSlideContent(3)
	if c1 != "C" {
		t.Errorf("slide 1 = %q, want C", c1)
	}
	if c2 != "A" {
		t.Errorf("slide 2 = %q, want A", c2)
	}
	if c3 != "B" {
		t.Errorf("slide 3 = %q, want B", c3)
	}
}
