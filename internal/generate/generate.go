// Package generate creates slyds presentations from structured JSON input.
// This enables non-technical tools (like Glean) to produce presentations
// without requiring a coding agent — just pipe JSON to `slyds generate`.
package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/panyam/slyds/internal/layout"
	"github.com/panyam/slyds/internal/scaffold"
)

// DeckSpec is the top-level JSON input for generating a presentation.
type DeckSpec struct {
	Title  string      `json:"title"`
	Theme  string      `json:"theme,omitempty"`
	Slides []SlideSpec `json:"slides"`
}

// SlideSpec defines a single slide's content.
// Slots maps slot names (from the layout) to HTML content.
// For convenience, common slot names are also top-level fields:
// "title", "subtitle", "body", "left", "right".
type SlideSpec struct {
	Layout string            `json:"layout,omitempty"`
	Title  string            `json:"title,omitempty"`
	Slots  map[string]string `json:"slots,omitempty"`
	Notes  string            `json:"notes,omitempty"`
}

// Validate checks a DeckSpec for basic correctness.
func (d *DeckSpec) Validate() error {
	if d.Title == "" {
		return fmt.Errorf("deck title is required")
	}
	if len(d.Slides) == 0 {
		return fmt.Errorf("at least one slide is required")
	}
	for i, s := range d.Slides {
		if s.Layout == "" {
			// Default layout
			if i == 0 {
				d.Slides[i].Layout = "title"
			} else if i == len(d.Slides)-1 {
				d.Slides[i].Layout = "closing"
			} else {
				d.Slides[i].Layout = "content"
			}
		}
		if !layout.LayoutExists(d.Slides[i].Layout) {
			layouts, _ := layout.ListLayouts()
			return fmt.Errorf("slide %d: unknown layout %q (available: %s)", i+1, d.Slides[i].Layout, strings.Join(layouts, ", "))
		}
	}
	if d.Theme == "" {
		d.Theme = "default"
	}
	if !scaffold.ThemeExists(d.Theme) {
		themes, _ := scaffold.ListThemes()
		return fmt.Errorf("unknown theme %q (available: %s)", d.Theme, strings.Join(themes, ", "))
	}
	return nil
}

// Generate creates a complete slyds presentation from a DeckSpec.
// It scaffolds the project structure, then populates each slide's
// slot content using goquery (DOM-safe HTML manipulation).
func Generate(spec *DeckSpec, outDir string) error {
	if err := spec.Validate(); err != nil {
		return err
	}

	// Scaffold the base presentation with the right number of slides.
	// We scaffold with 2 (minimum) then replace all slides ourselves.
	dir, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}

	// Create directory structure and engine files via scaffold
	_, err = scaffold.CreateInDir(spec.Title, 2, spec.Theme, outDir)
	if err != nil {
		return fmt.Errorf("scaffold failed: %w", err)
	}

	// Remove the scaffolded slides — we'll create our own
	slidesDir := filepath.Join(dir, "slides")
	entries, err := os.ReadDir(slidesDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		os.Remove(filepath.Join(slidesDir, e.Name()))
	}

	// Generate each slide from the spec
	var slideFiles []string
	for i, slideSpec := range spec.Slides {
		filename := fmt.Sprintf("%02d-%s.html", i+1, slideSlug(slideSpec, i))

		// Render the base layout template
		title := slideSpec.Title
		if title == "" {
			title = spec.Title
		}
		html, err := layout.Render(slideSpec.Layout, map[string]any{
			"Title":  title,
			"Number": i + 1,
		})
		if err != nil {
			return fmt.Errorf("slide %d: failed to render layout %q: %w", i+1, slideSpec.Layout, err)
		}

		// Populate slots with content from the spec
		html, err = populateSlots(html, slideSpec)
		if err != nil {
			return fmt.Errorf("slide %d: failed to populate content: %w", i+1, err)
		}

		slidePath := filepath.Join(slidesDir, filename)
		if err := os.WriteFile(slidePath, []byte(html), 0644); err != nil {
			return err
		}
		slideFiles = append(slideFiles, filename)
	}

	// Rebuild index.html with the correct includes
	if err := rewriteIndex(dir, slideFiles); err != nil {
		return fmt.Errorf("failed to update index.html: %w", err)
	}

	return nil
}

// populateSlots uses goquery to fill data-slot elements with content
// from the SlideSpec. This is DOM-safe — no regex HTML mutation.
func populateSlots(html string, spec SlideSpec) (string, error) {
	// Build the merged slot map: top-level fields + explicit slots
	slots := mergeSlots(spec)

	if len(slots) == 0 && spec.Notes == "" {
		return html, nil
	}

	doc, err := parseFragment(html)
	if err != nil {
		return "", err
	}

	for slotName, content := range slots {
		sel := doc.Find(fmt.Sprintf(`[data-slot="%s"]`, slotName))
		if sel.Length() > 0 {
			sel.SetHtml(content)
		}
	}

	// Handle speaker notes
	if spec.Notes != "" {
		notesSel := doc.Find(".speaker-notes")
		if notesSel.Length() > 0 {
			notesSel.SetHtml(spec.Notes)
		}
	}

	return extractFragment(doc)
}

// mergeSlots combines top-level convenience fields with the explicit Slots map.
// Explicit Slots take precedence over top-level fields.
func mergeSlots(spec SlideSpec) map[string]string {
	slots := make(map[string]string)

	// Top-level convenience fields
	if spec.Title != "" {
		slots["title"] = spec.Title
	}

	// Map common top-level fields from Slots
	// (subtitle, body, left, right are the standard slot names)
	for k, v := range spec.Slots {
		slots[k] = v
	}

	return slots
}

// slideSlug generates a filename slug for a slide.
func slideSlug(spec SlideSpec, index int) string {
	if spec.Title != "" {
		return scaffold.Slugify(spec.Title)
	}
	return fmt.Sprintf("slide-%d", index+1)
}

// rewriteIndex rebuilds the include directives in index.html.
func rewriteIndex(root string, slideFiles []string) error {
	indexPath := filepath.Join(root, "index.html")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var newLines []string
	includeInserted := false
	includeRe := scaffold.IncludeRe()

	for _, line := range lines {
		if includeRe.MatchString(line) {
			if !includeInserted {
				for _, f := range slideFiles {
					newLines = append(newLines, fmt.Sprintf(`    {{# include "slides/%s" #}}`, f))
				}
				includeInserted = true
			}
			continue
		}
		newLines = append(newLines, line)
	}

	return os.WriteFile(indexPath, []byte(strings.Join(newLines, "\n")), 0644)
}

// parseFragment parses an HTML fragment without adding <html><head><body> wrappers.
func parseFragment(content string) (*goquery.Document, error) {
	wrapped := `<div id="__slyds_wrapper__">` + content + `</div>`
	return goquery.NewDocumentFromReader(strings.NewReader(wrapped))
}

// extractFragment extracts the inner HTML of the synthetic wrapper.
func extractFragment(doc *goquery.Document) (string, error) {
	return doc.Find("#__slyds_wrapper__").Html()
}
