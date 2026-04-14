package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// IssueType categorizes a check finding.
type IssueType int

const (
	IssueError   IssueType = iota // must fix: missing file, broken reference
	IssueWarning                  // should fix: missing notes, orphan file, unknown layout
	IssueInfo                     // informational: talk time estimate
)

var issueTypeNames = [...]string{"error", "warning", "info"}

// String returns the human-readable label for an IssueType.
func (t IssueType) String() string {
	if int(t) < len(issueTypeNames) {
		return issueTypeNames[t]
	}
	return fmt.Sprintf("unknown(%d)", int(t))
}

// MarshalJSON encodes IssueType as a JSON string ("error", "warning", "info").
func (t IssueType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

// Issue is a single finding from Deck.Check().
type Issue struct {
	Type   IssueType `json:"type"`
	Slide  string    `json:"slide,omitempty"`
	Detail string    `json:"detail"`
}

// Issues is a list of check findings with filter helpers.
type Issues []Issue

// Errors returns only issues with IssueError type.
func (is Issues) Errors() Issues {
	return is.Filter(func(i Issue) bool { return i.Type == IssueError })
}

// Warnings returns only issues with IssueWarning type.
func (is Issues) Warnings() Issues {
	return is.Filter(func(i Issue) bool { return i.Type == IssueWarning })
}

// Filter returns issues matching the predicate.
func (is Issues) Filter(fn func(Issue) bool) Issues {
	var out Issues
	for _, i := range is {
		if fn(i) {
			out = append(out, i)
		}
	}
	return out
}

// Contains returns issues whose Detail contains the substring.
func (is Issues) Contains(substr string) Issues {
	return is.Filter(func(i Issue) bool {
		return strings.Contains(i.Detail, substr)
	})
}

// HasErrors returns true if any issue is an error.
func (is Issues) HasErrors() bool { return len(is.Errors()) > 0 }

// CheckResult holds the results of a deck validation.
type CheckResult struct {
	SlideCount       int     `json:"slide_count"`
	InSync           bool    `json:"in_sync"`
	Issues           Issues  `json:"issues"`
	EstimatedMinutes float64 `json:"estimated_minutes,omitempty"`
}

var assetRefRe = regexp.MustCompile(`(?:src|href)="([^"]+)"`)
var speakerNotesRe = regexp.MustCompile(`class="speaker-notes"`)
var tagStripRe = regexp.MustCompile(`<[^>]+>`)


// Check validates the deck for common issues: missing/orphan slides,
// broken asset references, missing speaker notes, unknown layouts,
// and estimated talk time. All checks go through DeckFS.
func (d *Deck) Check() (*CheckResult, error) {
	result := &CheckResult{InSync: true}

	// Get slides from index.html and from filesystem
	indexSlides, err := d.SlideFilenames()
	if err != nil {
		return nil, err
	}
	diskSlides := d.listSlideFiles()

	result.SlideCount = len(indexSlides)

	// Build sets for comparison
	indexSet := make(map[string]bool)
	for _, s := range indexSlides {
		indexSet[s] = true
	}
	diskSet := make(map[string]bool)
	for _, s := range diskSlides {
		diskSet[s] = true
	}

	// Missing files (in index but not on disk)
	for _, s := range indexSlides {
		if !diskSet[s] {
			result.Issues = append(result.Issues, Issue{
				Type:   IssueError,
				Slide:  s,
				Detail: fmt.Sprintf("%s referenced in index.html but not found on disk", s),
			})
			result.InSync = false
		}
	}

	// Orphan files (on disk but not in index)
	for _, s := range diskSlides {
		if !indexSet[s] {
			result.Issues = append(result.Issues, Issue{
				Type:   IssueWarning,
				Slide:  s,
				Detail: fmt.Sprintf("%s on disk but not in index.html (orphan)", s),
			})
			result.InSync = false
		}
	}

	// Check each slide
	totalWords := 0
	for _, s := range indexSlides {
		content, err := d.FS.ReadFile("slides/" + s)
		if err != nil {
			continue // already flagged as missing
		}
		html := string(content)

		// Speaker notes
		if !speakerNotesRe.MatchString(html) {
			result.Issues = append(result.Issues, Issue{
				Type:   IssueWarning,
				Slide:  s,
				Detail: fmt.Sprintf("%s: no speaker notes", s),
			})
		}

		// Word count in speaker notes for talk time estimate
		if idx := strings.Index(html, `class="speaker-notes"`); idx >= 0 {
			notesSection := html[idx:]
			text := tagStripRe.ReplaceAllString(notesSection, " ")
			totalWords += len(strings.Fields(text))
		}

		// Root element class
		if !hasSlideClass(html) {
			result.Issues = append(result.Issues, Issue{
				Type:   IssueError,
				Slide:  s,
				Detail: fmt.Sprintf("%s: root element missing class=\"slide\" — presentation engine requires .slide for pagination", s),
			})
		}

		// Layout attribute
		detectedLayout := DetectLayout(html)
		if !strings.Contains(html, "data-layout=") {
			result.Issues = append(result.Issues, Issue{
				Type:   IssueWarning,
				Slide:  s,
				Detail: fmt.Sprintf("%s: no data-layout attribute (detected as %q from CSS classes)", s, detectedLayout),
			})
		} else if !LayoutExists(detectedLayout) {
			result.Issues = append(result.Issues, Issue{
				Type:   IssueWarning,
				Slide:  s,
				Detail: fmt.Sprintf("%s: unknown layout %q", s, detectedLayout),
			})
		}

		// Local asset references
		matches := assetRefRe.FindAllStringSubmatch(html, -1)
		for _, m := range matches {
			ref := m[1]
			if isRemoteOrSpecialRef(ref) {
				continue
			}
			// Check if asset exists (relative to deck root or slides dir)
			if !d.assetExists(ref, s) {
				result.Issues = append(result.Issues, Issue{
					Type:   IssueWarning,
					Slide:  s,
					Detail: fmt.Sprintf("%s: broken asset reference %q", s, ref),
				})
			}
		}
	}

	// Estimate talk time: ~150 words per minute
	if totalWords > 0 {
		result.EstimatedMinutes = float64(totalWords) / 150.0
	}

	return result, nil
}

// assetExists checks if a referenced asset exists in the deck filesystem.
func (d *Deck) assetExists(ref, slideFile string) bool {
	// Try relative to deck root
	if _, err := d.FS.ReadFile(ref); err == nil {
		return true
	}
	// Try relative to slides directory
	if _, err := d.FS.ReadFile("slides/" + ref); err == nil {
		return true
	}
	return false
}

// hasSlideClass uses goquery to check whether the root element of a slide
// fragment has the "slide" CSS class (e.g. class="slide", class="slide title-slide").
// Returns false for non-standard classes like "slide-content".
func hasSlideClass(html string) bool {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return false
	}
	return doc.Find("div.slide").Length() > 0
}

// LintSlideContent checks raw HTML for common agent authoring mistakes
// before it is written to disk. Returns issues that should block the write.
func LintSlideContent(html string) Issues {
	var issues Issues
	if strings.Contains(html, `=\"`) {
		issues = append(issues, Issue{
			Type:   IssueError,
			Detail: `content contains escaped quotes (\"); this usually means the HTML was double-JSON-escaped — send raw HTML with normal quotes`,
		})
	}
	if strings.Contains(html, `\n`) && !strings.Contains(html, "\n") {
		issues = append(issues, Issue{
			Type:   IssueError,
			Detail: `content contains literal \n instead of newlines — send raw HTML, not a JSON-escaped string`,
		})
	}
	if !hasSlideClass(html) {
		issues = append(issues, Issue{
			Type:   IssueError,
			Detail: `root element must have class="slide" (not "slide-content" or other variants) — the presentation engine uses .slide for pagination and navigation`,
		})
	}
	return issues
}

// SanitizeSlideContent uses goquery to clean agent-generated HTML before
// writing. Currently strips <style> blocks that would pollute global CSS
// and break navigation. Returns the cleaned HTML and any warnings.
func SanitizeSlideContent(html string) (string, Issues) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return html, nil
	}

	var warnings Issues
	styleNodes := doc.Find("style")
	if styleNodes.Length() > 0 {
		warnings = append(warnings, Issue{
			Type:   IssueWarning,
			Detail: fmt.Sprintf("removed %d <style> block(s) — they pollute global CSS and break navigation. Use inline style= attributes on individual elements instead, or put shared styles in theme.css", styleNodes.Length()),
		})
		styleNodes.Remove()
	}

	// goquery wraps fragments in <html><body>; extract just the body content.
	out, err := doc.Find("body").Html()
	if err != nil {
		return html, warnings
	}
	return strings.TrimSpace(out), warnings
}

func isRemoteOrSpecialRef(ref string) bool {
	return strings.HasPrefix(ref, "http://") ||
		strings.HasPrefix(ref, "https://") ||
		strings.HasPrefix(ref, "#") ||
		strings.HasPrefix(ref, "data:") ||
		strings.HasSuffix(ref, ".css") ||
		strings.HasSuffix(ref, ".js")
}
