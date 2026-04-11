package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/panyam/templar"
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

// TestAddSlideOutOfRange verifies that AddSlide rejects invalid positions.
func TestAddSlideOutOfRange(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-a.html" #}}`))
	mfs.SetFile("slides/01-a.html", []byte(`A`))

	d, _ := OpenDeck(mfs)

	if err := d.AddSlide(0, "bad.html", "X"); err == nil {
		t.Error("expected error for position 0")
	}
	if err := d.AddSlide(3, "bad.html", "X"); err == nil {
		t.Error("expected error for position 3 (have 1 slide, max is 2)")
	}
}

// TestAddSlidePreservesContent verifies that existing slide content is
// not modified when a new slide is inserted.
func TestAddSlidePreservesContent(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-first.html" #}}
{{# include "slides/02-last.html" #}}`))
	mfs.SetFile("slides/01-first.html", []byte(`<h1>First</h1>`))
	mfs.SetFile("slides/02-last.html", []byte(`<h1>Last</h1>`))

	d, _ := OpenDeck(mfs)
	d.AddSlide(2, "middle.html", "<h1>Middle</h1>")

	// Verify original content preserved
	c1, _ := d.GetSlideContent(1)
	c3, _ := d.GetSlideContent(3)
	if c1 != "<h1>First</h1>" {
		t.Errorf("slide 1 content changed: %q", c1)
	}
	if c3 != "<h1>Last</h1>" {
		t.Errorf("slide 3 content changed: %q", c3)
	}
}

// TestMultipleInserts verifies sequential inserts at different positions.
func TestMultipleInserts(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-a.html" #}}`))
	mfs.SetFile("slides/01-a.html", []byte(`A`))

	d, _ := OpenDeck(mfs)

	d.AddSlide(1, "b.html", "B")  // insert at beginning
	d.AddSlide(3, "c.html", "C")  // append at end

	count, _ := d.SlideCount()
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}

	c1, _ := d.GetSlideContent(1)
	c2, _ := d.GetSlideContent(2)
	c3, _ := d.GetSlideContent(3)
	if c1 != "B" {
		t.Errorf("slide 1 = %q, want B", c1)
	}
	if c2 != "A" {
		t.Errorf("slide 2 = %q, want A", c2)
	}
	if c3 != "C" {
		t.Errorf("slide 3 = %q, want C", c3)
	}
}

// TestSlugifySlides verifies that slides are renamed based on h1 headings.
func TestSlugifySlides(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-untitled.html" #}}
{{# include "slides/02-untitled.html" #}}`))
	mfs.SetFile("slides/01-untitled.html", []byte(`<section><h1>Hello World</h1></section>`))
	mfs.SetFile("slides/02-untitled.html", []byte(`<section><h1>Getting Started</h1></section>`))

	d, _ := OpenDeck(mfs)
	slugFn := func(s string) string {
		return strings.ToLower(strings.ReplaceAll(s, " ", "-"))
	}

	renamed, err := d.SlugifySlides(slugFn)
	if err != nil {
		t.Fatal(err)
	}
	if renamed != 2 {
		t.Errorf("renamed = %d, want 2", renamed)
	}

	files, _ := d.SlideFilenames()
	if len(files) != 2 {
		t.Fatalf("got %d slides", len(files))
	}
	// Verify slugified names
	if !strings.Contains(files[0], "hello-world") {
		t.Errorf("slide 1 = %q, want hello-world", files[0])
	}
	if !strings.Contains(files[1], "getting-started") {
		t.Errorf("slide 2 = %q, want getting-started", files[1])
	}
}

// TestSlugifySlidesIdempotent verifies that running slugify twice doesn't rename.
func TestSlugifySlidesIdempotent(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-hello.html" #}}`))
	mfs.SetFile("slides/01-hello.html", []byte(`<section><h1>Hello</h1></section>`))

	d, _ := OpenDeck(mfs)
	slugFn := func(s string) string { return strings.ToLower(s) }

	renamed1, _ := d.SlugifySlides(slugFn)
	renamed2, _ := d.SlugifySlides(slugFn)

	if renamed1 != 0 {
		t.Errorf("first slugify renamed %d, expected 0 (already slugified)", renamed1)
	}
	if renamed2 != 0 {
		t.Errorf("second slugify renamed %d, expected 0", renamed2)
	}
}

// TestInsertSlideWithLayout verifies that InsertSlide renders from a layout
// template and inserts with the correct attributes and structure.
func TestInsertSlideWithLayout(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-title.html" #}}`))
	mfs.SetFile("slides/01-title.html", []byte(`<h1>Title</h1>`))

	d, _ := OpenDeck(mfs)
	if _, err := d.InsertSlide(2, "details", "content", ""); err != nil {
		t.Fatal(err)
	}

	content, _ := d.GetSlideContent(2)
	if !strings.Contains(content, `data-layout="content"`) {
		t.Error("missing data-layout=\"content\"")
	}
	if !strings.Contains(content, `data-slot="body"`) {
		t.Error("missing data-slot=\"body\"")
	}
}

// TestInsertSlideWithTwoCol verifies two-column layout has left/right slots.
func TestInsertSlideWithTwoCol(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-a.html" #}}`))
	mfs.SetFile("slides/01-a.html", []byte(`A`))

	d, _ := OpenDeck(mfs)
	_, _ = d.InsertSlide(2, "comparison", "two-col", "")

	content, _ := d.GetSlideContent(2)
	if !strings.Contains(content, `data-layout="two-col"`) {
		t.Error("missing data-layout")
	}
	if !strings.Contains(content, `data-slot="left"`) {
		t.Error("missing left slot")
	}
	if !strings.Contains(content, `data-slot="right"`) {
		t.Error("missing right slot")
	}
}

// TestInsertSlideWithTitle verifies title layout and custom title override.
func TestInsertSlideWithTitle(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-a.html" #}}`))
	mfs.SetFile("slides/01-a.html", []byte(`A`))

	d, _ := OpenDeck(mfs)
	_, _ = d.InsertSlide(1, "intro", "title", "Welcome Everyone")

	content, _ := d.GetSlideContent(1)
	if !strings.Contains(content, "Welcome Everyone") {
		t.Errorf("custom title not in content: %s", content)
	}
	if !strings.Contains(content, `data-layout="title"`) {
		t.Error("missing data-layout=\"title\"")
	}
}

// TestInsertSlideUnknownLayout verifies error for nonexistent layout.
func TestInsertSlideUnknownLayout(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-a.html" #}}`))
	mfs.SetFile("slides/01-a.html", []byte(`A`))

	d, _ := OpenDeck(mfs)
	_, err := d.InsertSlide(2, "bad", "nonexistent-layout", "")
	if err == nil {
		t.Fatal("expected error for unknown layout")
	}
}

// TestApplySlots verifies that ApplySlots sets HTML in named data-slot elements.
func TestApplySlots(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-a.html" #}}`))
	mfs.SetFile("slides/01-a.html", []byte(`A`))

	d, _ := OpenDeck(mfs)
	_, _ = d.InsertSlide(2, "details", "content", "")

	err := d.ApplySlots(2, map[string]string{
		"body": "<p>Custom body content</p>",
	})
	if err != nil {
		t.Fatal(err)
	}

	content, _ := d.GetSlideContent(2)
	if !strings.Contains(content, "Custom body content") {
		t.Errorf("slot content not applied: %s", content)
	}
}

// TestExtractNamePart verifies stripping numeric prefixes from filenames.
func TestExtractNamePart(t *testing.T) {
	tests := []struct{ in, want string }{
		{"01-intro.html", "intro.html"},
		{"99-outro.html", "outro.html"},
		{"intro.html", "intro.html"},
		{"1-x.html", "x.html"},
	}
	for _, tt := range tests {
		got := ExtractNamePart(tt.in)
		if got != tt.want {
			t.Errorf("ExtractNamePart(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestFindDeckRoot verifies that FindDeckRoot returns the correct path.
func TestFindDeckRoot(t *testing.T) {
	dir := t.TempDir()
	// No index.html → error
	_, err := FindDeckRoot(dir)
	if err == nil {
		t.Error("expected error for dir without index.html")
	}
}

// TestResolveSlide verifies slide resolution by number and name.
func TestResolveSlide(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-intro.html" #}}
{{# include "slides/02-details.html" #}}`))
	mfs.SetFile("slides/01-intro.html", []byte(`intro`))
	mfs.SetFile("slides/02-details.html", []byte(`details`))

	d, _ := OpenDeck(mfs)

	// By number
	f, err := d.ResolveSlide("1")
	if err != nil || f != "01-intro.html" {
		t.Errorf("resolve 1: %q, %v", f, err)
	}

	// By name
	f, err = d.ResolveSlide("details")
	if err != nil || f != "02-details.html" {
		t.Errorf("resolve details: %q, %v", f, err)
	}

	// Not found
	_, err = d.ResolveSlide("nope")
	if err == nil {
		t.Error("expected error for unknown slide")
	}
}

// --- Slug-as-ID tests (#78) ---

// makeDeckWithSlugs constructs an in-memory deck where slide filenames and
// slug portions are fully under test control. Used by the slug-focused
// tests below to create ambiguous or handcrafted slug states that a real
// scaffolder wouldn't produce.
func makeDeckWithSlugs(t *testing.T, slides ...string) *Deck {
	t.Helper()
	mfs := templar.NewMemFS()
	var includes []byte
	for _, f := range slides {
		includes = append(includes, []byte(`{{# include "slides/`+f+`" #}}`+"\n")...)
		mfs.SetFile("slides/"+f, []byte(`<section><h1>`+f+`</h1></section>`))
	}
	mfs.SetFile("index.html", includes)
	d, err := OpenDeck(mfs)
	if err != nil {
		t.Fatalf("OpenDeck: %v", err)
	}
	return d
}

// TestInsertSlide_ReturnsFinalName verifies the InsertSlide signature
// change: returns (finalName, error) so callers can see whether the
// requested name was used verbatim or auto-suffixed. Exercises the
// non-colliding happy path where the requested name is preserved.
func TestInsertSlide_ReturnsFinalName(t *testing.T) {
	d := makeDeckWithSlugs(t, "01-intro.html", "02-outro.html")

	finalName, err := d.InsertSlide(2, "middle", "content", "Middle")
	if err != nil {
		t.Fatalf("InsertSlide: %v", err)
	}
	if finalName != "middle" {
		t.Errorf("finalName = %q, want %q", finalName, "middle")
	}

	// The resulting filename should be 02-middle.html at position 2.
	slides, _ := d.SlideFilenames()
	if len(slides) != 3 {
		t.Fatalf("slides = %d, want 3", len(slides))
	}
	if slides[1] != "02-middle.html" {
		t.Errorf("slides[1] = %q, want 02-middle.html", slides[1])
	}
}

// TestInsertSlide_SlugCollisionAutoSuffix verifies that InsertSlide
// auto-suffixes the slug when it collides with an existing slide. This
// is the core uniqueness rule — two slides can never share a slug within
// a deck after this PR lands. The suffix pattern matches SlugifySlides
// (intro → intro-2 → intro-3 → ...).
func TestInsertSlide_SlugCollisionAutoSuffix(t *testing.T) {
	d := makeDeckWithSlugs(t, "01-intro.html", "02-outro.html")

	// Insert with a name that collides with the existing intro slide.
	finalName, err := d.InsertSlide(3, "intro", "content", "Intro Two")
	if err != nil {
		t.Fatalf("InsertSlide: %v", err)
	}
	if finalName != "intro-2" {
		t.Errorf("finalName = %q, want %q (auto-suffixed)", finalName, "intro-2")
	}

	slides, _ := d.SlideFilenames()
	// Expect: 01-intro.html, 02-outro.html, 03-intro-2.html
	if len(slides) != 3 {
		t.Fatalf("slides = %d, want 3", len(slides))
	}
	if slides[0] != "01-intro.html" {
		t.Errorf("slides[0] = %q, want 01-intro.html (original untouched)", slides[0])
	}
	if slides[2] != "03-intro-2.html" {
		t.Errorf("slides[2] = %q, want 03-intro-2.html (suffixed)", slides[2])
	}
}

// TestInsertSlide_SlugCollisionMultipleSuffix verifies that the -N counter
// walks past existing suffixes. Given a deck with foo, foo-2, foo-3, an
// InsertSlide call with name="foo" produces foo-4 (not foo-2 overwriting).
func TestInsertSlide_SlugCollisionMultipleSuffix(t *testing.T) {
	d := makeDeckWithSlugs(t,
		"01-foo.html",
		"02-foo-2.html",
		"03-foo-3.html",
	)

	finalName, err := d.InsertSlide(4, "foo", "content", "")
	if err != nil {
		t.Fatalf("InsertSlide: %v", err)
	}
	if finalName != "foo-4" {
		t.Errorf("finalName = %q, want foo-4", finalName)
	}
}

// TestResolveSlide_BySlugExactMatch verifies that an exact slug (the
// non-prefix portion of a filename) resolves to the right slide, even
// when substring matching would return an earlier slide first.
func TestResolveSlide_BySlugExactMatch(t *testing.T) {
	d := makeDeckWithSlugs(t, "01-intro.html", "02-outro.html")

	f, err := d.ResolveSlide("intro")
	if err != nil {
		t.Fatalf("ResolveSlide(intro): %v", err)
	}
	if f != "01-intro.html" {
		t.Errorf("ResolveSlide(intro) = %q, want 01-intro.html", f)
	}
}

// TestResolveSlide_BySlugAmbiguous verifies that a slug matching more
// than one slide returns ErrAmbiguousSlideRef. Hand-crafts an ambiguous
// state (two slides with slug "ambi") that the post-PR InsertSlide would
// never produce — protects existing decks that shipped with duplicate
// slugs before uniqueness was enforced.
func TestResolveSlide_BySlugAmbiguous(t *testing.T) {
	d := makeDeckWithSlugs(t, "01-ambi.html", "02-ambi.html")

	_, err := d.ResolveSlide("ambi")
	if err == nil {
		t.Fatal("expected error for ambiguous slug")
	}
	if !errors.Is(err, ErrAmbiguousSlideRef) {
		t.Errorf("expected ErrAmbiguousSlideRef, got %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "01-ambi.html") || !strings.Contains(msg, "02-ambi.html") {
		t.Errorf("error should name both candidates, got: %v", err)
	}
}

// TestResolveSlide_ByFilenameExact verifies that passing a full filename
// (e.g., "01-intro.html") resolves to that exact file, distinct from the
// slug-match and substring-match paths.
func TestResolveSlide_ByFilenameExact(t *testing.T) {
	d := makeDeckWithSlugs(t, "01-intro.html", "02-outro.html")

	f, err := d.ResolveSlide("01-intro.html")
	if err != nil {
		t.Fatalf("ResolveSlide: %v", err)
	}
	if f != "01-intro.html" {
		t.Errorf("got %q, want 01-intro.html", f)
	}
}

// TestResolveSlide_ByPositionNumeric is a regression guard for the
// existing numeric-ref path. Passing "2" resolves to the slide at
// position 2 (1-based).
func TestResolveSlide_ByPositionNumeric(t *testing.T) {
	d := makeDeckWithSlugs(t, "01-intro.html", "02-outro.html", "03-closing.html")

	f, err := d.ResolveSlide("2")
	if err != nil {
		t.Fatalf("ResolveSlide(2): %v", err)
	}
	if f != "02-outro.html" {
		t.Errorf("ResolveSlide(2) = %q, want 02-outro.html", f)
	}
}

// TestResolveSlide_BySubstringFallback verifies that the legacy substring
// match still works for references that don't match a slug exactly. This
// preserves backward compat for callers that pass partial filenames.
func TestResolveSlide_BySubstringFallback(t *testing.T) {
	d := makeDeckWithSlugs(t, "01-introduction.html", "02-outro.html")

	// "ntro" doesn't match any slug exactly, but substring-matches
	// 01-introduction.html. With only one match, resolution succeeds.
	f, err := d.ResolveSlide("ntro")
	if err != nil {
		t.Fatalf("ResolveSlide(ntro): %v", err)
	}
	if f != "01-introduction.html" {
		t.Errorf("got %q, want 01-introduction.html", f)
	}
}

// TestResolveSlide_BySubstringAmbiguous verifies that substring matches
// are now checked for ambiguity. Before this PR, strings.Contains picked
// the first match silently; after, multiple matches return an error.
// This is a strict improvement — the previous behavior was already wrong.
func TestResolveSlide_BySubstringAmbiguous(t *testing.T) {
	d := makeDeckWithSlugs(t, "01-intro.html", "02-retrospective.html")

	// "tro" substring-matches both files.
	_, err := d.ResolveSlide("tro")
	if err == nil {
		t.Fatal("expected error for ambiguous substring match")
	}
	if !errors.Is(err, ErrAmbiguousSlideRef) {
		t.Errorf("expected ErrAmbiguousSlideRef, got %v", err)
	}
}

// TestDescribe_IncludesSlug verifies that Describe() populates the Slug
// field on every SlideDescription. Slug is derived from the filename by
// stripping the NN- prefix and the .html suffix.
func TestDescribe_IncludesSlug(t *testing.T) {
	d := makeDeckWithSlugs(t, "01-intro.html", "02-metrics.html", "03-closing.html")

	desc, err := d.Describe()
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	want := []string{"intro", "metrics", "closing"}
	if len(desc.Slides) != len(want) {
		t.Fatalf("slides = %d, want %d", len(desc.Slides), len(want))
	}
	for i, s := range desc.Slides {
		if s.Slug != want[i] {
			t.Errorf("slides[%d].Slug = %q, want %q", i, s.Slug, want[i])
		}
	}
}
