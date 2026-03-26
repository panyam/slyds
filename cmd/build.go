package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/panyam/slyds/internal/builder"
)

var buildCmd = &cobra.Command{
	Use:   "build [dir]",
	Short: "Inline assets into a single self-contained HTML file",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		root, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		indexPath := filepath.Join(root, "index.html")
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			return fmt.Errorf("no index.html found in %s", root)
		}

		result, err := builder.Build(root)
		if err != nil {
			return fmt.Errorf("build failed: %w", err)
		}

		distDir := filepath.Join(root, "dist")
		if err := os.MkdirAll(distDir, 0755); err != nil {
			return err
		}
		outPath := filepath.Join(distDir, "index.html")
		if err := os.WriteFile(outPath, []byte(result.HTML), 0644); err != nil {
			return err
		}

		cwd, _ := os.Getwd()
		rel, _ := filepath.Rel(cwd, outPath)
		fmt.Printf("\nBuild complete: %s\n", rel)

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
	rootCmd.AddCommand(buildCmd)
}
