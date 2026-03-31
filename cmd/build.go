package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/panyam/slyds/internal/builder"
	"github.com/spf13/cobra"
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
		fmt.Println()
		if err := runBuild(dir); err != nil {
			return err
		}
		fmt.Println()
		return nil
	},
}

// runBuild builds a self-contained HTML file from the presentation at dir.
func runBuild(dir string) error {
	root, err := findRootIn(dir)
	if err != nil {
		return err
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
	fmt.Printf("Build complete: %s\n", rel)

	if len(result.Warnings) > 0 {
		fmt.Println("\nWarnings:")
		for _, w := range result.Warnings {
			fmt.Printf("  - %s\n", w)
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
