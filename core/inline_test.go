package core

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/panyam/templar"
)

// inlineDeck creates a Deck on a MemFS with the given files, then runs inlineAssets.
func inlineDeck(t *testing.T, html string, files map[string][]byte) (*Result, error) {
	t.Helper()
	mfs := templar.NewMemFS()
	for name, data := range files {
		mfs.WriteFile(name, data, 0644)
	}
	d := &Deck{FS: mfs}
	return d.inlineAssets(html)
}

func TestInlineCSS(t *testing.T) {
	cssContent := "body { color: red; }"
	html := `<html><head><link rel="stylesheet" href="style.css"></head></html>`
	result, err := inlineDeck(t, html, map[string][]byte{
		"style.css": []byte(cssContent),
	})
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
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
	cssContent := "h1 { font-size: 2em; }"
	html := `<link href="main.css" rel="stylesheet">`
	result, err := inlineDeck(t, html, map[string][]byte{
		"main.css": []byte(cssContent),
	})
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}

	if !strings.Contains(result.HTML, "<style>") {
		t.Error("CSS with reversed attrs not inlined")
	}
}

func TestInlineJS(t *testing.T) {
	jsContent := "console.log('hello');"
	html := `<html><script src="app.js"></script></html>`
	result, err := inlineDeck(t, html, map[string][]byte{
		"app.js": []byte(jsContent),
	})
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}

	if !strings.Contains(result.HTML, "<script>\n"+jsContent) {
		t.Error("JS not inlined into <script> tag")
	}
	if strings.Contains(result.HTML, `src="app.js"`) {
		t.Error("<script src> still present after inlining")
	}
}

func TestInlineImage(t *testing.T) {
	// 1x1 pixel PNG
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
	html := `<img src="img.png" alt="test">`
	result, err := inlineDeck(t, html, map[string][]byte{
		"img.png": pngData,
	})
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}

	encoded := base64.StdEncoding.EncodeToString(pngData)
	if !strings.Contains(result.HTML, "data:image/png;base64,"+encoded) {
		t.Error("image not converted to data URI")
	}
}

func TestInlineImageRemoteUntouched(t *testing.T) {
	html := `<img src="https://example.com/photo.jpg" alt="remote">`
	result, err := inlineDeck(t, html, nil)
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}

	if !strings.Contains(result.HTML, "https://example.com/photo.jpg") {
		t.Error("remote image URL was modified")
	}
}

func TestInlineDataURIUntouched(t *testing.T) {
	html := `<img src="data:image/png;base64,abc" alt="data">`
	result, err := inlineDeck(t, html, nil)
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}

	if result.HTML != html {
		t.Error("data URI was modified")
	}
}

func TestInlineMissingFile(t *testing.T) {
	html := `<link rel="stylesheet" href="missing.css">`
	result, err := inlineDeck(t, html, nil)
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warning for missing file")
	}
	if !strings.Contains(result.HTML, "<link") {
		t.Error("original tag removed despite missing file")
	}
}

func TestInlineMultipleJS(t *testing.T) {
	html := `<html><script src="a.js"></script><script src="b.js"></script></html>`
	result, err := inlineDeck(t, html, map[string][]byte{
		"a.js": []byte("var a=1;"),
		"b.js": []byte("var b=2;"),
	})
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}

	if !strings.Contains(result.HTML, "var a=1;") {
		t.Error("first JS file not inlined")
	}
	if !strings.Contains(result.HTML, "var b=2;") {
		t.Error("second JS file not inlined")
	}
	if strings.Contains(result.HTML, `src=`) {
		t.Error("script src attributes still present after inlining")
	}
}

func TestInlineNoAssets(t *testing.T) {
	html := `<html><body><h1>Hello</h1></body></html>`
	result, err := inlineDeck(t, html, nil)
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}

	if result.HTML != html {
		t.Error("HTML modified when no assets to inline")
	}
	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}
}
