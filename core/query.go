package core

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ErrAmbiguousSlideRef is returned by ResolveSlide when a reference matches
// more than one slide. The error message names the candidate filenames so
// callers can pick a specific one.
//
// Callers should use errors.Is(err, ErrAmbiguousSlideRef) to check for this
// condition rather than string matching.
var ErrAmbiguousSlideRef = errors.New("slide reference is ambiguous")

// QueryOpts holds the options for a CSS selector query operation on a slide.
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

// IsWrite returns true if this is a write (mutation) operation.
func (o QueryOpts) IsWrite() bool {
	return o.Set != nil || o.SetHTML != nil || o.Append != nil || o.SetAttr != nil || o.Remove
}

// BatchFile is the JSON envelope for batch query operations.
type BatchFile struct {
	Operations []BatchOperation `json:"operations"`
}

// BatchOperation is one write operation applied to a slide.
type BatchOperation struct {
	Slide    string `json:"slide"`
	Selector string `json:"selector"`
	Op       string `json:"op"`
	Value    string `json:"value,omitempty"`
	All      bool   `json:"all,omitempty"`
}

// ResolveSlide resolves a slide reference to a filename from the deck's
// slide list. Reference forms are tried in this priority order:
//
//  1. Numeric — a 1-based position number ("2" → the second slide)
//  2. Exact filename — "03-intro.html"
//  3. Exact slug — "intro" matches "NN-intro.html" for exactly one slide
//  4. Substring fallback — legacy behavior for partial filename matches
//
// Steps 3 and 4 check for ambiguity: if the reference matches more than one
// slide, the function returns ErrAmbiguousSlideRef wrapped with the list of
// candidate filenames, rather than silently picking the first match.
//
// TODO(#83): prepend a slide_id lookup branch above the slug match once
// .slyds.yaml stores per-slide metadata. slide_id is rename-safe where slug
// is not.
func (d *Deck) ResolveSlide(ref string) (string, error) {
	slides, err := d.SlideFilenames()
	if err != nil {
		return "", err
	}

	// 1. Numeric: 1-based position
	if num, err := strconv.Atoi(ref); err == nil {
		if num < 1 || num > len(slides) {
			return "", fmt.Errorf("slide %d out of range (have %d slides)", num, len(slides))
		}
		return slides[num-1], nil
	}

	// 2. Exact filename match
	for _, s := range slides {
		if s == ref {
			return s, nil
		}
	}

	// 3. Exact slug match (slug = filename without NN- prefix and .html suffix)
	var slugMatches []string
	for _, s := range slides {
		slug := strings.TrimSuffix(ExtractNamePart(s), ".html")
		if slug == ref {
			slugMatches = append(slugMatches, s)
		}
	}
	if len(slugMatches) == 1 {
		return slugMatches[0], nil
	}
	if len(slugMatches) > 1 {
		return "", fmt.Errorf("%w: slug %q matches %v", ErrAmbiguousSlideRef, ref, slugMatches)
	}

	// 4. Substring fallback (legacy behavior, now ambiguity-checked)
	var subMatches []string
	for _, s := range slides {
		if strings.Contains(s, ref) {
			subMatches = append(subMatches, s)
		}
	}
	if len(subMatches) == 1 {
		return subMatches[0], nil
	}
	if len(subMatches) > 1 {
		return "", fmt.Errorf("%w: %q matches %v", ErrAmbiguousSlideRef, ref, subMatches)
	}

	return "", fmt.Errorf("slide %q not found", ref)
}

// Query executes a CSS selector query on a slide. For read operations, returns
// the matching text/HTML/attribute values. For write operations, modifies the
// slide content in-place via DeckFS.
func (d *Deck) Query(slideRef, selector string, opts QueryOpts) ([]string, error) {
	slideFile, err := d.ResolveSlide(slideRef)
	if err != nil {
		return nil, err
	}

	data, err := d.FS.ReadFile( "slides/"+slideFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read slide: %w", err)
	}

	doc, err := parseFragment(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse slide HTML: %w", err)
	}

	sel := doc.Find(selector)

	// Write operations
	if opts.IsWrite() {
		if sel.Length() == 0 {
			return nil, fmt.Errorf("no match for selector %q in %s", selector, slideFile)
		}
		if err := applyMutation(doc, sel, opts); err != nil {
			return nil, err
		}
		html, err := extractFragment(doc)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize HTML: %w", err)
		}
		return nil, d.FS.WriteFile("slides/"+slideFile, []byte(html), 0644)
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

// BatchQuery applies batch write operations. In atomic mode, all mutations
// are applied in memory before writing to disk. Non-atomic mode writes after
// each operation.
func (d *Deck) BatchQuery(ops []BatchOperation, atomic, continueOnError bool) error {
	if atomic {
		return d.batchQueryAtomic(ops)
	}

	for i, op := range ops {
		opts, err := BatchOpToQueryOpts(op)
		if err != nil {
			if continueOnError {
				continue
			}
			return fmt.Errorf("operation %d: %w", i, err)
		}
		if !opts.IsWrite() {
			err := fmt.Errorf("only write ops supported")
			if continueOnError {
				continue
			}
			return fmt.Errorf("operation %d: %w", i, err)
		}
		_, err = d.Query(op.Slide, op.Selector, opts)
		if err != nil {
			if continueOnError {
				continue
			}
			return fmt.Errorf("operation %d: %w", i, err)
		}
	}
	return nil
}

func (d *Deck) batchQueryAtomic(ops []BatchOperation) error {
	type fileState struct {
		doc      *goquery.Document
		filename string
	}
	files := make(map[string]*fileState)

	loadDoc := func(slideRef string) (*goquery.Document, string, error) {
		slideFile, err := d.ResolveSlide(slideRef)
		if err != nil {
			return nil, "", err
		}
		if st, ok := files[slideFile]; ok {
			return st.doc, slideFile, nil
		}
		raw, err := d.FS.ReadFile( "slides/"+slideFile)
		if err != nil {
			return nil, "", err
		}
		doc, err := parseFragment(string(raw))
		if err != nil {
			return nil, "", err
		}
		files[slideFile] = &fileState{doc: doc, filename: slideFile}
		return doc, slideFile, nil
	}

	for i, op := range ops {
		opts, err := BatchOpToQueryOpts(op)
		if err != nil {
			return fmt.Errorf("operation %d: %w", i, err)
		}
		if !opts.IsWrite() {
			return fmt.Errorf("operation %d: only write ops supported", i)
		}
		doc, _, err := loadDoc(op.Slide)
		if err != nil {
			return fmt.Errorf("operation %d: %w", i, err)
		}
		sel := doc.Find(op.Selector)
		if sel.Length() == 0 {
			return fmt.Errorf("operation %d: no match for selector %q on slide %q", i, op.Selector, op.Slide)
		}
		if err := applyMutation(doc, sel, opts); err != nil {
			return fmt.Errorf("operation %d: %w", i, err)
		}
	}

	// Write all modified files
	for _, st := range files {
		html, err := extractFragment(st.doc)
		if err != nil {
			return fmt.Errorf("serialize %s: %w", st.filename, err)
		}
		if err := d.FS.WriteFile("slides/"+st.filename, []byte(html), 0644); err != nil {
			return err
		}
	}
	return nil
}

// BatchOpToQueryOpts converts a BatchOperation to QueryOpts.
func BatchOpToQueryOpts(op BatchOperation) (QueryOpts, error) {
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

// --- Internal helpers ---

func parseFragment(content string) (*goquery.Document, error) {
	wrapped := `<div id="__slyds_wrapper__">` + content + `</div>`
	return goquery.NewDocumentFromReader(strings.NewReader(wrapped))
}

func extractFragment(doc *goquery.Document) (string, error) {
	return doc.Find("#__slyds_wrapper__").Html()
}

func applyMutation(doc *goquery.Document, sel *goquery.Selection, opts QueryOpts) error {
	var targets *goquery.Selection
	if opts.All {
		targets = sel
	} else {
		targets = sel.First()
	}

	if opts.Set != nil {
		targets.Each(func(i int, s *goquery.Selection) { s.SetText(*opts.Set) })
	}
	if opts.SetHTML != nil {
		targets.Each(func(i int, s *goquery.Selection) { s.SetHtml(*opts.SetHTML) })
	}
	if opts.Append != nil {
		targets.Each(func(i int, s *goquery.Selection) { s.AppendHtml(*opts.Append) })
	}
	if opts.SetAttr != nil {
		parts := strings.SplitN(*opts.SetAttr, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("--set-attr must be NAME=VALUE, got %q", *opts.SetAttr)
		}
		targets.Each(func(i int, s *goquery.Selection) { s.SetAttr(parts[0], parts[1]) })
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
