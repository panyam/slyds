package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/slyds/core"
	"github.com/panyam/templar"
	"github.com/spf13/cobra"
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

		if _, err := core.FindDeckRoot(dir); err != nil {
			return err
		}

		manifest, err := core.ReadManifestFS(templar.NewLocalFS(root))
		if err == core.ErrManifestNotFound {
			manifest, err = promptForManifest()
			if err != nil {
				return err
			}
		} else if err != nil {
			return fmt.Errorf("failed to read .slyds.yaml: %w", err)
		}

		// Refresh engine files from go:embed
		fmt.Printf("Refreshing engine files from built-in assets...\n")
		if err := core.Update(root, manifest.Theme, manifest.Title); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}

		// Add default core source if no sources configured yet
		if !manifest.HasSources() {
			fmt.Printf("Adding default core engine source...\n")
			manifest.Sources = map[string]core.SourceConfig{
				"core": {
					URL:  core.DefaultCoreURL,
					Path: core.DefaultCorePath,
				},
			}
			if err := core.WriteManifestFS(templar.NewLocalFS(root), *manifest); err != nil {
				return fmt.Errorf("failed to update manifest: %w", err)
			}
		}

		// Fetch module dependencies
		fmt.Printf("Fetching module dependencies...\n")
		if err := core.FetchAll(templar.NewLocalFS(root), manifest); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: module fetch failed: %v\n", err)
			fmt.Fprintf(os.Stderr, "Engine files updated from built-in assets. Run 'slyds update' again when network is available.\n")
		} else {
			fmt.Printf("Modules fetched into %s/\n", core.DefaultModulesDir)
		}

		fmt.Printf("Updated %q (theme: %s).\n", dir, manifest.Theme)
		return nil
	},
}

func promptForManifest() (*core.Manifest, error) {
	reader := bufio.NewReader(os.Stdin)

	themes, _ := core.ListThemes()
	fmt.Printf("No .slyds.yaml found. Please provide presentation details.\n")
	fmt.Printf("Theme (%s) [default]: ", strings.Join(themes, ", "))
	theme, _ := reader.ReadString('\n')
	theme = strings.TrimSpace(theme)
	if theme == "" {
		theme = "default"
	}
	if !core.ThemeExists(theme) {
		return nil, fmt.Errorf("unknown theme %q", theme)
	}

	fmt.Print("Title: ")
	title, _ := reader.ReadString('\n')
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	return &core.Manifest{Theme: theme, Title: title}, nil
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
