package core

import (
	"io/fs"
	"testing"
)

// memFS is a simple in-memory DeckFS for testing.
// Demonstrates that the Deck API is backend-agnostic.
type memFS struct {
	files map[string][]byte
	dirs  map[string]bool
}

func newMemFS() *memFS {
	return &memFS{
		files: make(map[string][]byte),
		dirs:  map[string]bool{".": true},
	}
}

func (m *memFS) Open(name string) (fs.File, error)                     { return nil, fs.ErrNotExist }
func (m *memFS) MkdirAll(path string, perm fs.FileMode) error          { m.dirs[path] = true; return nil }
func (m *memFS) Remove(name string) error                              { delete(m.files, name); return nil }
func (m *memFS) Rename(old, new string) error                          { m.files[new] = m.files[old]; delete(m.files, old); return nil }
func (m *memFS) WriteFile(name string, data []byte, _ fs.FileMode) error { m.files[name] = data; return nil }
func (m *memFS) ReadFile(name string) ([]byte, error) {
	if d, ok := m.files[name]; ok {
		return d, nil
	}
	return nil, fs.ErrNotExist
}
func (m *memFS) Stat(name string) (fs.FileInfo, error) {
	if _, ok := m.files[name]; ok {
		return nil, nil // exists
	}
	return nil, fs.ErrNotExist
}
func (m *memFS) ReadDir(name string) ([]fs.DirEntry, error) { return nil, nil }

// TestOpenDeckMemFS verifies that OpenDeck works with an in-memory filesystem,
// proving the DeckFS abstraction is backend-agnostic.
func TestOpenDeckMemFS(t *testing.T) {
	mfs := newMemFS()
	mfs.files["index.html"] = []byte(`<html>
{{# include "slides/01-title.html" #}}
{{# include "slides/02-content.html" #}}
</html>`)
	mfs.files["slides/01-title.html"] = []byte(`<section><h1>Hello World</h1></section>`)
	mfs.files["slides/02-content.html"] = []byte(`<section><h1>Details</h1><p>Some content</p></section>`)

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
	mfs := newMemFS()
	mfs.files["index.html"] = []byte(`{{# include "slides/01-intro.html" #}}`)
	mfs.files["slides/01-intro.html"] = []byte(`<section><h1>Old</h1></section>`)

	d, err := OpenDeck(mfs)
	if err != nil {
		t.Fatal(err)
	}

	if err := d.EditSlideContent(1, `<section><h1>New</h1></section>`); err != nil {
		t.Fatal(err)
	}

	// Verify the write went through the FS
	got := string(mfs.files["slides/01-intro.html"])
	if got != `<section><h1>New</h1></section>` {
		t.Errorf("after edit: %q", got)
	}
}

// TestOpenDeckNoIndex verifies that OpenDeck fails cleanly when index.html is missing.
func TestOpenDeckNoIndex(t *testing.T) {
	mfs := newMemFS()
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
	mfs := newMemFS()
	mfs.files["index.html"] = []byte(`<html></html>`)
	d, _ := OpenDeck(mfs)
	if d.Title() != "" {
		t.Errorf("title without manifest = %q, want empty", d.Title())
	}
	if d.Theme() != "default" {
		t.Errorf("theme without manifest = %q, want default", d.Theme())
	}

	// With manifest
	mfs.files[".slyds.yaml"] = []byte("title: My Deck\ntheme: corporate\n")
	d2, _ := OpenDeck(mfs)
	if d2.Title() != "My Deck" {
		t.Errorf("title = %q, want My Deck", d2.Title())
	}
	if d2.Theme() != "corporate" {
		t.Errorf("theme = %q, want corporate", d2.Theme())
	}
}
