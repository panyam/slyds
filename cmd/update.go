package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/panyam/slyds/internal/scaffold"
)

var updateCmd = &cobra.Command{
	Use:   "update [dir]",
	Short: "Refresh engine and theme files without touching slides",
	Long: `Update refreshes slyds engine files (slyds.css, slyds.js, theme.css,
index.html layout, theme images) using the latest embedded assets, while
preserving your slide content and ordering.

The theme and title are read from .slyds.yaml in the presentation directory.
If this file is missing (e.g., for presentations created before this feature),
you will be prompted to enter the theme and title.`,
	Args: cobra.MaximumNArgs(1),
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
			return fmt.Errorf("no index.html found in %s — is this a slyds presentation?", root)
		}

		manifest, err := scaffold.ReadManifest(root)
		if err == scaffold.ErrManifestNotFound {
			manifest, err = promptForManifest()
			if err != nil {
				return err
			}
		} else if err != nil {
			return fmt.Errorf("failed to read .slyds.yaml: %w", err)
		}

		if err := scaffold.Update(root, manifest.Theme, manifest.Title); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}

		fmt.Printf("Updated %q (theme: %s).\n", dir, manifest.Theme)
		return nil
	},
}

func promptForManifest() (*scaffold.Manifest, error) {
	reader := bufio.NewReader(os.Stdin)

	themes, _ := scaffold.ListThemes()
	fmt.Printf("No .slyds.yaml found. Please provide presentation details.\n")
	fmt.Printf("Theme (%s) [default]: ", strings.Join(themes, ", "))
	theme, _ := reader.ReadString('\n')
	theme = strings.TrimSpace(theme)
	if theme == "" {
		theme = "default"
	}
	if !scaffold.ThemeExists(theme) {
		return nil, fmt.Errorf("unknown theme %q", theme)
	}

	fmt.Print("Title: ")
	title, _ := reader.ReadString('\n')
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	return &scaffold.Manifest{Theme: theme, Title: title}, nil
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
