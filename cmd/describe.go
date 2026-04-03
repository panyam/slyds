package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/panyam/slyds/core"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var describeJSON bool

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
type SlideDescription struct {
	Position int    `yaml:"position" json:"position"`
	File     string `yaml:"file" json:"file"`
	Layout   string `yaml:"layout" json:"layout"`
	Title    string `yaml:"title" json:"title"`
	Words    int    `yaml:"words" json:"words"`
	HasNotes bool   `yaml:"has_notes" json:"has_notes"`
	Images   int    `yaml:"images" json:"images"`
}

var describeCmd = &cobra.Command{
	Use:   "describe [dir]",
	Short: "Output a structured summary of the presentation deck",
	Long: `Describe outputs a YAML (default) or JSON summary of the deck including
slide count, layouts used, per-slide metadata (title, layout, word count,
speaker notes presence), available themes and layouts.

Designed for LLM consumption — provides efficient context about a deck
without reading every slide file.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		root, err := findRootIn(dir)
		if err != nil {
			return err
		}

		desc, err := describeDeck(root)
		if err != nil {
			return err
		}

		if describeJSON {
			data, err := json.MarshalIndent(desc, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		data, err := yaml.Marshal(desc)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	},
}

// describeDeck builds a structured description of the deck at root.
func describeDeck(root string) (*DeckDescription, error) {
	manifest, err := core.ReadManifest(root)
	if err != nil && err != core.ErrManifestNotFound {
		return nil, err
	}

	title := ""
	theme := "default"
	if manifest != nil {
		title = manifest.Title
		theme = manifest.Theme
	}

	slides, err := listSlidesFromIndex(root)
	if err != nil {
		return nil, err
	}

	layoutSet := map[string]bool{}
	var slideDescs []SlideDescription

	tagRe := regexp.MustCompile(`<[^>]+>`)
	imgRe := regexp.MustCompile(`<img\b`)
	notesRe := regexp.MustCompile(`class="speaker-notes"`)

	for i, f := range slides {
		slidePath := filepath.Join(root, "slides", f)
		data, _ := os.ReadFile(slidePath)
		content := string(data)

		slideLayout := core.DetectLayout(content)
		layoutSet[slideLayout] = true

		slideTitle := extractFirstHeading(slidePath)

		// Count words in visible content (exclude speaker notes and HTML tags)
		textContent := content
		if idx := strings.Index(textContent, `class="speaker-notes"`); idx >= 0 {
			textContent = textContent[:idx]
		}
		textContent = tagRe.ReplaceAllString(textContent, " ")
		wordCount := 0
		for _, w := range strings.Fields(textContent) {
			if len(w) > 0 {
				wordCount++
			}
		}

		images := len(imgRe.FindAllString(content, -1))
		hasNotes := notesRe.MatchString(content)

		slideDescs = append(slideDescs, SlideDescription{
			Position: i + 1,
			File:     f,
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
	sortStrings(layoutsUsed)

	themes := availableThemeNames()
	layouts, _ := core.ListLayouts()

	return &DeckDescription{
		Title:            title,
		Theme:            theme,
		SlideCount:       len(slides),
		LayoutsUsed:      layoutsUsed,
		Slides:           slideDescs,
		ThemesAvailable:  themes,
		LayoutsAvailable: layouts,
	}, nil
}

// availableThemeNames returns theme names from the embedded themes directory.
func availableThemeNames() []string {
	files := core.ThemeFileNames()
	var names []string
	for _, f := range files {
		if f == "_base.css" {
			continue
		}
		names = append(names, strings.TrimSuffix(f, ".css"))
	}
	return names
}

// sortStrings sorts a slice of strings in place.
func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

func init() {
	describeCmd.Flags().BoolVar(&describeJSON, "json", false, "output as JSON instead of YAML")
	rootCmd.AddCommand(describeCmd)
}
