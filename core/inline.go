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

	"github.com/panyam/templar"
)

var (
	cssRe = regexp.MustCompile(`<link\s+[^>]*(?:rel=["']stylesheet["'][^>]*href=["']([^"']+)["']|href=["']([^"']+)["'][^>]*rel=["']stylesheet["'])[^>]*/?>`)
	jsRe  = regexp.MustCompile(`<script\s+[^>]*src=["']([^"']+)["'][^>]*></script>`)
	inlineImgRe = regexp.MustCompile(`<img\s+[^>]*src=["']([^"']+)["'][^>]*/?>`)
)

// inlineAssets inlines CSS, JS, and images into the HTML using the deck's FS.
func (d *Deck) inlineAssets(html string) (*Result, error) {
	var warnings []string

	// Inline CSS
	html = cssRe.ReplaceAllStringFunc(html, func(match string) string {
		m := cssRe.FindStringSubmatch(match)
		href := m[1]
		if href == "" {
			href = m[2]
		}
		content, err := d.resolveAsset(href)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("CSS not inlined: %s (%v)", href, err))
			return match
		}
		return "<style>\n" + content + "\n</style>"
	})

	// Inline JS
	html = jsRe.ReplaceAllStringFunc(html, func(match string) string {
		m := jsRe.FindStringSubmatch(match)
		src := m[1]
		content, err := d.resolveAsset(src)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("JS not inlined: %s (%v)", src, err))
			return match
		}
		return "<script>\n" + content + "\n</script>"
	})

	// Inline images as base64 data URIs
	html = inlineImgRe.ReplaceAllStringFunc(html, func(match string) string {
		m := inlineImgRe.FindStringSubmatch(match)
		src := m[1]
		if strings.HasPrefix(src, "data:") || strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
			return match
		}
		data, err := d.FS.ReadFile(src)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Image not inlined: %s (%v)", src, err))
			return match
		}
		ext := path.Ext(src)
		mimeType := mime.TypeByExtension(ext)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)
		return strings.Replace(match, src, dataURI, 1)
	})

	return &Result{HTML: html, Warnings: warnings}, nil
}

// resolveAsset reads a local or remote asset. Local paths go through DeckFS.
func (d *Deck) resolveAsset(ref string) (string, error) {
	if strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "http://") {
		return fetchURL(ref)
	}
	data, err := d.FS.ReadFile(ref)
	if err != nil {
		return "", err
	}
	return string(data), nil
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
