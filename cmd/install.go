package cmd

import (
	"fmt"
	"strings"

	"github.com/panyam/slyds/core"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install <url[@version]>",
	Short: "Install a theme or layout package from a git repository",
	Long: `Install adds an external theme or layout package to .slyds.yaml and fetches
it into .slyds-modules/. The package is then available for use in this deck.

Examples:
  slyds install github.com/someone/slyds-theme-nord
  slyds install github.com/someone/slyds-theme-nord@v1.0.0
  slyds install github.com/someone/slyds-layout-timeline@main`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRootIn(".")
		if err != nil {
			return err
		}

		manifest, err := core.ReadManifest(root)
		if err == core.ErrManifestNotFound {
			return fmt.Errorf("no .slyds.yaml found — is this a slyds presentation?")
		}
		if err != nil {
			return err
		}

		// Parse url[@version]
		url, version := parseInstallArg(args[0])
		name := deriveSourceName(url)

		// Add to manifest
		if manifest.Sources == nil {
			manifest.Sources = make(map[string]core.SourceConfig)
		}

		src := core.SourceConfig{URL: url}
		if version != "" {
			if strings.HasPrefix(version, "v") {
				src.Version = version
			} else {
				src.Ref = version
			}
		}
		manifest.Sources[name] = src

		// Write updated manifest
		if err := core.WriteManifest(root, *manifest); err != nil {
			return fmt.Errorf("failed to update .slyds.yaml: %w", err)
		}

		// Fetch the new source
		fmt.Printf("Fetching %s...\n", url)
		if err := core.FetchAll(manifest, root); err != nil {
			return fmt.Errorf("fetch failed: %w", err)
		}

		fmt.Printf("Installed %q as %q\n", url, name)
		return nil
	},
}

// parseInstallArg splits "github.com/user/repo@v1.0" into url and version.
func parseInstallArg(arg string) (url, version string) {
	if idx := strings.LastIndex(arg, "@"); idx > 0 {
		return arg[:idx], arg[idx+1:]
	}
	return arg, ""
}

// deriveSourceName extracts a short name from a URL.
// "github.com/someone/slyds-theme-nord" → "slyds-theme-nord"
func deriveSourceName(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}

func init() {
	rootCmd.AddCommand(installCmd)
}
