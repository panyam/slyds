package examples

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/panyam/slyds/core"
	"github.com/panyam/templar"
)

// exampleDeck describes an expected example presentation for table-driven tests.
type exampleDeck struct {
	dir    string // subdirectory name under examples/
	theme  string // expected theme in .slyds.yaml
	title  string // expected title in .slyds.yaml
	slides int    // expected number of slides
	h1Text string // text that must appear in the title slide's h1
}

var decks = []exampleDeck{
	{"slyds-intro", "default", "slyds - Multi-File HTML Presentations", 8, "slyds"},
	{"rich-content", "dark", "Rich Content Showcase", 9, "Rich Content Showcase"},
	{"hacker-showcase", "hacker", "Hacker Theme Showcase", 8, "Hacker Theme Showcase"},
}

// examplesDir returns the absolute path to the examples/ directory.
func examplesDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path via runtime.Caller")
	}
	return filepath.Dir(filename)
}

// openDeck opens an example deck by name.
func openDeck(t *testing.T, name string) *core.Deck {
	t.Helper()
	dir := filepath.Join(examplesDir(t), name)
	d, err := core.OpenDeck(templar.NewLocalFS(dir))
	if err != nil {
		t.Fatalf("OpenDeck(%s) failed: %v", name, err)
	}
	return d
}

// TestExampleDirectoryStructure verifies that each example deck contains all
// required files for a valid slyds presentation.
func TestExampleDirectoryStructure(t *testing.T) {
	requiredFiles := []string{
		".slyds.yaml",
		"index.html",
		"slyds.css",
		"slyds.js",
		"theme.css",
	}

	for _, deck := range decks {
		t.Run(deck.dir, func(t *testing.T) {
			d := openDeck(t, deck.dir)

			for _, f := range requiredFiles {
				if _, err := d.FS.ReadFile(f); err != nil {
					t.Errorf("missing required file: %s", f)
				}
			}

			// slides/ directory must exist and contain the expected number of files
			entries, err := d.FS.ReadDir("slides")
			if err != nil {
				t.Fatalf("cannot read slides/ directory: %v", err)
			}
			if len(entries) != deck.slides {
				t.Errorf("expected %d slide files, got %d", deck.slides, len(entries))
			}
		})
	}
}

// TestExampleManifest verifies that each example's .slyds.yaml manifest
// contains the correct theme and title.
func TestExampleManifest(t *testing.T) {
	for _, deck := range decks {
		t.Run(deck.dir, func(t *testing.T) {
			dir := filepath.Join(examplesDir(t), deck.dir)
			m, err := core.ReadManifestFS(templar.NewLocalFS(dir))
			if err != nil {
				t.Fatalf("ReadManifestFS failed: %v", err)
			}
			if m.Theme != deck.theme {
				t.Errorf("theme = %q, want %q", m.Theme, deck.theme)
			}
			if m.Title != deck.title {
				t.Errorf("title = %q, want %q", m.Title, deck.title)
			}
		})
	}
}

// TestExampleSlideCount verifies that each example's index.html contains
// the expected number of templar include directives.
func TestExampleSlideCount(t *testing.T) {
	for _, deck := range decks {
		t.Run(deck.dir, func(t *testing.T) {
			d := openDeck(t, deck.dir)
			data, err := d.FS.ReadFile("index.html")
			if err != nil {
				t.Fatalf("cannot read index.html: %v", err)
			}
			count := strings.Count(string(data), `{{# include "slides/`)
			if count != deck.slides {
				t.Errorf("index.html has %d include directives, want %d", count, deck.slides)
			}
		})
	}
}

// TestExampleBuild verifies that each example deck builds successfully into
// a self-contained HTML file.
func TestExampleBuild(t *testing.T) {
	for _, deck := range decks {
		t.Run(deck.dir, func(t *testing.T) {
			d := openDeck(t, deck.dir)
			result, err := d.Build()
			if err != nil {
				t.Fatalf("Build failed: %v", err)
			}

			if strings.Contains(result.HTML, "{{#") {
				t.Error("built HTML still contains unresolved templar directives")
			}
			if strings.Contains(result.HTML, `<link rel="stylesheet"`) {
				t.Error("built HTML still contains <link> stylesheet tags")
			}
			if !strings.Contains(result.HTML, "<style>") {
				t.Error("built HTML missing inlined <style> tags")
			}
			if strings.Contains(result.HTML, `<script src=`) {
				t.Error("built HTML still contains <script src> tags")
			}
			if !strings.Contains(result.HTML, "showSlide") {
				t.Error("built HTML missing inlined JS (showSlide function)")
			}

			slideCount := strings.Count(result.HTML, `class="slide`)
			if slideCount < deck.slides {
				t.Errorf("expected at least %d slide elements, got %d", deck.slides, slideCount)
			}
		})
	}
}

// TestExampleContentNotPlaceholder verifies that each example's title slide
// contains real authored content rather than scaffold placeholders.
func TestExampleContentNotPlaceholder(t *testing.T) {
	for _, deck := range decks {
		t.Run(deck.dir, func(t *testing.T) {
			d := openDeck(t, deck.dir)
			entries, err := d.FS.ReadDir("slides")
			if err != nil {
				t.Fatalf("cannot read slides/: %v", err)
			}
			if len(entries) == 0 {
				t.Fatal("no slide files found")
			}

			titleSlide, err := d.FS.ReadFile("slides/" + entries[0].Name())
			if err != nil {
				t.Fatalf("cannot read title slide: %v", err)
			}
			content := string(titleSlide)

			if !strings.Contains(content, deck.h1Text) {
				t.Errorf("title slide missing expected h1 text %q", deck.h1Text)
			}
			if strings.Contains(content, "Your subtitle here") {
				t.Error("title slide still contains scaffold placeholder text")
			}
		})
	}
}

// TestExampleBuildContainsTimer verifies that each example deck's built
// output includes the presenter timer and reading time features.
func TestExampleBuildContainsTimer(t *testing.T) {
	timerMarkers := []string{
		"toggleTimer",
		"computeReadingTimes",
		"notesTimer",
		"notesTimerToggle",
		"formatReadingTime",
	}

	for _, deck := range decks {
		t.Run(deck.dir, func(t *testing.T) {
			d := openDeck(t, deck.dir)
			result, err := d.Build()
			if err != nil {
				t.Fatalf("Build failed: %v", err)
			}
			for _, marker := range timerMarkers {
				if !strings.Contains(result.HTML, marker) {
					t.Errorf("built output missing timer marker: %s", marker)
				}
			}
		})
	}
}

// TestExampleSpeakerNotes verifies that every slide in each example deck
// contains a speaker-notes section.
func TestExampleSpeakerNotes(t *testing.T) {
	for _, deck := range decks {
		t.Run(deck.dir, func(t *testing.T) {
			d := openDeck(t, deck.dir)
			entries, err := d.FS.ReadDir("slides")
			if err != nil {
				t.Fatalf("cannot read slides/: %v", err)
			}

			for _, entry := range entries {
				data, err := d.FS.ReadFile("slides/" + entry.Name())
				if err != nil {
					t.Errorf("cannot read %s: %v", entry.Name(), err)
					continue
				}
				if !strings.Contains(string(data), `speaker-notes`) {
					t.Errorf("%s: missing speaker-notes section", entry.Name())
				}
			}
		})
	}
}
