package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/user/slyds/assets"
	"github.com/user/slyds/internal/scaffold"
)

var (
	slideAfter int
	slideType  string
)

var includeRe = regexp.MustCompile(`\{\{#\s*include\s+"(slides/[^"]+)"\s*#\}\}`)

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

		existing := listSlideFiles(root)
		newNum := len(existing) + 1

		// If --after is specified, insert after that position
		insertAt := len(existing) // default: append at end
		if slideAfter > 0 {
			if slideAfter > len(existing) {
				return fmt.Errorf("--after %d is out of range (have %d slides)", slideAfter, len(existing))
			}
			insertAt = slideAfter
			newNum = slideAfter + 1
		}

		slideFileName := fmt.Sprintf("%02d-%s.html", newNum, name)
		slidePath := filepath.Join(root, "slides", slideFileName)

		// Render slide from theme template
		content, err := renderSlideFromTheme(name, slideType, newNum)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(root, "slides"), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(slidePath, []byte(content), 0644); err != nil {
			return err
		}

		// Update index.html
		indexPath := filepath.Join(root, "index.html")
		indexHTML, err := os.ReadFile(indexPath)
		if err != nil {
			return err
		}

		includeLine := fmt.Sprintf(`    {{# include "slides/%s" #}}`, slideFileName)
		lines := strings.Split(string(indexHTML), "\n")
		var newLines []string
		includeCount := 0
		inserted := false

		for _, line := range lines {
			if includeRe.MatchString(line) {
				includeCount++
				if includeCount == insertAt && !inserted {
					newLines = append(newLines, line)
					newLines = append(newLines, includeLine)
					inserted = true
					continue
				}
			}
			newLines = append(newLines, line)
		}
		if !inserted {
			// Insert before the navigation div
			var finalLines []string
			for _, line := range newLines {
				if strings.Contains(line, `<div class="navigation">`) {
					finalLines = append(finalLines, includeLine)
				}
				finalLines = append(finalLines, line)
			}
			newLines = finalLines
		}

		if err := os.WriteFile(indexPath, []byte(strings.Join(newLines, "\n")), 0644); err != nil {
			return err
		}

		// Renumber all slides
		if slideAfter > 0 {
			if err := renumberSlides(root); err != nil {
				return err
			}
		}

		fmt.Printf("Added slide: slides/%s\n", slideFileName)
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
		existing := listSlideFiles(root)

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

		// Remove include line from index.html
		indexPath := filepath.Join(root, "index.html")
		indexHTML, err := os.ReadFile(indexPath)
		if err != nil {
			return err
		}

		lines := strings.Split(string(indexHTML), "\n")
		var newLines []string
		for _, line := range lines {
			if strings.Contains(line, fmt.Sprintf(`"slides/%s"`, slideFile)) {
				continue
			}
			newLines = append(newLines, line)
		}

		if err := os.WriteFile(indexPath, []byte(strings.Join(newLines, "\n")), 0644); err != nil {
			return err
		}

		if err := renumberSlides(root); err != nil {
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

		existing := listSlideFiles(root)
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
	Use:   "ls",
	Short: "List slides in order",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}

		slides := listSlideFiles(root)
		if len(slides) == 0 {
			fmt.Println("No slides found.")
			return nil
		}

		for i, f := range slides {
			heading := extractFirstHeading(filepath.Join(root, "slides", f))
			fmt.Printf("  %2d. %-30s %s\n", i+1, f, heading)
		}
		return nil
	},
}

func init() {
	addCmd.Flags().IntVar(&slideAfter, "after", 0, "insert after slide N")
	addCmd.Flags().StringVar(&slideType, "type", "content", "slide type: title, content, closing, two-column, section")
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(mvCmd)
	rootCmd.AddCommand(lsCmd)
}

func findRoot() (string, error) {
	root, err := filepath.Abs(".")
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(root, "index.html")); os.IsNotExist(err) {
		return "", fmt.Errorf("no index.html found in current directory")
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
// It looks up the slide type in the theme's theme.yaml config to find the
// correct template file, falling back to the default theme if needed.
func renderSlideFromTheme(name, slideType string, number int) (string, error) {
	// Load theme config to resolve slide type → template path
	cfg, err := scaffold.LoadThemeConfig("default")
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

	tmplPath := fmt.Sprintf("templates/default/%s", tmplFile)
	content, err := assets.TemplatesFS.ReadFile(tmplPath)
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

func renumberSlides(root string) error {
	existing := listSlideFiles(root)
	return rewriteSlidesAndIndex(root, existing)
}

func rewriteSlidesAndIndex(root string, orderedFiles []string) error {
	slidesDir := filepath.Join(root, "slides")

	// First, rename to temp names to avoid conflicts
	type rename struct{ from, to string }
	var renames []rename

	for i, oldName := range orderedFiles {
		// Extract the name part (after the NN- prefix)
		parts := strings.SplitN(oldName, "-", 2)
		namePart := oldName
		if len(parts) == 2 {
			namePart = parts[1]
		}
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
