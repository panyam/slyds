package cmd

import (
	"fmt"

	"github.com/panyam/slyds/core"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check [dir]",
	Short: "Validate a presentation deck",
	Long: `Check validates a presentation for common issues:
- Slides referenced in index.html that don't exist on disk
- Slide files on disk not referenced in index.html (orphans)
- Slides missing speaker notes
- Broken local asset references (images, videos)
- Estimated talk time from speaker notes word count`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		d, err := core.OpenDeckDir(dir)
		if err != nil {
			return err
		}

		result, err := d.Check()
		if err != nil {
			return err
		}

		// Print results
		fmt.Printf("%d slides", result.SlideCount)
		if result.InSync {
			fmt.Println(", index.html in sync")
		} else {
			fmt.Println(", index.html OUT OF SYNC")
		}

		for _, issue := range result.Issues.Errors() {
			fmt.Printf("  ERROR: %s\n", issue.Detail)
		}
		for _, issue := range result.Issues.Warnings() {
			fmt.Printf("  WARN:  %s\n", issue.Detail)
		}

		if result.EstimatedMinutes > 0 {
			fmt.Printf("  Estimated talk time: ~%.0f min\n", result.EstimatedMinutes)
		}

		if result.Issues.HasErrors() {
			return fmt.Errorf("%d error(s) found", len(result.Issues.Errors()))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
