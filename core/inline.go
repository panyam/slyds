package core

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/panyam/templar"
)

// cssURLRe matches `url(...)` references inside CSS. Regex on CSS is safe
// (constrained grammar, no nesting) — the CONSTRAINTS.md "no regex HTML
// mutation" rule is about HTML, not CSS.
var cssURLRe = regexp.MustCompile(`url\(\s*(?:'([^']+)'|"([^"]+)"|([^)\s'"]+))\s*\)`)

// inlineAssets inlines CSS, JS, and images into the HTML using the deck's FS.
// Uses goquery for DOM-safe mutation (per CONSTRAINTS.md). CSS `url(...)`
// references inside inlined stylesheets are also rewritten to data URIs so
// themes with background images match `serve` fidelity in contexts without
// a base URL (e.g. MCP Apps resources).
func (d *Deck) inlineAssets(html string) (*Result, error) {
	// Fast path: nothing inlineable → return input unchanged. Lets callers
	// that pass fragments or minimal HTML round-trip byte-for-byte.
	if !needsInlining(html) {
		return &Result{HTML: html}, nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var warnings []string

	// Inline CSS: <link rel="stylesheet" href="..."> → <style>...</style>
	// Uses ~= so rel="preload stylesheet" etc. are also matched.
	doc.Find(`link[rel~="stylesheet"][href]`).Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if isRemote(href) {
			return
		}
		content, err := d.resolveAsset(href)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("CSS not inlined: %s (%v)", href, err))
			return
		}
		// Rewrite url(...) inside the inlined CSS to data URIs using the
		// original stylesheet's directory as the base path.
		content = d.inlineCSSURLs(content, path.Dir(href), &warnings)
		s.ReplaceWithHtml("<style>\n" + content + "\n</style>")
	})

	// Inline JS: <script src="..."></script> → <script>...</script>
	// Covers type="module", defer, async, nonce, etc. — only the src matters.
	doc.Find(`script[src]`).Each(func(_ int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		if isRemote(src) {
			return
		}
		content, err := d.resolveAsset(src)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("JS not inlined: %s (%v)", src, err))
			return
		}
		// Strip src; goquery doesn't have RemoveAttr, so use the underlying
		// node API via Nodes[0].
		for i, a := range s.Nodes[0].Attr {
			if a.Key == "src" {
				s.Nodes[0].Attr = append(s.Nodes[0].Attr[:i], s.Nodes[0].Attr[i+1:]...)
				break
			}
		}
		s.SetHtml("\n" + content + "\n")
	})

	// Inline images: <img src="..."> → <img src="data:...">
	doc.Find(`img[src]`).Each(func(_ int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		if isRemote(src) || strings.HasPrefix(src, "data:") {
			return
		}
		data, err := d.FS.ReadFile(src)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Image not inlined: %s (%v)", src, err))
			return
		}
		s.SetAttr("src", toDataURI(src, data))
	})

	out, err := doc.Html()
	if err != nil {
		return nil, fmt.Errorf("serialize HTML: %w", err)
	}
	return &Result{HTML: out, Warnings: warnings}, nil
}

// needsInlining is a cheap check so callers that pass HTML with no external
// assets skip the parse+serialize round-trip and get byte-identical output.
func needsInlining(html string) bool {
	return strings.Contains(html, "<link") ||
		strings.Contains(html, "<script") ||
		strings.Contains(html, "<img")
}

// inlineCSSURLs rewrites relative url(...) references in CSS to data URIs,
// resolving them against baseDir (the directory of the source stylesheet).
// Remote, data:, and root-absolute refs are left untouched. Missing files
// append to warnings but don't fail the inline.
func (d *Deck) inlineCSSURLs(css, baseDir string, warnings *[]string) string {
	return cssURLRe.ReplaceAllStringFunc(css, func(match string) string {
		m := cssURLRe.FindStringSubmatch(match)
		ref := m[1]
		if ref == "" {
			ref = m[2]
		}
		if ref == "" {
			ref = m[3]
		}
		if ref == "" || isRemote(ref) || strings.HasPrefix(ref, "data:") || strings.HasPrefix(ref, "/") {
			return match
		}

		resolved := ref
		if baseDir != "" && baseDir != "." {
			resolved = path.Join(baseDir, ref)
		}
		data, err := d.FS.ReadFile(resolved)
		if err != nil {
			*warnings = append(*warnings, fmt.Sprintf("CSS url() not inlined: %s (%v)", ref, err))
			return match
		}
		return "url('" + toDataURI(resolved, data) + "')"
	})
}

// resolveAsset reads a local or remote asset. Local paths go through DeckFS.
func (d *Deck) resolveAsset(ref string) (string, error) {
	if isRemote(ref) {
		return fetchURL(ref)
	}
	data, err := d.FS.ReadFile(ref)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// isRemote reports whether ref is an http(s) URL.
func isRemote(ref string) bool {
	return strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "//")
}

// toDataURI encodes data as a base64 data URI, using the extension of ref
// to pick a MIME type.
func toDataURI(ref string, data []byte) string {
	mimeType := mime.TypeByExtension(path.Ext(ref))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))
}

// InlineAssets is the package-level function for backward compatibility.
func InlineAssets(html string, baseDir string) (*Result, error) {
	d, err := OpenDeck(templar.NewLocalFS(baseDir))
	if err != nil {
		// If no index.html, create a minimal deck just for inlining
		d = &Deck{FS: templar.NewLocalFS(baseDir)}
	}
	return d.inlineAssets(html)
}

func fetchURL(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
