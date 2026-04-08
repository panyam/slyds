package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/panyam/slyds/core"
	"github.com/spf13/cobra"
)

var buildJSON bool

var buildCmd = &cobra.Command{
	Use:   "build [dir]",
	Short: "Inline assets into a single self-contained HTML file",
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

		result, err := d.Build()
		if err != nil {
			return fmt.Errorf("build failed: %w", err)
		}

		if buildJSON {
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		// Write output to dist/ via DeckFS
		d.FS.MkdirAll("dist", 0755)
		if err := d.FS.WriteFile("dist/index.html", []byte(result.HTML), 0644); err != nil {
			return err
		}

		fmt.Println("\nBuild complete: dist/index.html")
		if len(result.Warnings) > 0 {
			fmt.Println("\nWarnings:")
			for _, w := range result.Warnings {
				fmt.Printf("  - %s\n", w)
			}
		}
		fmt.Println()
		return nil
	},
}

func init() {
	buildCmd.Flags().BoolVar(&buildJSON, "json", false, "output as JSON to stdout instead of writing dist/index.html")
	rootCmd.AddCommand(buildCmd)
}
