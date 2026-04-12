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

// Deck represents an opened slyds presentation deck.
// All I/O goes through the FS field (templar.WritableFS), making it portable.
//
// Deck is NOT safe for concurrent use. Mutation methods assume a single
// caller at a time; hosted multi-tenant deployments (#76) will need a
// concurrency layer above this.
type Deck struct {
	// FS is the writable filesystem backing this deck.
	FS templar.WritableFS

	// Manifest is the parsed .slyds.yaml configuration. May be nil if
	// none exists on disk yet. Mutation methods that touch the slide set
	// populate and persist it on demand.
	Manifest *Manifest

	// Derived id→file and file→id indices, rebuilt from Manifest.Slides
	// on OpenDeck and after every mutation. Not persisted, not exported:
	// ResolveSlide and the mutation methods use them for O(1) lookups
	// instead of linearly scanning Manifest.Slides on every call. See
	// rebuildIndices and ensureSlideIDs in slide_ids.go.
	idByFile map[string]string
	fileByID map[string]string
}

// OpenDeck opens an existing deck from the given filesystem.
// The FS root should be the deck directory (containing index.html).
//
// The manifest's Slides records (if any) are loaded into derived
// id→file indices for O(1) lookup. Read-only operations do NOT
// auto-migrate legacy decks — that only happens on the first mutation
// via ensureSlideIDs.
func OpenDeck(fsys templar.WritableFS) (*Deck, error) {
	if _, err := fsys.ReadFile("index.html"); err != nil {
		return nil, fmt.Errorf("no index.html found — is this a slyds presentation?")
	}
	manifest := readManifest(fsys)
	d := &Deck{FS: fsys, Manifest: manifest}
	d.rebuildIndices()
	return d, nil
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
// After the rename pass, the slide_id mapping in the manifest is updated
// to reflect the new filenames and persisted to .slyds.yaml.
func (d *Deck) RewriteSlideOrder(orderedFiles []string) error {
	type renamePair struct{ from, to string }
	var renames []renamePair
	renameMap := make(map[string]string) // oldName → newName

	for i, oldName := range orderedFiles {
		namePart := ExtractNamePart(oldName)
		newName := fmt.Sprintf("%02d-%s", i+1, namePart)
		if oldName != newName {
			renames = append(renames, renamePair{oldName, newName})
			renameMap[oldName] = newName
		}
		orderedFiles[i] = newName
	}

	for _, r := range renames {
		d.FS.Rename("slides/"+r.from, "slides/"+r.from+".tmp")
	}
	for _, r := range renames {
		d.FS.Rename("slides/"+r.from+".tmp", "slides/"+r.to)
	}

	indexData, err := d.FS.ReadFile("index.html")
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

	if err := d.FS.WriteFile("index.html", []byte(strings.Join(newLines, "\n")), 0644); err != nil {
		return err
	}

	// Update slide_id mapping for any renamed files and persist.
	if len(renameMap) > 0 && d.Manifest != nil && len(d.Manifest.Slides) > 0 {
		d.updateSlideFilenames(renameMap)
		return d.saveManifest()
	}
	return nil
}

// AddSlide creates a new slide file at the given position with the given content,
// inserts it into the ordering, assigns a slide_id, and renumbers all slides.
func (d *Deck) AddSlide(position int, filename string, content string) error {
	if err := d.ensureSlideIDs(); err != nil {
		return err
	}

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

	// Assign a slide_id to the new file before RewriteSlideOrder renames it.
	usedIDs := make(map[string]bool, len(d.Manifest.Slides))
	for _, rec := range d.Manifest.Slides {
		usedIDs[rec.ID] = true
	}
	newID := uniqueSlideID(usedIDs)
	d.Manifest.Slides = append(d.Manifest.Slides, SlideRecord{ID: newID, File: filename})
	d.rebuildIndices()

	newOrder := make([]string, 0, len(existing)+1)
	newOrder = append(newOrder, existing[:position-1]...)
	newOrder = append(newOrder, filename)
	newOrder = append(newOrder, existing[position-1:]...)

	return d.RewriteSlideOrder(newOrder)
}

// RemoveSlide removes a slide by filename, drops its slide_id record,
// and renumbers remaining slides.
func (d *Deck) RemoveSlide(filename string) error {
	if err := d.ensureSlideIDs(); err != nil {
		return err
	}

	existing, err := d.SlideFilenames()
	if err != nil {
		return err
	}
	if err := d.FS.Remove("slides/" + filename); err != nil {
		return err
	}

	// Remove the slide_id record for the deleted file.
	kept := d.Manifest.Slides[:0]
	for _, rec := range d.Manifest.Slides {
		if rec.File != filename {
			kept = append(kept, rec)
		}
	}
	d.Manifest.Slides = kept
	d.rebuildIndices()

	var remaining []string
	for _, f := range existing {
		if f != filename {
			remaining = append(remaining, f)
		}
	}
	return d.RewriteSlideOrder(remaining)
}

// MoveSlide reorders a slide from one position to another (1-based).
// Slide IDs are preserved — only positions and filename prefixes change.
func (d *Deck) MoveSlide(from, to int) error {
	if err := d.ensureSlideIDs(); err != nil {
		return err
	}

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
// Returns the number of slides renamed. Slide IDs are preserved
// across the rename — the id→file mapping is updated to reflect
// the new filenames.
func (d *Deck) SlugifySlides(slugFn func(string) string) (int, error) {
	if err := d.ensureSlideIDs(); err != nil {
		return 0, err
	}

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
			slug := slideSlugFromFile(filename)
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

	// Build the rename map for the slug portion. RewriteSlideOrder will
	// handle the numeric-prefix portion and update the manifest mapping.
	slugRenames := make(map[string]string)
	type renamePair struct{ from, to string }
	var renames []renamePair
	for i, oldName := range slides {
		if newNames[i] != oldName {
			renames = append(renames, renamePair{oldName, newNames[i]})
			slugRenames[oldName] = newNames[i]
		}
	}

	// Update the manifest mapping for slug changes BEFORE the filesystem
	// rename, so RewriteSlideOrder sees the new filenames.
	d.updateSlideFilenames(slugRenames)

	for _, r := range renames {
		d.FS.Rename("slides/"+r.from, "slides/"+r.from+".tmp")
	}
	for _, r := range renames {
		d.FS.Rename("slides/"+r.from+".tmp", "slides/"+r.to)
	}

	if err := d.RewriteSlideOrder(newNames); err != nil {
		return renamed, err
	}
	// RewriteSlideOrder only saves the manifest when it renames files
	// (NN- prefix changes). After SlugifySlides' own rename pass, the
	// prefixes are already correct, so RewriteSlideOrder skips the save.
	// We must save explicitly here to persist the slug→file mapping
	// changes that updateSlideFilenames applied in memory.
	return renamed, d.saveManifest()
}

// InsertSlide renders a new slide from the named layout template and inserts
// it at the given position. The name is used to generate the filename slug.
// If a slide with the same slug already exists in the deck, the slug is
// auto-suffixed with -2, -3, ... (same convention as SlugifySlides) to keep
// slugs unique within a deck.
//
// Returns the final slug, the assigned slide_id, and any error. The
// slide_id is the rename-safe handle for this slide — it survives every
// future mutation including renames. Agents should prefer slide_id for
// follow-up references when they plan to cache the identifier.
//
// Title overrides the display title (otherwise derived from the slug).
func (d *Deck) InsertSlide(position int, name, layoutName, title string) (string, string, error) {
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
		return "", "", fmt.Errorf("layout %q: %w", layoutName, err)
	}

	filename := fmt.Sprintf("%02d-%s.html", position, finalName)
	if err := d.AddSlide(position, filename, content); err != nil {
		return "", "", err
	}

	// After AddSlide, the file may have been renumbered. Look up the
	// slide_id from the derived indices (AddSlide already assigned one
	// and called RewriteSlideOrder which updated the mapping).
	slideID := ""
	for _, rec := range d.Manifest.Slides {
		if slideSlugFromFile(rec.File) == finalName {
			slideID = rec.ID
			break
		}
	}
	return finalName, slideID, nil
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

// readManifest reads and parses .slyds.yaml into the full Manifest type.
// Returns nil on any error (missing file, malformed yaml) — callers treat
// a nil manifest as "legacy deck with no config" and degrade gracefully.
func readManifest(fsys templar.WritableFS) *Manifest {
	data, err := fsys.ReadFile(".slyds.yaml")
	if err != nil {
		return nil
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil
	}
	return &m
}
