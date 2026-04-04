package core

import (
	"testing"

	"github.com/panyam/templar"
)

func queryTestDeck() *Deck {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-title.html" #}}
{{# include "slides/02-content.html" #}}`))
	mfs.SetFile("slides/01-title.html", []byte(`<section><h1>Title Slide</h1><p class="subtitle">Welcome</p></section>`))
	mfs.SetFile("slides/02-content.html", []byte(`<section><h1>Content</h1><p>First</p><p>Second</p><img src="pic.png"></section>`))
	d, _ := OpenDeck(mfs)
	return d
}

// TestQueryReadH1 verifies reading the text content of an h1 element.
func TestQueryReadH1(t *testing.T) {
	d := queryTestDeck()
	results, err := d.Query("1", "h1", QueryOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0] != "Title Slide" {
		t.Errorf("got %v, want [Title Slide]", results)
	}
}

// TestQueryReadHTML verifies reading inner HTML of an element.
func TestQueryReadHTML(t *testing.T) {
	d := queryTestDeck()
	results, err := d.Query("1", "h1", QueryOpts{HTML: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0] != "Title Slide" {
		t.Errorf("got %v", results)
	}
}

// TestQueryReadAttr verifies reading an attribute value.
func TestQueryReadAttr(t *testing.T) {
	d := queryTestDeck()
	results, err := d.Query("2", "img", QueryOpts{Attr: "src"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0] != "pic.png" {
		t.Errorf("got %v, want [pic.png]", results)
	}
}

// TestQueryReadCount verifies counting matches.
func TestQueryReadCount(t *testing.T) {
	d := queryTestDeck()
	results, err := d.Query("2", "p", QueryOpts{Count: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0] != "2" {
		t.Errorf("got %v, want [2]", results)
	}
}

// TestQueryReadNoMatch verifies that querying with no matches returns empty.
func TestQueryReadNoMatch(t *testing.T) {
	d := queryTestDeck()
	results, err := d.Query("1", ".nonexistent", QueryOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("got %v, want empty", results)
	}
}

// TestQuerySetText verifies setting inner text of an element via DeckFS.
func TestQuerySetText(t *testing.T) {
	d := queryTestDeck()
	newText := "Updated Title"
	_, err := d.Query("1", "h1", QueryOpts{Set: &newText})
	if err != nil {
		t.Fatal(err)
	}
	content, _ := d.GetSlideContent(1)
	if content == "" {
		t.Fatal("slide content empty after write")
	}
	// Verify via re-query
	results, _ := d.Query("1", "h1", QueryOpts{})
	if len(results) != 1 || results[0] != "Updated Title" {
		t.Errorf("after set: got %v", results)
	}
}

// TestQuerySetHTML verifies setting inner HTML of an element.
func TestQuerySetHTML(t *testing.T) {
	d := queryTestDeck()
	newHTML := "<em>Bold Title</em>"
	_, err := d.Query("1", "h1", QueryOpts{SetHTML: &newHTML})
	if err != nil {
		t.Fatal(err)
	}
	results, _ := d.Query("1", "h1", QueryOpts{HTML: true})
	if len(results) != 1 || results[0] != "<em>Bold Title</em>" {
		t.Errorf("after set-html: got %v", results)
	}
}

// TestQueryAppend verifies appending child HTML to an element.
func TestQueryAppend(t *testing.T) {
	d := queryTestDeck()
	extra := "<span>!</span>"
	_, err := d.Query("1", "h1", QueryOpts{Append: &extra})
	if err != nil {
		t.Fatal(err)
	}
	results, _ := d.Query("1", "h1 span", QueryOpts{})
	if len(results) != 1 || results[0] != "!" {
		t.Errorf("after append: got %v", results)
	}
}

// TestQueryRemove verifies removing matched elements.
func TestQueryRemove(t *testing.T) {
	d := queryTestDeck()
	_, err := d.Query("1", ".subtitle", QueryOpts{Remove: true})
	if err != nil {
		t.Fatal(err)
	}
	results, _ := d.Query("1", ".subtitle", QueryOpts{Count: true})
	if results[0] != "0" {
		t.Errorf("after remove: count = %s, want 0", results[0])
	}
}

// TestQueryWriteNoMatch verifies that write operations on no-match return error.
func TestQueryWriteNoMatch(t *testing.T) {
	d := queryTestDeck()
	text := "nope"
	_, err := d.Query("1", ".nonexistent", QueryOpts{Set: &text})
	if err == nil {
		t.Error("expected error for write on no match")
	}
}

// TestQueryResolveSlideByName verifies resolving a slide by name substring.
func TestQueryResolveSlideByName(t *testing.T) {
	d := queryTestDeck()
	results, err := d.Query("content", "h1", QueryOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0] != "Content" {
		t.Errorf("got %v", results)
	}
}

// TestBatchQueryAtomic verifies that atomic batch applies all ops then writes.
func TestBatchQueryAtomic(t *testing.T) {
	d := queryTestDeck()
	title := "New Title"
	sub := "New Sub"
	ops := []BatchOperation{
		{Slide: "1", Selector: "h1", Op: "set", Value: title},
		{Slide: "1", Selector: ".subtitle", Op: "set", Value: sub},
	}
	if err := d.BatchQuery(ops, true, false); err != nil {
		t.Fatal(err)
	}
	r1, _ := d.Query("1", "h1", QueryOpts{})
	r2, _ := d.Query("1", ".subtitle", QueryOpts{})
	if r1[0] != "New Title" {
		t.Errorf("h1 = %q", r1[0])
	}
	if r2[0] != "New Sub" {
		t.Errorf("subtitle = %q", r2[0])
	}
}
