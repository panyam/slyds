package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/panyam/slyds/core"
)

var (
	initSlideCount    int
	initTheme         string
	initDir           string
	initMCPDocs       bool
	initFilenameStyle string
)

var initCmd = &cobra.Command{
	Use:   `init "Title"`,
	Short: "Scaffold a new presentation",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := strings.Join(args, " ")
		if title == "" {
			return fmt.Errorf("title is required")
		}
		if initSlideCount < 2 {
			return fmt.Errorf("slide count must be at least 2 (title + closing)")
		}

		outDir := initDir
		if outDir == "" {
			outDir = core.Slugify(title)
		}

		dir, err := core.CreateInDirWithOpts(outDir, core.ScaffoldOpts{
			Title:           title,
			SlideCount:      initSlideCount,
			ThemeName:       initTheme,
			IncludeMCPAgent: initMCPDocs,
			FilenameStyle:   initFilenameStyle,
		})
		if err != nil {
			return err
		}
		fmt.Printf("\nCreated %q with %d slides (theme: %s).\n", dir, initSlideCount, initTheme)
		fmt.Println("Scaffolded from built-in engine assets.")
		fmt.Println("\nNext steps:")
		fmt.Printf("  slyds serve %s          # preview locally\n", dir)
		fmt.Printf("  slyds build %s          # build self-contained HTML\n", dir)
		fmt.Printf("  cd %s && slyds update   # fetch latest engine from git\n\n", dir)
		return nil
	},
}

func init() {
	initCmd.Flags().IntVarP(&initSlideCount, "slides", "n", 3, "number of slides (min 2)")
	themes, _ := core.ListThemes()
	initCmd.Flags().StringVar(&initTheme, "theme", "default", "theme to use ("+strings.Join(themes, ", ")+")")
	initCmd.Flags().StringVar(&initDir, "dir", "", "output directory (default: slugified title)")
	initCmd.Flags().BoolVar(&initMCPDocs, "mcp", true, "include MCP setup section in AGENT.md (stored in .slyds.yaml as agent_include_mcp)")
	initCmd.Flags().StringVar(&initFilenameStyle, "filename-style", "", "slide filename format: 'numbered' (default: 01-title.html) or 'slug-only' (title.html)")
	rootCmd.AddCommand(initCmd)
}
