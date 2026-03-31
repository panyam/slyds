package examples

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/panyam/slyds/internal/builder"
	"github.com/panyam/slyds/internal/scaffold"
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

// examplesDir returns the absolute path to the examples/ directory
// (the directory containing this test file).
func examplesDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path via runtime.Caller")
	}
	return filepath.Dir(filename)
}

// TestExampleDirectoryStructure verifies that each example deck contains all
// required files for a valid slyds presentation: manifest, index, engine
// assets, theme CSS, and a slides directory with the expected number of files.
func TestExampleDirectoryStructure(t *testing.T) {
	root := examplesDir(t)
	requiredFiles := []string{
		".slyds.yaml",
		"index.html",
		"slyds.css",
		"slyds.js",
		"theme.css",
	}

	for _, deck := range decks {
		t.Run(deck.dir, func(t *testing.T) {
			base := filepath.Join(root, deck.dir)

			for _, f := range requiredFiles {
				path := filepath.Join(base, f)
				if _, err := os.Stat(path); os.IsNotExist(err) {
					t.Errorf("missing required file: %s", f)
				}
			}

			// slides/ directory must exist and contain the expected number of files
			slidesDir := filepath.Join(base, "slides")
			entries, err := os.ReadDir(slidesDir)
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
// contains the correct theme and title, ensuring the deck was scaffolded
// with the intended configuration.
func TestExampleManifest(t *testing.T) {
	root := examplesDir(t)

	for _, deck := range decks {
		t.Run(deck.dir, func(t *testing.T) {
			m, err := scaffold.ReadManifest(filepath.Join(root, deck.dir))
			if err != nil {
				t.Fatalf("ReadManifest failed: %v", err)
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
// the expected number of templar include directives, confirming all slides
// are composed into the presentation.
func TestExampleSlideCount(t *testing.T) {
	root := examplesDir(t)

	for _, deck := range decks {
		t.Run(deck.dir, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, deck.dir, "index.html"))
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
// a self-contained HTML file. Checks that templar includes are resolved,
// CSS and JS are inlined (no external references remain), and the built
// output contains the expected number of slide elements.
func TestExampleBuild(t *testing.T) {
	root := examplesDir(t)

	for _, deck := range decks {
		t.Run(deck.dir, func(t *testing.T) {
			deckDir := filepath.Join(root, deck.dir)
			result, err := builder.Build(deckDir)
			if err != nil {
				t.Fatalf("Build failed: %v", err)
			}

			// All templar includes must be resolved
			if strings.Contains(result.HTML, "{{#") {
				t.Error("built HTML still contains unresolved templar directives")
			}

			// CSS must be inlined (no <link> stylesheet tags)
			if strings.Contains(result.HTML, `<link rel="stylesheet"`) {
				t.Error("built HTML still contains <link> stylesheet tags")
			}
			if !strings.Contains(result.HTML, "<style>") {
				t.Error("built HTML missing inlined <style> tags")
			}

			// JS must be inlined (no <script src> tags)
			if strings.Contains(result.HTML, `<script src=`) {
				t.Error("built HTML still contains <script src> tags")
			}
			if !strings.Contains(result.HTML, "showSlide") {
				t.Error("built HTML missing inlined JS (showSlide function)")
			}

			// Correct number of slide elements in output
			slideCount := strings.Count(result.HTML, `class="slide`)
			if slideCount < deck.slides {
				t.Errorf("expected at least %d slide elements, got %d", deck.slides, slideCount)
			}
		})
	}
}

// TestExampleContentNotPlaceholder verifies that each example's title slide
// contains real authored content rather than the default scaffold placeholder
// text. This ensures examples were actually customized after scaffolding.
func TestExampleContentNotPlaceholder(t *testing.T) {
	root := examplesDir(t)

	for _, deck := range decks {
		t.Run(deck.dir, func(t *testing.T) {
			// Find the title slide (first file in slides/ sorted alphabetically)
			slidesDir := filepath.Join(root, deck.dir, "slides")
			entries, err := os.ReadDir(slidesDir)
			if err != nil {
				t.Fatalf("cannot read slides/: %v", err)
			}
			if len(entries) == 0 {
				t.Fatal("no slide files found")
			}

			titleSlide, err := os.ReadFile(filepath.Join(slidesDir, entries[0].Name()))
			if err != nil {
				t.Fatalf("cannot read title slide: %v", err)
			}
			content := string(titleSlide)

			// Must contain the expected h1 text
			if !strings.Contains(content, deck.h1Text) {
				t.Errorf("title slide missing expected h1 text %q", deck.h1Text)
			}

			// Must NOT contain default scaffold placeholder
			if strings.Contains(content, "Your subtitle here") {
				t.Error("title slide still contains scaffold placeholder text")
			}
		})
	}
}

// TestExampleBuildContainsTimer verifies that each example deck's built
// output includes the presenter timer and reading time features. Checks
// for the toggleTimer function, the notes window timer UI elements, and
// the reading time computation — ensuring the timer feature is present
// in all shipped examples.
func TestExampleBuildContainsTimer(t *testing.T) {
	root := examplesDir(t)

	timerMarkers := []string{
		"toggleTimer",
		"computeReadingTimes",
		"notesTimer",
		"notesTimerToggle",
		"formatReadingTime",
	}

	for _, deck := range decks {
		t.Run(deck.dir, func(t *testing.T) {
			result, err := builder.Build(filepath.Join(root, deck.dir))
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
// contains a speaker-notes section. This ensures presenters always have
// notes available and validates consistent slide authoring.
func TestExampleSpeakerNotes(t *testing.T) {
	root := examplesDir(t)

	for _, deck := range decks {
		t.Run(deck.dir, func(t *testing.T) {
			slidesDir := filepath.Join(root, deck.dir, "slides")
			entries, err := os.ReadDir(slidesDir)
			if err != nil {
				t.Fatalf("cannot read slides/: %v", err)
			}

			for _, entry := range entries {
				data, err := os.ReadFile(filepath.Join(slidesDir, entry.Name()))
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
