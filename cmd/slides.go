package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/panyam/slyds/core"
	"github.com/panyam/slyds/internal/layout"
	"github.com/panyam/slyds/internal/scaffold"
	"github.com/spf13/cobra"
)

var (
	slideAfter  int
	slideType   string
	slideLayout string
	insertType  string
	insertLayout string
	insertTitle string
	slotsFileAdd string
	slotsFileInsert string
)

var includeRe = regexp.MustCompile(`\{\{#\s*include\s+"(slides/[^"]+)"\s*#\}\}`)
var numPrefixRe = regexp.MustCompile(`^(\d+)-(.+)$`)

var addCmd = &cobra.Command{
	Use:   `add "name"`,
	Short: "Add a new slide",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := scaffold.Slugify(args[0])
		root, err := findRoot()
		if err != nil {
			return err
		}

		existing, err := listSlidesFromIndex(root)
		if err != nil {
			return err
		}

		// Determine insert position
		position := len(existing) + 1 // default: append at end
		if slideAfter > 0 {
			if slideAfter > len(existing) {
				return fmt.Errorf("--after %d is out of range (have %d slides)", slideAfter, len(existing))
			}
			position = slideAfter + 1
		}

		layoutName := resolveLayoutFlag(slideLayout, slideType)
		if err := runInsert(root, position, name, layoutName, ""); err != nil {
			return err
		}

		if slotsFileAdd != "" {
			if err := applySlotsFile(root, position, slotsFileAdd); err != nil {
				return err
			}
		}

		slides, _ := listSlidesFromIndex(root)
		fmt.Printf("Added slide: slides/%s\n", slides[position-1])
		return nil
	},
}

var rmCmd = &cobra.Command{
	Use:   "rm <name-or-number>",
	Short: "Remove a slide",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}

		target := args[0]
		existing, err := listSlidesFromIndex(root)
		if err != nil {
			return err
		}

		var slideFile string
		// Try as number first
		if num, err := strconv.Atoi(target); err == nil {
			if num < 1 || num > len(existing) {
				return fmt.Errorf("slide %d out of range (have %d slides)", num, len(existing))
			}
			slideFile = existing[num-1]
		} else {
			// Try as name match
			for _, f := range existing {
				if strings.Contains(f, target) {
					slideFile = f
					break
				}
			}
		}

		if slideFile == "" {
			return fmt.Errorf("slide %q not found", target)
		}

		// Remove the file
		slidePath := filepath.Join(root, "slides", slideFile)
		if err := os.Remove(slidePath); err != nil && !os.IsNotExist(err) {
			return err
		}

		// Remove from ordering and renumber
		var remaining []string
		for _, f := range existing {
			if f != slideFile {
				remaining = append(remaining, f)
			}
		}

		if err := rewriteSlidesAndIndex(root, remaining); err != nil {
			return err
		}

		fmt.Printf("Removed slide: slides/%s\n", slideFile)
		return nil
	},
}

var mvCmd = &cobra.Command{
	Use:   "mv <from> <to>",
	Short: "Move/reorder a slide",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}

		from, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("from must be a slide number: %w", err)
		}
		to, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("to must be a slide number: %w", err)
		}

		existing, err := listSlidesFromIndex(root)
		if err != nil {
			return err
		}
		if from < 1 || from > len(existing) || to < 1 || to > len(existing) {
			return fmt.Errorf("slide numbers out of range (have %d slides)", len(existing))
		}

		// Reorder the slice
		item := existing[from-1]
		// Remove from old position
		existing = append(existing[:from-1], existing[from:]...)
		// Insert at new position
		if to-1 >= len(existing) {
			existing = append(existing, item)
		} else {
			existing = append(existing[:to-1], append([]string{item}, existing[to-1:]...)...)
		}

		// Rename files and update index
		return rewriteSlidesAndIndex(root, existing)
	},
}

var lsCmd = &cobra.Command{
	Use:   "ls [dir]",
	Short: "List slides in order",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		root, err := findRootIn(dir)
		if err != nil {
			return err
		}

		slides, err := listSlidesFromIndex(root)
		if err != nil {
			return err
		}
		if len(slides) == 0 {
			fmt.Println("No slides found.")
			return nil
		}

		for i, f := range slides {
			slidePath := filepath.Join(root, "slides", f)
			heading := extractFirstHeading(slidePath)
			slideLayout := detectSlideLayout(slidePath)
			fmt.Printf("  %2d. %-30s [%-8s] %s\n", i+1, f, slideLayout, heading)
		}
		return nil
	},
}

var insertCmd = &cobra.Command{
	Use:   "insert <position> <name>",
	Short: "Insert a new slide at a specific position",
	Long: `Insert creates a new slide at the given position (1-based), shifting all
subsequent slides by +1. The position can range from 1 (insert at beginning)
to len(slides)+1 (append at end).

Handles slides with or without numeric prefixes — all files are renumbered
after insertion to maintain consistent NN-name.html naming.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		pos, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("position must be an integer: %w", err)
		}
		name := scaffold.Slugify(args[1])

		root, err := findRoot()
		if err != nil {
			return err
		}

		layoutName := resolveLayoutFlag(insertLayout, insertType)
		if err := runInsert(root, pos, name, layoutName, insertTitle); err != nil {
			return err
		}

		if slotsFileInsert != "" {
			if err := applySlotsFile(root, pos, slotsFileInsert); err != nil {
				return err
			}
		}

		slides, _ := listSlidesFromIndex(root)
		fmt.Printf("Inserted slide %d of %d: slides/%s\n", pos, len(slides), slides[pos-1])
		return nil
	},
}

// runInsert is the core logic for inserting a slide at a given position.
// It reads ordering from index.html, creates the new slide file, inserts it
// into the ordering, and renumbers all slides + rebuilds index.html.
// The layoutName parameter selects the structural layout template.
func runInsert(root string, position int, name, layoutName, title string) error {
	existing, err := listSlidesFromIndex(root)
	if err != nil {
		return err
	}

	if position < 1 || position > len(existing)+1 {
		return fmt.Errorf("position %d out of range (have %d slides, valid range 1-%d)", position, len(existing), len(existing)+1)
	}

	// Create a temporary filename (will be renumbered by rewriteSlidesAndIndex)
	tmpFileName := fmt.Sprintf("%02d-%s.html", position, name)
	slidePath := filepath.Join(root, "slides", tmpFileName)

	// Render slide from layout template
	content, err := renderSlideFromLayout(name, layoutName, position, title)
	if err != nil {
		// Fall back to legacy theme-based rendering for backward compatibility
		content, err = renderSlideFromTheme(root, name, layoutName, position, title)
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "slides"), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(slidePath, []byte(content), 0644); err != nil {
		return err
	}

	// Build new ordered list with the insertion
	newOrder := make([]string, 0, len(existing)+1)
	newOrder = append(newOrder, existing[:position-1]...)
	newOrder = append(newOrder, tmpFileName)
	newOrder = append(newOrder, existing[position-1:]...)

	// Renumber everything and rebuild index.html
	return rewriteSlidesAndIndex(root, newOrder)
}

var slugifyCmd = &cobra.Command{
	Use:   "slugify [dir]",
	Short: "Rename all slides to slug-based filenames from their <h1> content",
	Long: `Slugify reads each slide's <h1> heading, slugifies it, and renames the file
to use that slug (preserving the numeric prefix). This makes git diffs cleaner
when slides are reordered or inserted, since the slug stays stable.

Slides without an <h1> or whose slug already matches are left unchanged.`,
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

		renamed, err := renameToSlugs(root)
		if err != nil {
			return err
		}

		if renamed == 0 {
			fmt.Println("All slides already have slug-based names.")
		} else {
			fmt.Printf("Renamed %d slide(s).\n", renamed)
		}
		return nil
	},
}

// renameToSlugs reads each slide's <h1> content, slugifies it, and renames
// files + updates index.html. Returns the number of slides renamed.
// Slides without an <h1> or whose slug already matches are left unchanged.
func renameToSlugs(root string) (int, error) {
	slides, err := listSlidesFromIndex(root)
	if err != nil {
		return 0, err
	}

	// Build new names, tracking used slugs for deduplication
	usedSlugs := make(map[string]int)
	newNames := make([]string, len(slides))
	renamed := 0

	for i, filename := range slides {
		heading := extractFirstHeading(filepath.Join(root, "slides", filename))
		if heading == "" {
			// No h1 — keep existing name
			newNames[i] = filename
			namePart := extractNamePart(filename)
			slug := strings.TrimSuffix(namePart, ".html")
			usedSlugs[slug]++
			continue
		}

		slug := scaffold.Slugify(heading)
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

	// Use the existing rewrite infrastructure — but we need to rename
	// files directly since rewriteSlidesAndIndex expects the files to
	// exist with their OLD names and renames them.
	slidesDir := filepath.Join(root, "slides")

	// Rename via temp to avoid collisions
	type renamePair struct{ from, to string }
	var renames []renamePair
	for i, oldName := range slides {
		if newNames[i] != oldName {
			renames = append(renames, renamePair{oldName, newNames[i]})
		}
	}
	for _, r := range renames {
		os.Rename(filepath.Join(slidesDir, r.from), filepath.Join(slidesDir, r.from+".tmp"))
	}
	for _, r := range renames {
		os.Rename(filepath.Join(slidesDir, r.from+".tmp"), filepath.Join(slidesDir, r.to))
	}

	// Rebuild index.html with new names
	indexPath := filepath.Join(root, "index.html")
	indexHTML, err := os.ReadFile(indexPath)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(indexHTML), "\n")
	var newLines []string
	includeInserted := false

	for _, line := range lines {
		if includeRe.MatchString(line) {
			if !includeInserted {
				for _, f := range newNames {
					newLines = append(newLines, fmt.Sprintf(`    {{# include "slides/%s" #}}`, f))
				}
				includeInserted = true
			}
			continue
		}
		newLines = append(newLines, line)
	}

	if err := os.WriteFile(indexPath, []byte(strings.Join(newLines, "\n")), 0644); err != nil {
		return 0, err
	}

	return renamed, nil
}

func init() {
	addCmd.Flags().IntVar(&slideAfter, "after", 0, "insert after slide N")
	addCmd.Flags().StringVar(&slideLayout, "layout", "content", "slide layout: title, content, two-col, section, blank, closing")
	addCmd.Flags().StringVar(&slotsFileAdd, "slots-file", "", "JSON map of layout slot name to inner HTML fragment (after add)")
	addCmd.Flags().StringVar(&slideType, "type", "", "deprecated: use --layout instead")
	_ = addCmd.Flags().MarkHidden("type")

	insertCmd.Flags().StringVar(&insertLayout, "layout", "content", "slide layout: title, content, two-col, section, blank, closing")
	insertCmd.Flags().StringVar(&slotsFileInsert, "slots-file", "", "JSON map of layout slot name to inner HTML fragment (after insert)")
	insertCmd.Flags().StringVar(&insertType, "type", "", "deprecated: use --layout instead")
	_ = insertCmd.Flags().MarkHidden("type")
	insertCmd.Flags().StringVar(&insertTitle, "title", "", "display title for the slide")

	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(mvCmd)
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(insertCmd)
	rootCmd.AddCommand(slugifyCmd)
}

// extractNamePart strips the numeric prefix (e.g., "01-") from a slide filename,
// returning just the name portion. For files without a numeric prefix (e.g.,
// "blah.html" or "my-intro.html"), returns the filename unchanged.
func extractNamePart(filename string) string {
	if m := numPrefixRe.FindStringSubmatch(filename); m != nil {
		return m[2]
	}
	return filename
}

// listSlidesFromIndex returns slide filenames in the order they appear in
// index.html include directives. This is the canonical ordering source.
// Falls back to filesystem listing if index.html has no includes.
func listSlidesFromIndex(root string) ([]string, error) {
	indexPath := filepath.Join(root, "index.html")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}

	var slides []string
	for _, line := range strings.Split(string(data), "\n") {
		if m := includeRe.FindStringSubmatch(line); m != nil {
			// m[1] is "slides/filename.html", strip the "slides/" prefix
			name := strings.TrimPrefix(m[1], "slides/")
			slides = append(slides, name)
		}
	}

	if len(slides) == 0 {
		// Fallback to filesystem
		return listSlideFiles(root), nil
	}
	return slides, nil
}

func findRoot() (string, error) {
	return findRootIn(".")
}

func findRootIn(dir string) (string, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(root, "index.html")); os.IsNotExist(err) {
		return "", fmt.Errorf("no index.html found in %s — is this a slyds presentation? Run 'slyds init' to create one", root)
	}
	return root, nil
}

func listSlideFiles(root string) []string {
	slidesDir := filepath.Join(root, "slides")
	entries, err := os.ReadDir(slidesDir)
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

func extractFirstHeading(filePath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	re := regexp.MustCompile(`<h1[^>]*>(.*?)</h1>`)
	m := re.FindSubmatch(data)
	if m != nil {
		return string(m[1])
	}
	return ""
}

// renderSlideFromTheme renders a slide using the embedded theme template.
// It reads the theme from .slyds.yaml manifest in root (falling back to
// "default"), then looks up the slide type in the theme's theme.yaml config
// to find the correct template file.
func renderSlideFromTheme(root, name, slideType string, number int, titleOverride ...string) (string, error) {
	theme := "default"
	if m, err := scaffold.ReadManifest(root); err == nil && m.Theme != "" {
		theme = m.Theme
	}

	cfg, err := scaffold.LoadThemeConfig(theme)
	if err != nil {
		return "", err
	}

	tmplFile, err := cfg.TemplateForType(slideType)
	if err != nil {
		return "", err
	}

	// Title-case the name for display
	displayName := strings.ReplaceAll(name, "-", " ")
	words := strings.Fields(displayName)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	displayName = strings.Join(words, " ")

	// Allow explicit title override
	if len(titleOverride) > 0 && titleOverride[0] != "" {
		displayName = titleOverride[0]
	}

	tmplPath := fmt.Sprintf("templates/%s/%s", theme, tmplFile)
	content, err := core.TemplatesFS.ReadFile(tmplPath)
	if err != nil {
		return "", fmt.Errorf("slide template %q not found: %w", tmplFile, err)
	}

	tmpl, err := template.New(tmplFile).Parse(string(content))
	if err != nil {
		return "", fmt.Errorf("failed to parse slide template: %w", err)
	}

	data := map[string]any{
		"Title":  displayName,
		"Number": number,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func rewriteSlidesAndIndex(root string, orderedFiles []string) error {
	slidesDir := filepath.Join(root, "slides")

	// First, rename to temp names to avoid conflicts
	type rename struct{ from, to string }
	var renames []rename

	for i, oldName := range orderedFiles {
		namePart := extractNamePart(oldName)
		newName := fmt.Sprintf("%02d-%s", i+1, namePart)
		if oldName != newName {
			renames = append(renames, rename{oldName, newName})
		}
		orderedFiles[i] = newName
	}

	// Rename via temp to avoid collisions
	for _, r := range renames {
		tmpName := r.from + ".tmp"
		os.Rename(filepath.Join(slidesDir, r.from), filepath.Join(slidesDir, tmpName))
	}
	for _, r := range renames {
		tmpName := r.from + ".tmp"
		os.Rename(filepath.Join(slidesDir, tmpName), filepath.Join(slidesDir, r.to))
	}

	// Rebuild include lines in index.html
	indexPath := filepath.Join(root, "index.html")
	indexHTML, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(indexHTML), "\n")
	var newLines []string
	includeInserted := false

	for _, line := range lines {
		if includeRe.MatchString(line) {
			if !includeInserted {
				// Insert all includes at the position of the first include
				for _, f := range orderedFiles {
					newLines = append(newLines, fmt.Sprintf(`    {{# include "slides/%s" #}}`, f))
				}
				includeInserted = true
			}
			// Skip old include lines
			continue
		}
		newLines = append(newLines, line)
	}

	return os.WriteFile(indexPath, []byte(strings.Join(newLines, "\n")), 0644)
}

// resolveLayoutFlag resolves the layout name from --layout and deprecated --type flags.
// If --type is set (non-empty), it maps to a layout name and prints a deprecation warning.
// If both are set, --layout takes precedence.
func resolveLayoutFlag(layoutFlag, typeFlag string) string {
	if typeFlag != "" && layoutFlag == "content" {
		// --type was explicitly set and --layout was left at default
		resolved, _ := layout.ResolveType(typeFlag)
		fmt.Fprintf(os.Stderr, "Warning: --type is deprecated, use --layout %s instead\n", resolved)
		return resolved
	}
	return layoutFlag
}

// renderSlideFromLayout renders a slide using the layout template system.
// This is the preferred method for creating new slides (Phase 2+).
func renderSlideFromLayout(name, layoutName string, number int, titleOverride string) (string, error) {
	displayName := strings.ReplaceAll(name, "-", " ")
	words := strings.Fields(displayName)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	displayName = strings.Join(words, " ")

	if titleOverride != "" {
		displayName = titleOverride
	}

	data := map[string]any{
		"Title":  displayName,
		"Number": number,
	}
	return layout.Render(layoutName, data)
}

// applySlotsFile sets inner HTML for each [data-slot] from a JSON object { "slotName": "<html>..." }.
func applySlotsFile(root string, slidePosition int, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("slots-file: %w", err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("slots-file JSON: %w", err)
	}
	ref := strconv.Itoa(slidePosition)
	for slot, html := range m {
		h := html
		sel := `[data-slot="` + strings.ReplaceAll(slot, `"`, `\"`) + `"]`
		if _, err := runQuery(root, ref, sel, QueryOpts{SetHTML: &h}); err != nil {
			return fmt.Errorf("slot %q: %w", slot, err)
		}
	}
	return nil
}

// detectSlideLayout reads a slide file and detects its layout from the
// data-layout attribute or legacy CSS classes.
func detectSlideLayout(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "content"
	}
	return layout.DetectLayout(string(data))
}
