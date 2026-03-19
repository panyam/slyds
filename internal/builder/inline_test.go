package builder

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInlineCSS(t *testing.T) {
	tmp := t.TempDir()

	cssContent := "body { color: red; }"
	os.WriteFile(filepath.Join(tmp, "style.css"), []byte(cssContent), 0644)

	html := `<html><head><link rel="stylesheet" href="style.css"></head></html>`
	result, err := InlineAssets(html, tmp)
	if err != nil {
		t.Fatalf("InlineAssets failed: %v", err)
	}

	if !strings.Contains(result.HTML, "<style>") {
		t.Error("CSS not inlined into <style> tag")
	}
	if !strings.Contains(result.HTML, cssContent) {
		t.Error("CSS content not found in output")
	}
	if strings.Contains(result.HTML, "<link") {
		t.Error("<link> tag still present after inlining")
	}
}

func TestInlineCSSReversedAttrs(t *testing.T) {
	tmp := t.TempDir()

	cssContent := "h1 { font-size: 2em; }"
	os.WriteFile(filepath.Join(tmp, "main.css"), []byte(cssContent), 0644)

	// href before rel
	html := `<link href="main.css" rel="stylesheet">`
	result, err := InlineAssets(html, tmp)
	if err != nil {
		t.Fatalf("InlineAssets failed: %v", err)
	}

	if !strings.Contains(result.HTML, "<style>") {
		t.Error("CSS with reversed attrs not inlined")
	}
}

func TestInlineJS(t *testing.T) {
	tmp := t.TempDir()

	jsContent := "console.log('hello');"
	os.WriteFile(filepath.Join(tmp, "app.js"), []byte(jsContent), 0644)

	html := `<html><script src="app.js"></script></html>`
	result, err := InlineAssets(html, tmp)
	if err != nil {
		t.Fatalf("InlineAssets failed: %v", err)
	}

	if !strings.Contains(result.HTML, "<script>\n"+jsContent) {
		t.Error("JS not inlined into <script> tag")
	}
	if strings.Contains(result.HTML, `src="app.js"`) {
		t.Error("<script src> still present after inlining")
	}
}

func TestInlineImage(t *testing.T) {
	tmp := t.TempDir()

	// Write a tiny PNG (1x1 pixel)
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc,
		0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
	os.WriteFile(filepath.Join(tmp, "img.png"), pngData, 0644)

	html := `<img src="img.png" alt="test">`
	result, err := InlineAssets(html, tmp)
	if err != nil {
		t.Fatalf("InlineAssets failed: %v", err)
	}

	encoded := base64.StdEncoding.EncodeToString(pngData)
	if !strings.Contains(result.HTML, "data:image/png;base64,"+encoded) {
		t.Error("image not converted to data URI")
	}
}

func TestInlineImageRemoteUntouched(t *testing.T) {
	html := `<img src="https://example.com/photo.jpg" alt="remote">`
	result, err := InlineAssets(html, ".")
	if err != nil {
		t.Fatalf("InlineAssets failed: %v", err)
	}

	if !strings.Contains(result.HTML, "https://example.com/photo.jpg") {
		t.Error("remote image URL was modified")
	}
}

func TestInlineDataURIUntouched(t *testing.T) {
	html := `<img src="data:image/png;base64,abc" alt="data">`
	result, err := InlineAssets(html, ".")
	if err != nil {
		t.Fatalf("InlineAssets failed: %v", err)
	}

	if result.HTML != html {
		t.Error("data URI was modified")
	}
}

func TestInlineMissingFile(t *testing.T) {
	html := `<link rel="stylesheet" href="missing.css">`
	result, err := InlineAssets(html, ".")
	if err != nil {
		t.Fatalf("InlineAssets failed: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warning for missing file")
	}
	// Original tag should be preserved
	if !strings.Contains(result.HTML, "<link") {
		t.Error("original tag removed despite missing file")
	}
}

func TestInlineNoAssets(t *testing.T) {
	html := `<html><body><h1>Hello</h1></body></html>`
	result, err := InlineAssets(html, ".")
	if err != nil {
		t.Fatalf("InlineAssets failed: %v", err)
	}

	if result.HTML != html {
		t.Error("HTML modified when no assets to inline")
	}
	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}
}
