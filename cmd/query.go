package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/panyam/slyds/core"
	"github.com/spf13/cobra"
)

var (
	qHTML    bool
	qAttr   string
	qCount  bool
	qSet    string
	qSetHTML string
	qAppend string
	qSetAttr string
	qRemove bool
	qAll    bool

	queryBatchPath     string
	queryBatchAtomic   bool
	queryBatchContinue bool
)

var queryCmd = &cobra.Command{
	Use:   "query <slide> <selector> [dir]",
	Short: "Read or modify slide content with CSS selectors",
	Long: `Query uses CSS selectors to read or modify slide content.

Read examples:
  slyds query 1 h1               # text of first <h1> in slide 1
  slyds query 2 "img" --attr src  # src attribute of first <img> in slide 2
  slyds query 3 p --count         # number of <p> elements in slide 3
  slyds query intro h1            # search by slide name substring

Write examples:
  slyds query 1 h1 --set "New Title"
  slyds query 2 ".body" --set-html "<p>New content</p>"
  slyds query 3 ".footer" --append "<span>v2</span>"
  slyds query 1 "section" --set-attr "class=highlight"
  slyds query 2 ".old" --remove

Batch mode:
  slyds query --batch ops.json       # apply batch operations from file
  cat ops.json | slyds query --batch -  # from stdin`,
	Args: cobra.RangeArgs(0, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		if queryBatchPath != "" {
			return runBatchCmd(args)
		}

		if len(args) < 2 {
			return fmt.Errorf("requires args: <slide> <selector> [dir], or use --batch")
		}

		slideRef := args[0]
		selector := args[1]
		dir := "."
		if len(args) > 2 {
			dir = args[2]
		}

		d, err := core.OpenDeckDir(dir)
		if err != nil {
			return err
		}

		opts := core.QueryOpts{
			HTML:   qHTML,
			Attr:   qAttr,
			Count:  qCount,
			Remove: qRemove,
			All:    qAll,
		}
		if cmd.Flags().Changed("set") {
			opts.Set = &qSet
		}
		if cmd.Flags().Changed("set-html") {
			opts.SetHTML = &qSetHTML
		}
		if cmd.Flags().Changed("append") {
			opts.Append = &qAppend
		}
		if cmd.Flags().Changed("set-attr") {
			opts.SetAttr = &qSetAttr
		}

		result, err := d.Query(slideRef, selector, opts)
		if err != nil {
			return err
		}
		for _, r := range result {
			fmt.Println(r)
		}
		return nil
	},
}

func runBatchCmd(args []string) error {
	dir := "."
	if len(args) == 1 {
		dir = args[0]
	} else if len(args) > 1 {
		return fmt.Errorf("with --batch, use at most one argument: [dir]")
	}

	d, err := core.OpenDeckDir(dir)
	if err != nil {
		return err
	}

	var r io.Reader
	if queryBatchPath == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(queryBatchPath)
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	var batch core.BatchFile
	if err := json.Unmarshal(data, &batch); err != nil {
		return fmt.Errorf("batch JSON: %w", err)
	}

	return d.BatchQuery(batch.Operations, queryBatchAtomic, queryBatchContinue)
}

func init() {
	queryCmd.Flags().BoolVar(&qHTML, "html", false, "return inner HTML instead of text")
	queryCmd.Flags().StringVar(&qAttr, "attr", "", "return attribute value")
	queryCmd.Flags().BoolVar(&qCount, "count", false, "return match count")
	queryCmd.Flags().StringVar(&qSet, "set", "", "set inner text of matched element")
	queryCmd.Flags().StringVar(&qSetHTML, "set-html", "", "set inner HTML of matched element")
	queryCmd.Flags().StringVar(&qAppend, "append", "", "append child HTML to matched element")
	queryCmd.Flags().StringVar(&qSetAttr, "set-attr", "", "set attribute (NAME=VALUE)")
	queryCmd.Flags().BoolVar(&qRemove, "remove", false, "remove matched elements")
	queryCmd.Flags().BoolVar(&qAll, "all", false, "apply write to all matches")
	queryCmd.Flags().StringVar(&queryBatchPath, "batch", "", "JSON batch file path (use - for stdin)")
	queryCmd.Flags().BoolVar(&queryBatchAtomic, "atomic", true, "with --batch: apply all in memory then write (default true)")
	queryCmd.Flags().BoolVar(&queryBatchContinue, "continue-on-error", false, "with --batch: skip failed ops (only when --atomic=false)")
	rootCmd.AddCommand(queryCmd)
}
