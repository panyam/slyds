package core

import (
	"regexp"
	"sort"
	"strings"
)

// DeckDescription holds the structured description of a presentation deck.
type DeckDescription struct {
	Title            string             `yaml:"title" json:"title"`
	Theme            string             `yaml:"theme" json:"theme"`
	SlideCount       int                `yaml:"slide_count" json:"slide_count"`
	DeckVersion      string             `yaml:"deck_version" json:"deck_version"`
	LayoutsUsed      []string           `yaml:"layouts_used" json:"layouts_used"`
	Slides           []SlideDescription `yaml:"slides" json:"slides"`
	ThemesAvailable  []string           `yaml:"themes_available" json:"themes_available"`
	LayoutsAvailable []string           `yaml:"layouts_available" json:"layouts_available"`
}

// SlideDescription holds metadata for a single slide.
//
// SlideDescription holds metadata for a single slide.
//
// Three overlapping identifiers are provided so agents can choose the
// level of stability they need:
//
//   - Position: mutable — changes on insert/remove/move
//   - Slug: stable across position shifts, but NOT across renames
//   - SlideID: stable across everything including renames — the truly
//     immutable handle assigned by slyds and stored in .slyds.yaml.
//     Empty for legacy decks that haven't been mutated since #83.
type SlideDescription struct {
	Position int    `yaml:"position" json:"position"`
	File     string `yaml:"file" json:"file"`
	SlideID  string `yaml:"slide_id" json:"slide_id"`
	Slug     string `yaml:"slug" json:"slug"`
	Layout   string `yaml:"layout" json:"layout"`
	Title    string `yaml:"title" json:"title"`
	Words    int    `yaml:"words" json:"words"`
	HasNotes bool   `yaml:"has_notes" json:"has_notes"`
	Images  int    `yaml:"images" json:"images"`
	Version string `yaml:"version" json:"version"`
}

var descTagRe = regexp.MustCompile(`<[^>]+>`)
var descImgRe = regexp.MustCompile(`<img\b`)
var descNotesRe = regexp.MustCompile(`class="speaker-notes"`)

// Describe builds a structured description of the deck: title, theme,
// slide metadata (layout, title, word count, speaker notes, images),
// available themes and layouts. All reads go through DeckFS.
func (d *Deck) Describe() (*DeckDescription, error) {
	slides, err := d.SlideFilenames()
	if err != nil {
		return nil, err
	}

	layoutSet := map[string]bool{}
	var slideDescs []SlideDescription

	for i, f := range slides {
		content, _ := d.GetSlideContent(i + 1)

		slideLayout := DetectLayout(content)
		layoutSet[slideLayout] = true

		slideTitle := ExtractFirstHeading(content)

		// Word count (exclude speaker notes and HTML tags)
		textContent := content
		if idx := strings.Index(textContent, `class="speaker-notes"`); idx >= 0 {
			textContent = textContent[:idx]
		}
		textContent = descTagRe.ReplaceAllString(textContent, " ")
		wordCount := len(strings.Fields(textContent))

		images := len(descImgRe.FindAllString(content, -1))
		hasNotes := descNotesRe.MatchString(content)

		slideVer, _ := d.SlideVersion(i + 1)

		slideDescs = append(slideDescs, SlideDescription{
			Position: i + 1,
			File:     f,
			SlideID:  d.SlideIDForFile(f),
			Slug:     slideSlugFromFile(f),
			Layout:   slideLayout,
			Title:    slideTitle,
			Words:    wordCount,
			HasNotes: hasNotes,
			Images:   images,
			Version:  slideVer,
		})
	}

	var layoutsUsed []string
	for l := range layoutSet {
		layoutsUsed = append(layoutsUsed, l)
	}
	sort.Strings(layoutsUsed)

	themes := AvailableThemeNames()
	layouts, _ := ListLayouts()
	deckVer, _ := d.DeckVersion()

	return &DeckDescription{
		Title:            d.Title(),
		Theme:            d.Theme(),
		SlideCount:       len(slides),
		DeckVersion:      deckVer,
		LayoutsUsed:      layoutsUsed,
		Slides:           slideDescs,
		ThemesAvailable:  themes,
		LayoutsAvailable: layouts,
	}, nil
}

// AvailableThemeNames returns the list of built-in theme names.
func AvailableThemeNames() []string {
	files := ThemeFileNames()
	var names []string
	for _, f := range files {
		if f == "_base.css" {
			continue
		}
		names = append(names, strings.TrimSuffix(f, ".css"))
	}
	return names
}
