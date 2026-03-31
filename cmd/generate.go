package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/panyam/slyds/internal/generate"
	"github.com/panyam/slyds/internal/scaffold"
	"github.com/spf13/cobra"
)

var (
	generateOutDir string
	generateBuild  bool
)

var generateCmd = &cobra.Command{
	Use:   "generate [file]",
	Short: "Generate a presentation from a JSON spec",
	Long: `Generate creates a complete slyds presentation from a JSON specification.
The JSON can come from a file argument or stdin (pipe-friendly).

This is designed for integration with tools like Glean — any system that
can produce structured JSON can create polished presentations without
needing a coding agent or CLI expertise.

JSON format:
  {
    "title": "My Presentation",
    "theme": "corporate",
    "slides": [
      {
        "layout": "title",
        "title": "Welcome",
        "slots": { "subtitle": "An overview" },
        "notes": "Opening remarks"
      },
      {
        "layout": "content",
        "title": "Key Points",
        "slots": { "body": "<ul><li>Point 1</li><li>Point 2</li></ul>" }
      },
      {
        "layout": "two-col",
        "title": "Comparison",
        "slots": {
          "left": "<h2>Before</h2><p>Old approach</p>",
          "right": "<h2>After</h2><p>New approach</p>"
        }
      }
    ]
  }

Available layouts: title, content, two-col, section, blank, closing.
If layout is omitted, first slide defaults to "title", last to "closing",
and middle slides to "content".

Available themes: default, minimal, dark, corporate, hacker.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var input io.Reader

		if len(args) > 0 {
			f, err := os.Open(args[0])
			if err != nil {
				return fmt.Errorf("failed to open %s: %w", args[0], err)
			}
			defer f.Close()
			input = f
		} else {
			// Check if stdin has data
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) != 0 {
				return fmt.Errorf("no input: provide a JSON file argument or pipe JSON to stdin")
			}
			input = os.Stdin
		}

		data, err := io.ReadAll(input)
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		var spec generate.DeckSpec
		if err := json.Unmarshal(data, &spec); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}

		outDir := generateOutDir
		if outDir == "" {
			outDir = scaffold.Slugify(spec.Title)
		}

		if err := generate.Generate(&spec, outDir); err != nil {
			return err
		}

		fmt.Printf("Generated presentation in %s/ (%d slides, theme: %s)\n", outDir, len(spec.Slides), spec.Theme)

		if generateBuild {
			fmt.Println("Building self-contained HTML...")
			return runBuild(outDir)
		}

		fmt.Println("\nNext steps:")
		fmt.Printf("  slyds serve %s    # preview\n", outDir)
		fmt.Printf("  slyds build %s    # build self-contained HTML\n", outDir)
		return nil
	},
}

func init() {
	generateCmd.Flags().StringVarP(&generateOutDir, "out", "o", "", "output directory (default: slugified title)")
	generateCmd.Flags().BoolVar(&generateBuild, "build", false, "also build dist/index.html after generating")
	rootCmd.AddCommand(generateCmd)
}
