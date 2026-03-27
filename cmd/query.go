package cmd

import (
	"fmt"
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
)

var queryCmd = &cobra.Command{
	Use:   "query <slide> <selector> [dir]",
	Short: "Query or modify slide content using CSS selectors",
	Long: `Query reads or modifies slide HTML using CSS selectors (jQuery-style).

Read operations return matching content to stdout. Write operations modify
the slide file in place. Writes apply to the first match only by default;
use --all to apply to every match.

Slide can be a number (position) or a name substring.`,
	Args: cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
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

// applyWrite applies a write operation to the matched selection and writes
// the modified HTML back to disk.
func applyWrite(doc *goquery.Document, sel *goquery.Selection, opts QueryOpts, slidePath string) error {
	// Determine which elements to modify
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

	// Write back
	html, err := extractFragment(doc)
	if err != nil {
		return fmt.Errorf("failed to serialize HTML: %w", err)
	}

	return os.WriteFile(slidePath, []byte(html), 0644)
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
	rootCmd.AddCommand(queryCmd)
}
