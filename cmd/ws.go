package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// wsCmd is a debug/introspection subcommand for the Workspace abstraction.
// It lets humans see what a `slyds mcp --deck-root DIR` would see without
// having to start an MCP server and send JSON-RPC. Also used by the demo
// smoke test (make demo-smoke) to verify the workspace wiring end-to-end.
var wsCmd = &cobra.Command{
	Use:   "ws",
	Short: "Inspect the current workspace (deck root and visible decks)",
	Long: `ws is a small debug/introspection subcommand for the Workspace layer.

Subcommands:
  slyds ws info            Print the workspace root path
  slyds ws list            List decks visible to the workspace

Workspaces are how the MCP server resolves decks from authenticated context.
For the CLI, there is one implicit workspace rooted at --deck-root (default: ".").`,
}

var (
	wsDeckRoot string
	wsJSON     bool
)

var wsInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Print the workspace root path and deck count",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := NewLocalWorkspace(resolveDeckRoot(wsDeckRoot))
		if err != nil {
			return err
		}
		refs, err := ws.ListDecks()
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		info := map[string]any{
			"root":       ws.Root(),
			"deck_count": len(refs),
			"kind":       "local",
		}
		if wsJSON {
			data, _ := json.MarshalIndent(info, "", "  ")
			fmt.Fprintln(out, string(data))
			return nil
		}
		fmt.Fprintf(out, "Workspace: local\n")
		fmt.Fprintf(out, "Root:      %s\n", ws.Root())
		fmt.Fprintf(out, "Decks:     %d\n", len(refs))
		return nil
	},
}

var wsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List decks visible to the workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := NewLocalWorkspace(resolveDeckRoot(wsDeckRoot))
		if err != nil {
			return err
		}
		refs, err := ws.ListDecks()
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()

		if wsJSON {
			// Return full metadata for each deck — matches the `list_decks`
			// MCP tool output so scripts that consume one can consume the
			// other.
			summaries := make([]deckSummary, 0, len(refs))
			for _, ref := range refs {
				d, err := ws.OpenDeck(ref.Name)
				if err != nil {
					continue
				}
				count, _ := d.SlideCount()
				summaries = append(summaries, deckSummary{
					Name:   ref.Name,
					Title:  d.Title(),
					Theme:  d.Theme(),
					Slides: count,
				})
			}
			data, _ := json.MarshalIndent(summaries, "", "  ")
			fmt.Fprintln(out, string(data))
			return nil
		}

		if len(refs) == 0 {
			fmt.Fprintln(out, "(no decks)")
			return nil
		}
		for _, ref := range refs {
			d, err := ws.OpenDeck(ref.Name)
			if err != nil {
				fmt.Fprintf(out, "%-30s  (error: %v)\n", ref.Name, err)
				continue
			}
			count, _ := d.SlideCount()
			fmt.Fprintf(out, "%-30s  %-10s  %3d slides  %s\n",
				ref.Name, d.Theme(), count, d.Title())
		}
		return nil
	},
}

func init() {
	wsCmd.PersistentFlags().StringVar(&wsDeckRoot, "deck-root", "", "Workspace root directory (default: $SLYDS_DECK_ROOT, or current directory)")
	wsCmd.PersistentFlags().BoolVar(&wsJSON, "json", false, "Output as JSON instead of human-readable text")
	wsCmd.AddCommand(wsInfoCmd)
	wsCmd.AddCommand(wsListCmd)
	rootCmd.AddCommand(wsCmd)
}
