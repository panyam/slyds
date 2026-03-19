package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/slyds/internal/scaffold"
)

var (
	initSlideCount int
	initTheme      string
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
		dir, err := scaffold.CreateWithTheme(title, initSlideCount, initTheme)
		if err != nil {
			return err
		}
		fmt.Printf("\nCreated %q with %d slides (theme: %s).\n", dir, initSlideCount, initTheme)
		fmt.Println("\nNext steps:")
		fmt.Printf("  slyds serve %s\n", dir)
		fmt.Printf("  slyds build %s\n\n", dir)
		return nil
	},
}

func init() {
	initCmd.Flags().IntVarP(&initSlideCount, "slides", "n", 3, "number of slides (min 2)")
	initCmd.Flags().StringVar(&initTheme, "theme", "default", "theme to use (default, minimal, dark, corporate)")
	rootCmd.AddCommand(initCmd)
}
