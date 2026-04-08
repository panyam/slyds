package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/panyam/slyds/core"
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
		name := core.Slugify(args[0])
		d, err := core.OpenDeckCwd()
		if err != nil {
			return err
		}

		existing, err := d.SlideFilenames()
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
		if err := d.InsertSlide(position, name, layoutName, ""); err != nil {
			return err
		}

		if slotsFileAdd != "" {
			slots, err := readSlotsFile(slotsFileAdd)
			if err != nil {
				return err
			}
			if err := d.ApplySlots(position, slots); err != nil {
				return err
			}
		}

		slides, _ := d.SlideFilenames()
		fmt.Printf("Added slide: slides/%s\n", slides[position-1])
		return nil
	},
}

var rmCmd = &cobra.Command{
	Use:   "rm <name-or-number>",
	Short: "Remove a slide",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := core.OpenDeckCwd()
		if err != nil {
			return err
		}

		target := args[0]
		existing, err := d.SlideFilenames()
		if err != nil {
			return err
		}

		var slideFile string
		if num, err := strconv.Atoi(target); err == nil {
			if num < 1 || num > len(existing) {
				return fmt.Errorf("slide %d out of range (have %d slides)", num, len(existing))
			}
			slideFile = existing[num-1]
		} else {
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

		if err := d.RemoveSlide(slideFile); err != nil {
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
		d, err := core.OpenDeckCwd()
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

		return d.MoveSlide(from, to)
	},
}

var lsJSON bool

type slideInfo struct {
	Position int    `json:"position"`
	File     string `json:"file"`
	Layout   string `json:"layout"`
	Title    string `json:"title"`
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
		d, err := core.OpenDeckDir(dir)
		if err != nil {
			return err
		}

		slides, err := d.SlideFilenames()
		if err != nil {
			return err
		}
		if len(slides) == 0 {
			if lsJSON {
				fmt.Println("[]")
			} else {
				fmt.Println("No slides found.")
			}
			return nil
		}

		if lsJSON {
			var infos []slideInfo
			for i, f := range slides {
				content, _ := d.GetSlideContent(i + 1)
				infos = append(infos, slideInfo{
					Position: i + 1,
					File:     f,
					Layout:   core.DetectLayout(content),
					Title:    core.ExtractFirstHeading(content),
				})
			}
			data, err := json.MarshalIndent(infos, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		for i, f := range slides {
			content, _ := d.GetSlideContent(i + 1)
			heading := core.ExtractFirstHeading(content)
			slideLayout := core.DetectLayout(content)
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
		name := core.Slugify(args[1])

		d, err := core.OpenDeckCwd()
		if err != nil {
			return err
		}

		layoutName := resolveLayoutFlag(insertLayout, insertType)
		if err := d.InsertSlide(pos, name, layoutName, insertTitle); err != nil {
			return err
		}

		if slotsFileInsert != "" {
			slots, err := readSlotsFile(slotsFileInsert)
			if err != nil {
				return err
			}
			if err := d.ApplySlots(pos, slots); err != nil {
				return err
			}
		}

		slides, _ := d.SlideFilenames()
		fmt.Printf("Inserted slide %d of %d: slides/%s\n", pos, len(slides), slides[pos-1])
		return nil
	},
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
		d, err := core.OpenDeckDir(dir)
		if err != nil {
			return err
		}

		renamed, err := d.SlugifySlides(core.Slugify)
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
	lsCmd.Flags().BoolVar(&lsJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(insertCmd)
	rootCmd.AddCommand(slugifyCmd)
}









// resolveLayoutFlag resolves the layout name from --layout and deprecated --type flags.
// If --type is set (non-empty), it maps to a layout name and prints a deprecation warning.
// If both are set, --layout takes precedence.
func resolveLayoutFlag(layoutFlag, typeFlag string) string {
	if typeFlag != "" && layoutFlag == "content" {
		// --type was explicitly set and --layout was left at default
		resolved, _ := core.ResolveType(typeFlag)
		fmt.Fprintf(os.Stderr, "Warning: --type is deprecated, use --layout %s instead\n", resolved)
		return resolved
	}
	return layoutFlag
}


// readSlotsFile reads a JSON slots file and returns the slot name → HTML map.
func readSlotsFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("slots-file: %w", err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("slots-file JSON: %w", err)
	}
	return m, nil
}

