package core

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/panyam/templar"
)

func checkTestDeck() *Deck {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-title.html" #}}
{{# include "slides/02-content.html" #}}
{{# include "slides/03-closing.html" #}}`))
	mfs.SetFile("slides/01-title.html", []byte(`<div class="slide" data-layout="title"><h1>Welcome</h1><div class="speaker-notes"><p>Intro notes</p></div></div>`))
	mfs.SetFile("slides/02-content.html", []byte(`<div class="slide" data-layout="content"><h1>Details</h1><p>Body text</p><div class="speaker-notes"><p>Some detailed notes here with enough words</p></div></div>`))
	mfs.SetFile("slides/03-closing.html", []byte(`<div class="slide" data-layout="closing"><h1>Thanks</h1><div class="speaker-notes"><p>Wrap up</p></div></div>`))
	d, _ := OpenDeck(mfs)
	return d
}

// TestCheckCleanDeck verifies that a well-formed deck produces no errors or warnings.
func TestCheckCleanDeck(t *testing.T) {
	d := checkTestDeck()
	result, err := d.Check()
	if err != nil {
		t.Fatal(err)
	}
	if result.SlideCount != 3 {
		t.Errorf("slide count = %d, want 3", result.SlideCount)
	}
	if !result.InSync {
		t.Error("expected InSync = true")
	}
	if result.Issues.HasErrors() {
		t.Errorf("unexpected errors: %v", result.Issues.Errors())
	}
}

// TestCheckMissingFiles verifies that slides referenced in index.html but
// missing from the filesystem are flagged as errors.
func TestCheckMissingFiles(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-exists.html" #}}
{{# include "slides/02-missing.html" #}}`))
	mfs.SetFile("slides/01-exists.html", []byte(`<div data-layout="content"><div class="speaker-notes">notes</div></div>`))
	// 02-missing.html intentionally NOT created

	d, _ := OpenDeck(mfs)
	result, _ := d.Check()

	errors := result.Issues.Errors()
	if len(errors) == 0 {
		t.Fatal("expected error for missing slide")
	}
	if !strings.Contains(errors[0].Detail, "02-missing.html") {
		t.Errorf("error detail = %q, want mention of 02-missing.html", errors[0].Detail)
	}
	if result.InSync {
		t.Error("expected InSync = false")
	}
}

// TestCheckOrphanFiles verifies that slide files on disk but not referenced
// in index.html are flagged as warnings.
func TestCheckOrphanFiles(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-used.html" #}}`))
	mfs.SetFile("slides/01-used.html", []byte(`<div data-layout="content"><div class="speaker-notes">notes</div></div>`))
	mfs.SetFile("slides/02-orphan.html", []byte(`<div>orphan</div>`))

	d, _ := OpenDeck(mfs)
	result, _ := d.Check()

	orphans := result.Issues.Contains("orphan")
	if len(orphans) == 0 {
		t.Fatal("expected orphan warning")
	}
	if !strings.Contains(orphans[0].Detail, "02-orphan.html") {
		t.Errorf("warning = %q", orphans[0].Detail)
	}
}

// TestCheckMissingSpeakerNotes verifies that slides without speaker notes
// are flagged as warnings.
func TestCheckMissingSpeakerNotes(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-no-notes.html" #}}`))
	mfs.SetFile("slides/01-no-notes.html", []byte(`<div data-layout="content"><h1>No Notes</h1></div>`))

	d, _ := OpenDeck(mfs)
	result, _ := d.Check()

	notes := result.Issues.Contains("no speaker notes")
	if len(notes) == 0 {
		t.Fatal("expected warning for missing speaker notes")
	}
}

// TestCheckBrokenAssetRef verifies that references to nonexistent local assets
// (images, etc.) are flagged as warnings.
func TestCheckBrokenAssetRef(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-broken.html" #}}`))
	mfs.SetFile("slides/01-broken.html", []byte(`<div data-layout="content"><h1>Demo</h1><img src="images/missing.png"><div class="speaker-notes"><p>notes</p></div></div>`))

	d, _ := OpenDeck(mfs)
	result, _ := d.Check()

	broken := result.Issues.Contains("missing.png")
	if len(broken) == 0 {
		t.Fatal("expected broken asset warning for missing.png")
	}
}

// TestCheckBrokenAssetExistingAsset verifies that references to existing assets
// do NOT produce warnings.
func TestCheckBrokenAssetExistingAsset(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-ok.html" #}}`))
	mfs.SetFile("slides/01-ok.html", []byte(`<div data-layout="content"><img src="images/logo.png"><div class="speaker-notes">notes</div></div>`))
	mfs.SetFile("images/logo.png", []byte("PNG..."))

	d, _ := OpenDeck(mfs)
	result, _ := d.Check()

	broken := result.Issues.Contains("broken asset")
	if len(broken) != 0 {
		t.Errorf("unexpected broken asset warning: %v", broken)
	}
}

// TestCheckRemoteAssetIgnored verifies that remote URLs (http/https) are
// not flagged as broken assets.
func TestCheckRemoteAssetIgnored(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-remote.html" #}}`))
	mfs.SetFile("slides/01-remote.html", []byte(`<div data-layout="content"><img src="https://example.com/photo.jpg"><div class="speaker-notes">notes</div></div>`))

	d, _ := OpenDeck(mfs)
	result, _ := d.Check()

	broken := result.Issues.Contains("broken asset")
	if len(broken) != 0 {
		t.Errorf("remote URL should not be flagged: %v", broken)
	}
}

// TestCheckTalkTime verifies that speaker notes word count produces
// a talk time estimate (~150 words per minute).
func TestCheckTalkTime(t *testing.T) {
	// ~300 words of notes = ~2 minutes
	notes := strings.Repeat("word ", 300)
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-talk.html" #}}`))
	mfs.SetFile("slides/01-talk.html", []byte(`<div data-layout="content"><div class="speaker-notes"><p>`+notes+`</p></div></div>`))

	d, _ := OpenDeck(mfs)
	result, _ := d.Check()

	if result.EstimatedMinutes < 1.5 || result.EstimatedMinutes > 2.5 {
		t.Errorf("estimated minutes = %.1f, want ~2.0", result.EstimatedMinutes)
	}
}

// TestCheckMissingDataLayout verifies that slides without data-layout attribute
// are flagged with the detected layout from CSS classes.
func TestCheckMissingDataLayout(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-no-attr.html" #}}`))
	mfs.SetFile("slides/01-no-attr.html", []byte(`<div class="slide title-slide"><h1>Title</h1><div class="speaker-notes">notes</div></div>`))

	d, _ := OpenDeck(mfs)
	result, _ := d.Check()

	layout := result.Issues.Contains("no data-layout")
	if len(layout) == 0 {
		t.Fatal("expected warning for missing data-layout attribute")
	}
}

// TestCheckUnknownLayout verifies that slides with an unrecognized
// data-layout value are flagged.
func TestCheckUnknownLayout(t *testing.T) {
	mfs := templar.NewMemFS()
	mfs.SetFile("index.html", []byte(`{{# include "slides/01-bad.html" #}}`))
	mfs.SetFile("slides/01-bad.html", []byte(`<div data-layout="nonexistent-layout"><h1>Bad</h1><div class="speaker-notes">notes</div></div>`))

	d, _ := OpenDeck(mfs)
	result, _ := d.Check()

	unknown := result.Issues.Contains("unknown layout")
	if len(unknown) == 0 {
		t.Fatal("expected warning for unknown layout")
	}
}

// TestIssuesFilter verifies the Issues helper methods.
func TestIssuesFilter(t *testing.T) {
	issues := Issues{
		{Type: IssueError, Detail: "missing file"},
		{Type: IssueWarning, Detail: "orphan file"},
		{Type: IssueWarning, Detail: "no speaker notes"},
	}

	if len(issues.Errors()) != 1 {
		t.Errorf("Errors() = %d, want 1", len(issues.Errors()))
	}
	if len(issues.Warnings()) != 2 {
		t.Errorf("Warnings() = %d, want 2", len(issues.Warnings()))
	}
	if len(issues.Contains("orphan")) != 1 {
		t.Errorf("Contains(orphan) = %d, want 1", len(issues.Contains("orphan")))
	}
}

// TestIssueType_String verifies the human-readable label for each IssueType.
func TestIssueType_String(t *testing.T) {
	tests := []struct {
		t    IssueType
		want string
	}{
		{IssueError, "error"},
		{IssueWarning, "warning"},
		{IssueInfo, "info"},
		{IssueType(99), "unknown(99)"},
	}
	for _, tt := range tests {
		if got := tt.t.String(); got != tt.want {
			t.Errorf("IssueType(%d).String() = %q, want %q", tt.t, got, tt.want)
		}
	}
}

// TestIssueType_MarshalJSON verifies JSON encoding of IssueType values.
func TestIssueType_MarshalJSON(t *testing.T) {
	for _, tt := range []struct {
		t    IssueType
		want string
	}{
		{IssueError, `"error"`},
		{IssueWarning, `"warning"`},
		{IssueInfo, `"info"`},
	} {
		data, err := json.Marshal(tt.t)
		if err != nil {
			t.Fatalf("MarshalJSON(%d): %v", tt.t, err)
		}
		if string(data) != tt.want {
			t.Errorf("MarshalJSON(%d) = %s, want %s", tt.t, data, tt.want)
		}
	}
}
