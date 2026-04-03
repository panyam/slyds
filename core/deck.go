// Package core provides the slyds presentation library API.
// This is the primary interface for programmatic access to slyds decks —
// used by both the CLI (cmd/) and MCP tool handlers.
//
// All file operations go through the DeckFS interface, making decks portable
// across local filesystems, S3, IndexedDB (WASM), in-memory (tests), etc.
package core

import (
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var includeRe = regexp.MustCompile(`\{\{#\s*include\s+"(slides/[^"]+)"\s*#\}\}`)

// DeckFS is the filesystem interface for deck storage.
// It extends fs.FS (read) with write operations, making decks portable
// across local disk, S3, IndexedDB (WASM), in-memory (tests), etc.
type DeckFS interface {
	fs.FS

	// WriteFile writes data to the named file, creating it if necessary.
	WriteFile(name string, data []byte, perm fs.FileMode) error

	// MkdirAll creates a directory path and all parents.
	MkdirAll(path string, perm fs.FileMode) error

	// Remove deletes the named file or empty directory.
	Remove(name string) error

	// Rename renames (moves) a file.
	Rename(oldname, newname string) error
}

// DeckManifest holds the parsed .slyds.yaml configuration.
// This is the core subset of the manifest — the full scaffold.Manifest
// type in internal/scaffold adds source management fields.
type DeckManifest struct {
	Theme string `yaml:"theme" json:"theme"`
	Title string `yaml:"title" json:"title"`
}

// Deck represents an opened slyds presentation deck.
// All I/O goes through the FS field, making it portable across backends.
type Deck struct {
	// FS is the filesystem backing this deck.
	FS DeckFS

	// Manifest is the parsed .slyds.yaml configuration. May be nil if none exists.
	Manifest *DeckManifest
}

// OpenDeck opens an existing deck from the given filesystem.
// The FS root should be the deck directory (containing index.html).
func OpenDeck(fsys DeckFS) (*Deck, error) {
	// Verify index.html exists
	if _, err := readFile(fsys, "index.html"); err != nil {
		return nil, fmt.Errorf("no index.html found — is this a slyds presentation?")
	}

	// Read manifest (optional)
	manifest := readManifest(fsys)

	return &Deck{FS: fsys, Manifest: manifest}, nil
}

// OpenDeckDir is a convenience for opening a deck from a local directory.
func OpenDeckDir(dir string) (*Deck, error) {
	return OpenDeck(NewOSFS(dir))
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
	data, err := readFile(d.FS, "index.html")
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
	data, err := readFile(d.FS, "slides/"+files[position-1])
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

// --- Internal helpers ---

// listSlideFiles returns slide filenames from the filesystem, sorted alphabetically.
func (d *Deck) listSlideFiles() []string {
	entries, err := readDir(d.FS, "slides")
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

// readFile reads from DeckFS, preferring ReadFile interface when available.
func readFile(fsys DeckFS, name string) ([]byte, error) {
	if rf, ok := fsys.(interface{ ReadFile(string) ([]byte, error) }); ok {
		return rf.ReadFile(name)
	}
	return fs.ReadFile(fsys, name)
}

// readDir reads from DeckFS, preferring ReadDir interface when available.
func readDir(fsys DeckFS, name string) ([]fs.DirEntry, error) {
	if rd, ok := fsys.(interface{ ReadDir(string) ([]fs.DirEntry, error) }); ok {
		return rd.ReadDir(name)
	}
	return fs.ReadDir(fsys, name)
}

// readManifest reads and parses .slyds.yaml. Returns nil if not found.
func readManifest(fsys DeckFS) *DeckManifest {
	data, err := readFile(fsys, ".slyds.yaml")
	if err != nil {
		return nil
	}
	var m DeckManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil
	}
	return &m
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
