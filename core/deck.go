// Package core provides the slyds presentation library API.
// This is the primary interface for programmatic access to slyds decks —
// used by both the CLI (cmd/) and MCP tool handlers.
//
// All file operations go through templar.WritableFS, making decks portable
// across local filesystems, S3, IndexedDB (WASM), in-memory (tests), etc.
package core

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/panyam/templar"
	"gopkg.in/yaml.v3"
)

var includeRe = regexp.MustCompile(`\{\{#\s*include\s+"(slides/[^"]+)"\s*#\}\}`)

// DeckManifest holds the parsed .slyds.yaml configuration.
type DeckManifest struct {
	Theme string `yaml:"theme" json:"theme"`
	Title string `yaml:"title" json:"title"`
}

// Deck represents an opened slyds presentation deck.
// All I/O goes through the FS field (templar.WritableFS), making it portable.
type Deck struct {
	// FS is the writable filesystem backing this deck.
	FS templar.WritableFS

	// Manifest is the parsed .slyds.yaml configuration. May be nil if none exists.
	Manifest *DeckManifest
}

// OpenDeck opens an existing deck from the given filesystem.
// The FS root should be the deck directory (containing index.html).
func OpenDeck(fsys templar.WritableFS) (*Deck, error) {
	if _, err := fsys.ReadFile("index.html"); err != nil {
		return nil, fmt.Errorf("no index.html found — is this a slyds presentation?")
	}
	manifest := readManifest(fsys)
	return &Deck{FS: fsys, Manifest: manifest}, nil
}

// Title returns the deck's title from the manifest, or "" if unknown.
func (d *Deck) Title() string {
	if d.Manifest != nil {
		return d.Manifest.Title
	}
	return ""
}

// Theme returns the deck's theme name, or "default" if not set.
func (d *Deck) Theme() string {
	if d.Manifest != nil && d.Manifest.Theme != "" {
		return d.Manifest.Theme
	}
	return "default"
}

// SlideFilenames returns the ordered list of slide filenames from index.html.
// Falls back to alphabetical filesystem listing if no includes are found.
func (d *Deck) SlideFilenames() ([]string, error) {
	data, err := d.FS.ReadFile( "index.html")
	if err != nil {
		return nil, err
	}

	var slides []string
	for _, line := range strings.Split(string(data), "\n") {
		if m := includeRe.FindStringSubmatch(line); m != nil {
			name := strings.TrimPrefix(m[1], "slides/")
			slides = append(slides, name)
		}
	}

	if len(slides) == 0 {
		return d.listSlideFiles(), nil
	}
	return slides, nil
}

// SlideCount returns the number of slides in the deck.
func (d *Deck) SlideCount() (int, error) {
	files, err := d.SlideFilenames()
	if err != nil {
		return 0, err
	}
	return len(files), nil
}

// GetSlideContent reads the raw HTML content of a slide by 1-based position.
func (d *Deck) GetSlideContent(position int) (string, error) {
	files, err := d.SlideFilenames()
	if err != nil {
		return "", err
	}
	if position < 1 || position > len(files) {
		return "", fmt.Errorf("slide %d out of range (have %d slides)", position, len(files))
	}
	data, err := d.FS.ReadFile( "slides/"+files[position-1])
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// EditSlideContent writes new HTML content to a slide by 1-based position.
func (d *Deck) EditSlideContent(position int, content string) error {
	files, err := d.SlideFilenames()
	if err != nil {
		return err
	}
	if position < 1 || position > len(files) {
		return fmt.Errorf("slide %d out of range (have %d slides)", position, len(files))
	}
	return d.FS.WriteFile("slides/"+files[position-1], []byte(content), 0644)
}

// RewriteSlideOrder renumbers slide files and rebuilds index.html to match
// the given ordering. Files are renamed via temp names to avoid collisions.
func (d *Deck) RewriteSlideOrder(orderedFiles []string) error {
	type renamePair struct{ from, to string }
	var renames []renamePair

	for i, oldName := range orderedFiles {
		namePart := ExtractNamePart(oldName)
		newName := fmt.Sprintf("%02d-%s", i+1, namePart)
		if oldName != newName {
			renames = append(renames, renamePair{oldName, newName})
		}
		orderedFiles[i] = newName
	}

	for _, r := range renames {
		d.FS.Rename("slides/"+r.from, "slides/"+r.from+".tmp")
	}
	for _, r := range renames {
		d.FS.Rename("slides/"+r.from+".tmp", "slides/"+r.to)
	}

	indexData, err := d.FS.ReadFile( "index.html")
	if err != nil {
		return err
	}

	lines := strings.Split(string(indexData), "\n")
	var newLines []string
	includeInserted := false

	for _, line := range lines {
		if includeRe.MatchString(line) {
			if !includeInserted {
				for _, f := range orderedFiles {
					newLines = append(newLines, fmt.Sprintf(`    {{# include "slides/%s" #}}`, f))
				}
				includeInserted = true
			}
			continue
		}
		newLines = append(newLines, line)
	}

	return d.FS.WriteFile("index.html", []byte(strings.Join(newLines, "\n")), 0644)
}

// AddSlide creates a new slide file at the given position with the given content,
// inserts it into the ordering, and renumbers all slides.
func (d *Deck) AddSlide(position int, filename string, content string) error {
	existing, err := d.SlideFilenames()
	if err != nil {
		return err
	}
	if position < 1 || position > len(existing)+1 {
		return fmt.Errorf("position %d out of range (have %d slides, valid range 1-%d)", position, len(existing), len(existing)+1)
	}

	d.FS.MkdirAll("slides", 0755)
	if err := d.FS.WriteFile("slides/"+filename, []byte(content), 0644); err != nil {
		return err
	}

	newOrder := make([]string, 0, len(existing)+1)
	newOrder = append(newOrder, existing[:position-1]...)
	newOrder = append(newOrder, filename)
	newOrder = append(newOrder, existing[position-1:]...)

	return d.RewriteSlideOrder(newOrder)
}

// RemoveSlide removes a slide by filename, renumbers remaining slides.
func (d *Deck) RemoveSlide(filename string) error {
	existing, err := d.SlideFilenames()
	if err != nil {
		return err
	}
	if err := d.FS.Remove("slides/" + filename); err != nil {
		return err
	}
	var remaining []string
	for _, f := range existing {
		if f != filename {
			remaining = append(remaining, f)
		}
	}
	return d.RewriteSlideOrder(remaining)
}

// MoveSlide reorders a slide from one position to another (1-based).
func (d *Deck) MoveSlide(from, to int) error {
	existing, err := d.SlideFilenames()
	if err != nil {
		return err
	}
	if from < 1 || from > len(existing) || to < 1 || to > len(existing) {
		return fmt.Errorf("slide numbers out of range (have %d slides)", len(existing))
	}

	item := existing[from-1]
	existing = append(existing[:from-1], existing[from:]...)
	if to-1 >= len(existing) {
		existing = append(existing, item)
	} else {
		existing = append(existing[:to-1], append([]string{item}, existing[to-1:]...)...)
	}

	return d.RewriteSlideOrder(existing)
}

// SlugifySlides renames slides based on their <h1> headings.
// The slugFn parameter converts a heading string to a slug.
// Returns the number of slides renamed.
func (d *Deck) SlugifySlides(slugFn func(string) string) (int, error) {
	slides, err := d.SlideFilenames()
	if err != nil {
		return 0, err
	}

	usedSlugs := make(map[string]int)
	newNames := make([]string, len(slides))
	renamed := 0

	for i, filename := range slides {
		content, _ := d.GetSlideContent(i + 1)
		heading := ExtractFirstHeading(content)
		if heading == "" {
			newNames[i] = filename
			slug := strings.TrimSuffix(ExtractNamePart(filename), ".html")
			usedSlugs[slug]++
			continue
		}

		slug := slugFn(heading)
		usedSlugs[slug]++
		if usedSlugs[slug] > 1 {
			slug = fmt.Sprintf("%s-%d", slug, usedSlugs[slug])
		}

		newName := fmt.Sprintf("%02d-%s.html", i+1, slug)
		newNames[i] = newName
		if newName != filename {
			renamed++
		}
	}

	if renamed == 0 {
		return 0, nil
	}

	type renamePair struct{ from, to string }
	var renames []renamePair
	for i, oldName := range slides {
		if newNames[i] != oldName {
			renames = append(renames, renamePair{oldName, newNames[i]})
		}
	}
	for _, r := range renames {
		d.FS.Rename("slides/"+r.from, "slides/"+r.from+".tmp")
	}
	for _, r := range renames {
		d.FS.Rename("slides/"+r.from+".tmp", "slides/"+r.to)
	}

	return renamed, d.RewriteSlideOrder(newNames)
}

// InsertSlide renders a new slide from the named layout template and inserts
// it at the given position. The name is used to generate the filename slug.
// If a slide with the same slug already exists in the deck, the slug is
// auto-suffixed with -2, -3, ... (same convention as SlugifySlides) to keep
// slugs unique within a deck.
//
// Returns the final slug actually used (may differ from the requested name
// if auto-suffix was applied) and any error encountered.
//
// Title overrides the display title (otherwise derived from the slug).
//
// TODO(#83): also return assigned slide_id once .slyds.yaml stores per-slide
// metadata — gives callers a rename-safe handle separate from the slug.
func (d *Deck) InsertSlide(position int, name, layoutName, title string) (string, error) {
	finalName := d.uniqueSlug(name)

	displayName := slugToTitle(finalName)
	if title != "" {
		displayName = title
	}

	content, err := Render(layoutName, map[string]any{
		"Title":  displayName,
		"Number": position,
	})
	if err != nil {
		return "", fmt.Errorf("layout %q: %w", layoutName, err)
	}

	filename := fmt.Sprintf("%02d-%s.html", position, finalName)
	if err := d.AddSlide(position, filename, content); err != nil {
		return "", err
	}
	return finalName, nil
}

// uniqueSlug returns the desired slug if no existing slide uses it, otherwise
// appends -2, -3, ... until an unused slug is found. Uses the same convention
// as SlugifySlides so the two creation paths stay consistent. A collision-free
// name is always returned — the loop terminates because slide counts are
// bounded by the filesystem.
func (d *Deck) uniqueSlug(desired string) string {
	used := map[string]bool{}
	slides, _ := d.SlideFilenames()
	for _, f := range slides {
		used[strings.TrimSuffix(ExtractNamePart(f), ".html")] = true
	}
	if !used[desired] {
		return desired
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", desired, i)
		if !used[candidate] {
			return candidate
		}
	}
}

// ApplySlots sets inner HTML for named layout slots on a slide.
// Each key in slots is a slot name (matching data-slot="name"), value is the HTML.
func (d *Deck) ApplySlots(position int, slots map[string]string) error {
	ref := fmt.Sprintf("%d", position)
	for slot, html := range slots {
		h := html
		sel := `[data-slot="` + strings.ReplaceAll(slot, `"`, `\"`) + `"]`
		if _, err := d.Query(ref, sel, QueryOpts{SetHTML: &h}); err != nil {
			return fmt.Errorf("slot %q: %w", slot, err)
		}
	}
	return nil
}

// slugToTitle converts a slug like "my-demo" to a display title "My Demo".
func slugToTitle(name string) string {
	words := strings.Fields(strings.ReplaceAll(name, "-", " "))
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// ExtractNamePart strips the numeric prefix (e.g., "01-") from a slide filename.
func ExtractNamePart(filename string) string {
	re := regexp.MustCompile(`^(\d+)-(.+)$`)
	if m := re.FindStringSubmatch(filename); m != nil {
		return m[2]
	}
	return filename
}

// ExtractFirstHeading returns the text content of the first <h1> in an HTML string.
func ExtractFirstHeading(html string) string {
	re := regexp.MustCompile(`<h1[^>]*>(.*?)</h1>`)
	m := re.FindStringSubmatch(html)
	if m != nil {
		return m[1]
	}
	return ""
}

// --- Internal helpers ---

func (d *Deck) listSlideFiles() []string {
	entries, err := d.FS.ReadDir( "slides")
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".html") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files
}

func readManifest(fsys templar.WritableFS) *DeckManifest {
	data, err := fsys.ReadFile(".slyds.yaml")
	if err != nil {
		return nil
	}
	var m DeckManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil
	}
	return &m
}
