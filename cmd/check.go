package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/panyam/slyds/internal/scaffold"
	"github.com/spf13/cobra"
)

// CheckResult holds the results of a deck validation.
type CheckResult struct {
	SlideCount       int
	InSync           bool
	Errors           []string
	Warnings         []string
	EstimatedMinutes float64
}

var assetRefRe = regexp.MustCompile(`(?:src|href)="([^"]+)"`)
var speakerNotesRe = regexp.MustCompile(`class="speaker-notes"`)

var checkCmd = &cobra.Command{
	Use:   "check [dir]",
	Short: "Validate a presentation deck",
	Long: `Check validates a presentation for common issues:
- Slides referenced in index.html that don't exist on disk
- Slide files on disk not referenced in index.html (orphans)
- Slides missing speaker notes
- Broken local asset references (images, videos)
- Estimated talk time from speaker notes word count`,
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

		result, err := checkDeck(root)
		if err != nil {
			return err
		}

		// Print results
		fmt.Printf("%d slides", result.SlideCount)
		if result.InSync {
			fmt.Println(", index.html in sync")
		} else {
			fmt.Println(", index.html OUT OF SYNC")
		}

		for _, e := range result.Errors {
			fmt.Printf("  ERROR: %s\n", e)
		}
		for _, w := range result.Warnings {
			fmt.Printf("  WARN:  %s\n", w)
		}

		if result.EstimatedMinutes > 0 {
			fmt.Printf("  Estimated talk time: ~%.0f min\n", result.EstimatedMinutes)
		}

		if len(result.Errors) > 0 {
			return fmt.Errorf("%d error(s) found", len(result.Errors))
		}
		return nil
	},
}

func checkDeck(root string) (*CheckResult, error) {
	result := &CheckResult{InSync: true}

	// Get slides from index.html and from filesystem
	indexSlides, err := listSlidesFromIndex(root)
	if err != nil {
		return nil, err
	}
	diskSlides := listSlideFiles(root)

	result.SlideCount = len(indexSlides)

	// Build sets for comparison
	indexSet := make(map[string]bool)
	for _, s := range indexSlides {
		indexSet[s] = true
	}
	diskSet := make(map[string]bool)
	for _, s := range diskSlides {
		diskSet[s] = true
	}

	// Check for missing files (in index but not on disk)
	for _, s := range indexSlides {
		if !diskSet[s] {
			result.Errors = append(result.Errors, fmt.Sprintf("%s referenced in index.html but not found on disk", s))
			result.InSync = false
		}
	}

	// Check for orphan files (on disk but not in index)
	for _, s := range diskSlides {
		if !indexSet[s] {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s on disk but not in index.html (orphan)", s))
			result.InSync = false
		}
	}

	// Check each slide for speaker notes, broken assets
	totalWords := 0
	slidesDir := filepath.Join(root, "slides")

	for _, s := range indexSlides {
		slidePath := filepath.Join(slidesDir, s)
		data, err := os.ReadFile(slidePath)
		if err != nil {
			continue // already flagged as missing
		}
		content := string(data)

		// Check speaker notes
		if !speakerNotesRe.MatchString(content) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: no speaker notes", s))
		}

		// Count words in speaker notes for talk time estimate
		if idx := strings.Index(content, `class="speaker-notes"`); idx >= 0 {
			notesSection := content[idx:]
			// Strip HTML tags for word counting
			tagRe := regexp.MustCompile(`<[^>]+>`)
			text := tagRe.ReplaceAllString(notesSection, " ")
			words := strings.Fields(text)
			totalWords += len(words)
		}

		// Check local asset references
		matches := assetRefRe.FindAllStringSubmatch(content, -1)
		for _, m := range matches {
			ref := m[1]
			// Skip remote URLs, anchors, and CSS/JS (already handled)
			if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") ||
				strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "data:") ||
				strings.HasSuffix(ref, ".css") || strings.HasSuffix(ref, ".js") {
				continue
			}
			// Resolve relative to presentation root (slides reference assets relative to root)
			assetPath := filepath.Join(root, ref)
			if _, err := os.Stat(assetPath); os.IsNotExist(err) {
				// Also try relative to slides dir
				assetPath = filepath.Join(slidesDir, ref)
				if _, err := os.Stat(assetPath); os.IsNotExist(err) {
					result.Warnings = append(result.Warnings, fmt.Sprintf("%s: broken asset reference %q", s, ref))
				}
			}
		}
	}

	// Estimate talk time: ~150 words per minute
	if totalWords > 0 {
		result.EstimatedMinutes = float64(totalWords) / 150.0
	}

	// Check module state if sources are configured
	manifest, err := scaffold.ReadManifest(root)
	if err == nil && manifest.HasSources() {
		modulesDir := manifest.ResolveModulesDir(root)
		if _, err := os.Stat(modulesDir); os.IsNotExist(err) {
			result.Warnings = append(result.Warnings, "sources configured in .slyds.yaml but .slyds-modules/ not found — run 'slyds update' to fetch dependencies")
		}
		lockPath := scaffold.LockPath(root)
		if _, err := os.Stat(lockPath); os.IsNotExist(err) {
			result.Warnings = append(result.Warnings, "no .slyds.lock found — run 'slyds update' to generate lock file")
		}
	}

	return result, nil
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
