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
	LayoutsUsed      []string           `yaml:"layouts_used" json:"layouts_used"`
	Slides           []SlideDescription `yaml:"slides" json:"slides"`
	ThemesAvailable  []string           `yaml:"themes_available" json:"themes_available"`
	LayoutsAvailable []string           `yaml:"layouts_available" json:"layouts_available"`
}

// SlideDescription holds metadata for a single slide.
//
// Slug is the stable, human-readable identifier for a slide within a deck.
// It's derived from the filename by stripping the NN- prefix and .html
// suffix. Stable across inserts/removes/moves (because RewriteSlideOrder
// preserves the slug portion when renumbering) but NOT across renames
// (slugify, manual rename). Agents should prefer Slug over Position when
// referencing a slide across multiple MCP calls.
//
// TODO(#83): add SlideID field once .slyds.yaml stores per-slide metadata.
// SlideID will be rename-safe where Slug is not.
type SlideDescription struct {
	Position int    `yaml:"position" json:"position"`
	File     string `yaml:"file" json:"file"`
	Slug     string `yaml:"slug" json:"slug"`
	Layout   string `yaml:"layout" json:"layout"`
	Title    string `yaml:"title" json:"title"`
	Words    int    `yaml:"words" json:"words"`
	HasNotes bool   `yaml:"has_notes" json:"has_notes"`
	Images   int    `yaml:"images" json:"images"`
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

		slideDescs = append(slideDescs, SlideDescription{
			Position: i + 1,
			File:     f,
			Slug:     strings.TrimSuffix(ExtractNamePart(f), ".html"),
			Layout:   slideLayout,
			Title:    slideTitle,
			Words:    wordCount,
			HasNotes: hasNotes,
			Images:   images,
		})
	}

	var layoutsUsed []string
	for l := range layoutSet {
		layoutsUsed = append(layoutsUsed, l)
	}
	sort.Strings(layoutsUsed)

	themes := AvailableThemeNames()
	layouts, _ := ListLayouts()

	return &DeckDescription{
		Title:            d.Title(),
		Theme:            d.Theme(),
		SlideCount:       len(slides),
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
