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
	html := `<html><head><link href="main.css" rel="stylesheet"></head></html>`
	result, err := inlineDeck(t, html, map[string][]byte{
		"main.css": []byte(cssContent),
	})
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}

	if !strings.Contains(result.HTML, "<style>") {
		t.Error("CSS with reversed attrs not inlined")
	}
	if !strings.Contains(result.HTML, cssContent) {
		t.Error("CSS content not found in output")
	}
}

// TestInlineCSSMultiAttrLink exercises a <link> tag that carries extra
// attributes beyond rel/href (media, integrity, crossorigin) — the kind of
// shape the old regex path would miss or mis-match.
func TestInlineCSSMultiAttrLink(t *testing.T) {
	cssContent := ".card { padding: 1rem; }"
	html := `<html><head><link rel="stylesheet" href="theme.css" media="all" integrity="sha384-abc" crossorigin="anonymous"></head></html>`
	result, err := inlineDeck(t, html, map[string][]byte{
		"theme.css": []byte(cssContent),
	})
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}
	if !strings.Contains(result.HTML, cssContent) {
		t.Error("CSS from multi-attr <link> not inlined")
	}
	if strings.Contains(result.HTML, `href="theme.css"`) {
		t.Error("original <link href> still present")
	}
}

// TestInlineCSSPreloadStylesheet confirms rel="preload stylesheet" is
// treated as a stylesheet. The rel~= matcher should cover space-separated
// rel values.
func TestInlineCSSPreloadStylesheet(t *testing.T) {
	cssContent := "body { margin: 0; }"
	html := `<html><head><link rel="preload stylesheet" as="style" href="base.css"></head></html>`
	result, err := inlineDeck(t, html, map[string][]byte{
		"base.css": []byte(cssContent),
	})
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}
	if !strings.Contains(result.HTML, cssContent) {
		t.Error("preload stylesheet not inlined")
	}
}

// TestInlineCSSRemoteUntouched confirms remote stylesheet refs are left
// alone (fetching is a separate concern and browsers handle it).
func TestInlineCSSRemoteUntouched(t *testing.T) {
	html := `<html><head><link rel="stylesheet" href="https://cdn.example.com/app.css"></head></html>`
	result, err := inlineDeck(t, html, nil)
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}
	if !strings.Contains(result.HTML, "https://cdn.example.com/app.css") {
		t.Error("remote stylesheet href was rewritten")
	}
	if !strings.Contains(result.HTML, "<link") {
		t.Error("remote <link> tag was removed")
	}
}

func TestInlineJS(t *testing.T) {
	jsContent := "console.log('hello');"
	html := `<html><body><script src="app.js"></script></body></html>`
	result, err := inlineDeck(t, html, map[string][]byte{
		"app.js": []byte(jsContent),
	})
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}

	if !strings.Contains(result.HTML, jsContent) {
		t.Error("JS not inlined into <script> tag")
	}
	if strings.Contains(result.HTML, `src="app.js"`) {
		t.Error("<script src> still present after inlining")
	}
}

// TestInlineJSModule verifies a <script type="module" src="..."> is inlined
// and its type attribute is preserved. The old regex captured a greedy
// [^>]* run between `<script` and `src=` that quietly swallowed nothing
// here but would have missed self-closing forms.
func TestInlineJSModule(t *testing.T) {
	jsContent := "export const x = 1;"
	html := `<html><body><script type="module" src="app.js" defer></script></body></html>`
	result, err := inlineDeck(t, html, map[string][]byte{
		"app.js": []byte(jsContent),
	})
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}
	if !strings.Contains(result.HTML, jsContent) {
		t.Error("module JS not inlined")
	}
	if !strings.Contains(result.HTML, `type="module"`) {
		t.Error("type=\"module\" attribute lost after inlining")
	}
	if strings.Contains(result.HTML, `src="app.js"`) {
		t.Error("src attribute not stripped from inlined script")
	}
}

// TestInlineJSRemoteUntouched confirms remote script srcs are left alone.
func TestInlineJSRemoteUntouched(t *testing.T) {
	html := `<html><body><script src="https://cdn.example.com/lib.js"></script></body></html>`
	result, err := inlineDeck(t, html, nil)
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}
	if !strings.Contains(result.HTML, "https://cdn.example.com/lib.js") {
		t.Error("remote script src was rewritten")
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
	html := `<html><body><img src="img.png" alt="test"></body></html>`
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
	// The alt text must survive the rewrite — the old regex-based
	// strings.Replace could corrupt adjacent attrs if they happened to
	// share a substring with the src.
	if !strings.Contains(result.HTML, `alt="test"`) {
		t.Error("alt attribute lost during image inlining")
	}
}

// TestInlineImageAltCollision guards against the old regex footgun where
// `strings.Replace(match, src, dataURI, 1)` would rewrite the wrong
// substring if the src value also appeared in another attribute.
func TestInlineImageAltCollision(t *testing.T) {
	pngData := []byte{0x89, 0x50, 0x4e, 0x47}
	html := `<html><body><img src="logo.png" alt="logo.png company mark"></body></html>`
	result, err := inlineDeck(t, html, map[string][]byte{
		"logo.png": pngData,
	})
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}
	// src should be a data URI; alt should still contain the literal filename.
	if !strings.Contains(result.HTML, `src="data:image/png;base64,`) {
		t.Error("src not rewritten to data URI")
	}
	if !strings.Contains(result.HTML, `alt="logo.png company mark"`) {
		t.Error("alt attribute mutated when it shared a substring with src")
	}
}

func TestInlineImageRemoteUntouched(t *testing.T) {
	html := `<html><body><img src="https://example.com/photo.jpg" alt="remote"></body></html>`
	result, err := inlineDeck(t, html, nil)
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}

	if !strings.Contains(result.HTML, "https://example.com/photo.jpg") {
		t.Error("remote image URL was modified")
	}
}

func TestInlineDataURIUntouched(t *testing.T) {
	// Data URIs must round-trip intact. We no longer assert byte-exact
	// equality (goquery normalizes the outer document on serialize), just
	// that the data URI itself is preserved.
	html := `<html><body><img src="data:image/png;base64,abc" alt="data"></body></html>`
	result, err := inlineDeck(t, html, nil)
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}

	if !strings.Contains(result.HTML, "data:image/png;base64,abc") {
		t.Error("data URI src was modified")
	}
	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings for data URI img: %v", result.Warnings)
	}
}

func TestInlineMissingFile(t *testing.T) {
	html := `<html><head><link rel="stylesheet" href="missing.css"></head></html>`
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
	html := `<html><body><script src="a.js"></script><script src="b.js"></script></body></html>`
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
	if strings.Contains(result.HTML, `src="a.js"`) || strings.Contains(result.HTML, `src="b.js"`) {
		t.Error("script src attributes still present after inlining")
	}
}

func TestInlineNoAssets(t *testing.T) {
	// No inlineable assets → fast path returns the input byte-for-byte.
	html := `<html><body><h1>Hello</h1></body></html>`
	result, err := inlineDeck(t, html, nil)
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}

	if result.HTML != html {
		t.Errorf("HTML modified when no assets to inline\nwant: %s\ngot:  %s", html, result.HTML)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}
}

// TestInlineCSSRewritesURLRefs verifies that relative `url(...)` references
// inside an inlined stylesheet are rewritten to data URIs, so themes with
// background images work in contexts without a base URL (MCP Apps
// resources, email, air-gapped exports). This is the key fidelity fix that
// brings `preview_deck` output in line with what `serve` renders.
func TestInlineCSSRewritesURLRefs(t *testing.T) {
	pngData := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	cssContent := `.hero { background: url('images/bg.jpg') center/cover; }`
	html := `<html><head><link rel="stylesheet" href="theme.css"></head></html>`
	result, err := inlineDeck(t, html, map[string][]byte{
		"theme.css":      []byte(cssContent),
		"images/bg.jpg": pngData,
	})
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}
	// The literal relative ref must be gone; a data URI must be present.
	if strings.Contains(result.HTML, "url('images/bg.jpg')") {
		t.Error("relative url() still present in inlined CSS")
	}
	if !strings.Contains(result.HTML, "url('data:image/jpeg;base64,") {
		t.Errorf("url() not rewritten to data URI\noutput: %s", result.HTML)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}
}

// TestInlineCSSURLRefsSubdir checks url() rewriting when the stylesheet
// itself lives in a subdirectory — the rewriter must resolve the ref
// against the stylesheet's directory, not the deck root.
func TestInlineCSSURLRefsSubdir(t *testing.T) {
	pngData := []byte{0x89, 0x50, 0x4e, 0x47}
	cssContent := `body { background: url("bg.jpg"); }`
	html := `<html><head><link rel="stylesheet" href="themes/dark/theme.css"></head></html>`
	result, err := inlineDeck(t, html, map[string][]byte{
		"themes/dark/theme.css": []byte(cssContent),
		"themes/dark/bg.jpg":    pngData,
	})
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}
	if !strings.Contains(result.HTML, "data:image/jpeg;base64,") {
		t.Errorf("subdir-relative url() not resolved\noutput: %s", result.HTML)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}
}

// TestInlineCSSURLRefsRemoteUntouched — @import url('https://...') and
// similar absolute refs should not be touched by the rewriter.
func TestInlineCSSURLRefsRemoteUntouched(t *testing.T) {
	cssContent := `@import url('https://fonts.googleapis.com/css2?family=Inter');
body { background: url("data:image/png;base64,abc"); }`
	html := `<html><head><link rel="stylesheet" href="theme.css"></head></html>`
	result, err := inlineDeck(t, html, map[string][]byte{
		"theme.css": []byte(cssContent),
	})
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}
	if !strings.Contains(result.HTML, "https://fonts.googleapis.com/css2?family=Inter") {
		t.Error("@import remote url() was rewritten")
	}
	if !strings.Contains(result.HTML, "data:image/png;base64,abc") {
		t.Error("existing data URI url() was modified")
	}
	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings for remote/data url()s: %v", result.Warnings)
	}
}

// TestInlineCSSURLRefMissing — a missing image referenced from url() must
// produce a warning but not fail the overall inline, and the original ref
// must be left in place so the downstream browser gets a best-effort render.
func TestInlineCSSURLRefMissing(t *testing.T) {
	cssContent := `.hero { background: url('images/missing.jpg'); }`
	html := `<html><head><link rel="stylesheet" href="theme.css"></head></html>`
	result, err := inlineDeck(t, html, map[string][]byte{
		"theme.css": []byte(cssContent),
	})
	if err != nil {
		t.Fatalf("inlineAssets failed: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Error("expected warning for missing url() target")
	}
	if !strings.Contains(result.HTML, "images/missing.jpg") {
		t.Error("missing url() ref should be left as-is (best-effort)")
	}
}
