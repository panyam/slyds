package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
)

// QueryOpts holds the options for a query operation.
type QueryOpts struct {
	HTML    bool    // return inner HTML instead of text
	Attr    string  // return attribute value
	Count   bool    // return match count
	Set     *string // set inner text
	SetHTML *string // set inner HTML
	Append  *string // append child HTML
	SetAttr *string // set attribute (NAME=VAL)
	Remove  bool    // remove matched elements
	All     bool    // apply write to all matches (not just first)
}

func (o QueryOpts) isWrite() bool {
	return o.Set != nil || o.SetHTML != nil || o.Append != nil || o.SetAttr != nil || o.Remove
}

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

// BatchFile is the JSON envelope for `slyds query --batch`.
type BatchFile struct {
	Operations []BatchOperation `json:"operations"`
}

// BatchOperation is one write operation applied in order to a slide.
type BatchOperation struct {
	Slide    string `json:"slide"`
	Selector string `json:"selector"`
	Op       string `json:"op"`
	Value    string `json:"value,omitempty"`
	All      bool   `json:"all,omitempty"`
}

var queryCmd = &cobra.Command{
	Use:   "query <slide> <selector> [dir]",
	Short: "Query or modify slide content using CSS selectors",
	Long: `Query reads or modifies slide HTML using CSS selectors (jQuery-style).

Read operations return matching content to stdout. Write operations modify
the slide file in place. Writes apply to the first match only by default;
use --all to apply to every match.

Slide can be a number (position) or a name substring.

Use --batch FILE (or - for stdin) to apply multiple write operations from JSON.
With --atomic (default), all operations are applied in memory and all files are
written only if every step succeeds; otherwise no files change.`,
	Args: cobra.RangeArgs(0, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		if queryBatchPath != "" {
			dir := "."
			switch len(args) {
			case 0:
			case 1:
				dir = args[0]
			default:
				return fmt.Errorf("with --batch, use at most one argument: [dir]")
			}
			root, err := findRootIn(dir)
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
			return runBatchQuery(root, data, queryBatchAtomic, queryBatchContinue)
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
		root, err := findRootIn(dir)
		if err != nil {
			return err
		}

		opts := QueryOpts{
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

		result, err := runQuery(root, slideRef, selector, opts)
		if err != nil {
			return err
		}
		for _, r := range result {
			fmt.Println(r)
		}
		return nil
	},
}

// resolveSlide resolves a slide reference (number or name substring) to a
// filepath relative to the slides directory.
func resolveSlide(root, ref string) (string, error) {
	slides, err := listSlidesFromIndex(root)
	if err != nil {
		return "", err
	}

	// Try as number first
	if num, err := strconv.Atoi(ref); err == nil {
		if num < 1 || num > len(slides) {
			return "", fmt.Errorf("slide %d out of range (have %d slides)", num, len(slides))
		}
		return slides[num-1], nil
	}

	// Try as name substring
	for _, s := range slides {
		if strings.Contains(s, ref) {
			return s, nil
		}
	}
	return "", fmt.Errorf("slide %q not found", ref)
}

// parseFragment parses an HTML fragment without adding <html><head><body>
// wrappers. Returns a goquery document rooted at a synthetic wrapper div.
func parseFragment(content string) (*goquery.Document, error) {
	wrapped := `<div id="__slyds_wrapper__">` + content + `</div>`
	return goquery.NewDocumentFromReader(strings.NewReader(wrapped))
}

// extractFragment extracts the inner HTML of the synthetic wrapper,
// returning just the original fragment content (possibly modified).
func extractFragment(doc *goquery.Document) (string, error) {
	return doc.Find("#__slyds_wrapper__").Html()
}

// runQuery executes a query operation on a slide. Returns result strings
// for read operations, or an empty slice for writes.
func runQuery(root, slideRef, selector string, opts QueryOpts) ([]string, error) {
	slideFile, err := resolveSlide(root, slideRef)
	if err != nil {
		return nil, err
	}
	slidePath := filepath.Join(root, "slides", slideFile)

	data, err := os.ReadFile(slidePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read slide: %w", err)
	}

	doc, err := parseFragment(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse slide HTML: %w", err)
	}

	sel := doc.Find(selector)

	// Write operations
	if opts.isWrite() {
		if sel.Length() == 0 {
			return nil, fmt.Errorf("no match for selector %q in %s", selector, slideFile)
		}
		return nil, applyWrite(doc, sel, opts, slidePath)
	}

	// Read operations
	if opts.Count {
		return []string{strconv.Itoa(sel.Length())}, nil
	}

	var results []string
	sel.Each(func(i int, s *goquery.Selection) {
		if opts.Attr != "" {
			val, _ := s.Attr(opts.Attr)
			results = append(results, val)
		} else if opts.HTML {
			html, _ := s.Html()
			results = append(results, strings.TrimSpace(html))
		} else {
			results = append(results, strings.TrimSpace(s.Text()))
		}
	})
	return results, nil
}

// applyMutation applies a write operation to the matched selection in memory.
func applyMutation(doc *goquery.Document, sel *goquery.Selection, opts QueryOpts) error {
	var targets *goquery.Selection
	if opts.All {
		targets = sel
	} else {
		targets = sel.First()
	}

	if opts.Set != nil {
		targets.Each(func(i int, s *goquery.Selection) {
			s.SetText(*opts.Set)
		})
	}

	if opts.SetHTML != nil {
		targets.Each(func(i int, s *goquery.Selection) {
			s.SetHtml(*opts.SetHTML)
		})
	}

	if opts.Append != nil {
		targets.Each(func(i int, s *goquery.Selection) {
			s.AppendHtml(*opts.Append)
		})
	}

	if opts.SetAttr != nil {
		parts := strings.SplitN(*opts.SetAttr, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("--set-attr must be NAME=VALUE, got %q", *opts.SetAttr)
		}
		targets.Each(func(i int, s *goquery.Selection) {
			s.SetAttr(parts[0], parts[1])
		})
	}

	if opts.Remove {
		if opts.All {
			sel.Remove()
		} else {
			sel.First().Remove()
		}
	}

	return nil
}

// applyWrite applies a write operation to the matched selection and writes
// the modified HTML back to disk.
func applyWrite(doc *goquery.Document, sel *goquery.Selection, opts QueryOpts, slidePath string) error {
	if err := applyMutation(doc, sel, opts); err != nil {
		return err
	}
	html, err := extractFragment(doc)
	if err != nil {
		return fmt.Errorf("failed to serialize HTML: %w", err)
	}

	return os.WriteFile(slidePath, []byte(html), 0644)
}

// runBatchQuery applies write operations from JSON. Atomic mode rolls back all disk changes on any error.
func runBatchQuery(root string, data []byte, atomic, continueOnError bool) error {
	var batch BatchFile
	if err := json.Unmarshal(data, &batch); err != nil {
		return fmt.Errorf("batch JSON: %w", err)
	}
	if len(batch.Operations) == 0 {
		return fmt.Errorf("batch JSON: no operations")
	}

	type fileState struct {
		doc *goquery.Document
	}
	files := make(map[string]*fileState)

	loadDoc := func(slideRef string) (*goquery.Document, string, error) {
		slideFile, err := resolveSlide(root, slideRef)
		if err != nil {
			return nil, "", err
		}
		slidePath := filepath.Join(root, "slides", slideFile)
		if st, ok := files[slidePath]; ok {
			return st.doc, slidePath, nil
		}
		raw, err := os.ReadFile(slidePath)
		if err != nil {
			return nil, "", err
		}
		doc, err := parseFragment(string(raw))
		if err != nil {
			return nil, "", err
		}
		files[slidePath] = &fileState{doc: doc}
		return doc, slidePath, nil
	}

	applyOne := func(op BatchOperation) error {
		opts, err := batchOpToQueryOpts(op)
		if err != nil {
			return err
		}
		if !opts.isWrite() {
			return fmt.Errorf("batch op %q: only write ops are supported (set, set-html, append, set-attr, remove)", op.Op)
		}
		doc, _, err := loadDoc(op.Slide)
		if err != nil {
			return err
		}
		sel := doc.Find(op.Selector)
		if sel.Length() == 0 {
			return fmt.Errorf("no match for selector %q on slide %q", op.Selector, op.Slide)
		}
		return applyMutation(doc, sel, opts)
	}

	if atomic {
		for i, op := range batch.Operations {
			if err := applyOne(op); err != nil {
				return fmt.Errorf("operation %d: %w", i, err)
			}
		}
		for path, st := range files {
			html, err := extractFragment(st.doc)
			if err != nil {
				return fmt.Errorf("serialize %s: %w", path, err)
			}
			if err := os.WriteFile(path, []byte(html), 0644); err != nil {
				return err
			}
		}
		return nil
	}

	// Non-atomic: apply and flush each op immediately (re-reads file each time via runQuery)
	for i, op := range batch.Operations {
		opts, err := batchOpToQueryOpts(op)
		if err != nil {
			if continueOnError {
				fmt.Fprintf(os.Stderr, "skip op %d: %v\n", i, err)
				continue
			}
			return fmt.Errorf("operation %d: %w", i, err)
		}
		if !opts.isWrite() {
			err := fmt.Errorf("only write ops supported")
			if continueOnError {
				fmt.Fprintf(os.Stderr, "skip op %d: %v\n", i, err)
				continue
			}
			return fmt.Errorf("operation %d: %w", i, err)
		}
		_, err = runQuery(root, op.Slide, op.Selector, opts)
		if err != nil {
			if continueOnError {
				fmt.Fprintf(os.Stderr, "skip op %d: %v\n", i, err)
				continue
			}
			return fmt.Errorf("operation %d: %w", i, err)
		}
	}
	return nil
}

func batchOpToQueryOpts(op BatchOperation) (QueryOpts, error) {
	opts := QueryOpts{All: op.All}
	switch strings.ToLower(strings.TrimSpace(op.Op)) {
	case "set":
		opts.Set = &op.Value
	case "set-html":
		opts.SetHTML = &op.Value
	case "append":
		opts.Append = &op.Value
	case "set-attr":
		opts.SetAttr = &op.Value
	case "remove":
		opts.Remove = true
	default:
		return QueryOpts{}, fmt.Errorf("unknown op %q (use set, set-html, append, set-attr, remove)", op.Op)
	}
	return opts, nil
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
