package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/panyam/slyds/core"
	"github.com/panyam/templar"
	"github.com/spf13/cobra"
)

var updateSkipFetch bool

var updateCmd = &cobra.Command{
	Use:   "update [dir]",
	Short: "Refresh engine and theme files without touching slides",
	Long: `Update refreshes slyds engine files (slyds.css, slyds.js, theme.css,
index.html layout, theme images) using the latest embedded assets, while
preserving your slide content and ordering.

The theme and title are read from .slyds.yaml in the presentation directory.
If this file is missing (e.g., for presentations created before this feature),
you will be prompted to enter the theme and title.

If the theme is not a built-in theme (custom/external), engine files are
still refreshed but theme-specific rendering is skipped with a warning.

Use --skip-fetch to skip module dependency fetching (useful offline or
when the module URL is unreachable).`,
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
			var warn *core.UnknownThemeWarning
			if errors.As(err, &warn) {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", warn)
			} else {
				return fmt.Errorf("update failed: %w", err)
			}
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

		// Fetch module dependencies (with timeout)
		if !updateSkipFetch && manifest.HasSources() {
			fetchModules(root, manifest)
		} else if updateSkipFetch {
			fmt.Printf("Skipping module fetch (--skip-fetch).\n")
		}

		fmt.Printf("Updated %q (theme: %s).\n", dir, manifest.Theme)
		return nil
	},
}

const fetchTimeout = 15 * time.Second

// fetchModules runs FetchAll with a timeout to prevent hanging on
// unreachable module URLs. Prints a warning on failure or timeout.
func fetchModules(root string, manifest *core.Manifest) {
	fmt.Printf("Fetching module dependencies (%s timeout)...\n", fetchTimeout)
	done := make(chan error, 1)
	go func() {
		done <- core.FetchAll(templar.NewLocalFS(root), manifest)
	}()
	select {
	case err := <-done:
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: module fetch failed: %v\n", err)
			fmt.Fprintf(os.Stderr, "Engine files updated. Run 'slyds update' again when network is available.\n")
		} else {
			fmt.Printf("Modules fetched into %s/\n", core.DefaultModulesDir)
		}
	case <-time.After(fetchTimeout):
		fmt.Fprintf(os.Stderr, "Warning: module fetch timed out after %s. Use --skip-fetch to skip.\n", fetchTimeout)
	}
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
	updateCmd.Flags().BoolVar(&updateSkipFetch, "skip-fetch", false, "Skip fetching module dependencies (useful offline)")
	rootCmd.AddCommand(updateCmd)
}
