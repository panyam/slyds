package core

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	cssRe = regexp.MustCompile(`<link\s+[^>]*(?:rel=["']stylesheet["'][^>]*href=["']([^"']+)["']|href=["']([^"']+)["'][^>]*rel=["']stylesheet["'])[^>]*/?>`)
	jsRe  = regexp.MustCompile(`<script\s+[^>]*src=["']([^"']+)["'][^>]*></script>`)
	imgRe = regexp.MustCompile(`<img\s+[^>]*src=["']([^"']+)["'][^>]*/?>`)
)

// InlineAssets inlines CSS, JS, and images into the HTML.
func InlineAssets(html string, baseDir string) (*Result, error) {
	var warnings []string

	// Inline CSS
	html = cssRe.ReplaceAllStringFunc(html, func(match string) string {
		m := cssRe.FindStringSubmatch(match)
		href := m[1]
		if href == "" {
			href = m[2]
		}
		content, err := resolveAsset(href, baseDir)
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
		content, err := resolveAsset(src, baseDir)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("JS not inlined: %s (%v)", src, err))
			return match
		}
		return "<script>\n" + content + "\n</script>"
	})

	// Inline images as base64 data URIs
	html = imgRe.ReplaceAllStringFunc(html, func(match string) string {
		m := imgRe.FindStringSubmatch(match)
		src := m[1]
		if strings.HasPrefix(src, "data:") {
			return match // already a data URI
		}
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
			return match // leave remote images
		}
		imgPath := filepath.Join(baseDir, src)
		data, err := os.ReadFile(imgPath)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Image not inlined: %s (%v)", src, err))
			return match
		}
		ext := filepath.Ext(src)
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

func resolveAsset(ref string, baseDir string) (string, error) {
	if strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "http://") {
		return fetchURL(ref)
	}
	filePath := filepath.Join(baseDir, ref)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
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
