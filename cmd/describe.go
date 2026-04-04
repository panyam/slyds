package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/panyam/slyds/core"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var describeJSON bool

var describeCmd = &cobra.Command{
	Use:   "describe [dir]",
	Short: "Output a structured summary of the presentation deck",
	Long: `Describe outputs a YAML (default) or JSON summary of the deck including
slide count, layouts used, per-slide metadata (title, layout, word count,
speaker notes presence), available themes and layouts.

Designed for LLM consumption — provides efficient context about a deck
without reading every slide file.`,
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

		desc, err := d.Describe()
		if err != nil {
			return err
		}

		if describeJSON {
			data, err := json.MarshalIndent(desc, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		data, err := yaml.Marshal(desc)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	},
}

func init() {
	describeCmd.Flags().BoolVar(&describeJSON, "json", false, "output as JSON instead of YAML")
	rootCmd.AddCommand(describeCmd)
}
