package core

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/panyam/templar"
)

// minimalExternalThemeFS returns a MapFS with the minimum viable external
// theme: theme.yaml + theme.css.tmpl + 3 slide templates. No index.html.tmpl
// — the scaffold should fall back to the shared embedded template.
func minimalExternalThemeFS(name, description string) fstest.MapFS {
	return fstest.MapFS{
		"theme.yaml": &fstest.MapFile{Data: []byte(
			"name: " + name + "\n" +
				"description: " + description + "\n" +
				"slide_types:\n" +
				"  title: slides/title.html.tmpl\n" +
				"  content: slides/content.html.tmpl\n" +
				"  closing: slides/closing.html.tmpl\n",
		)},
		"theme.css.tmpl": &fstest.MapFile{Data: []byte(
			"/* {{.Title}} — " + name + " theme */\n",
		)},
		"slides/title.html.tmpl": &fstest.MapFile{Data: []byte(
			`<div class="slide active title-slide"><h1>{{.Title}}</h1></div>` + "\n",
		)},
		"slides/content.html.tmpl": &fstest.MapFile{Data: []byte(
			`<div class="slide"><h1>Slide {{.Number}}</h1><p>Content</p></div>` + "\n",
		)},
		"slides/closing.html.tmpl": &fstest.MapFile{Data: []byte(
			`<div class="slide conclusion-slide"><h1>Thank You</h1></div>` + "\n",
		)},
	}
}

// fullExternalThemeFS returns a MapFS that includes its own index.html.tmpl.
func fullExternalThemeFS(name string) fstest.MapFS {
	m := minimalExternalThemeFS(name, name+" theme")
	m["index.html.tmpl"] = &fstest.MapFile{Data: []byte(
		`<!DOCTYPE html>
<html lang="en" data-theme="{{.Theme}}">
<head><title>{{.Title}}</title></head>
<body>
{{.Includes}}</body>
</html>
`)}
	return m
}

// TestScaffoldFromThemeDir_ThemeNameInManifest verifies that the manifest
// records the theme name from theme.yaml, not the hardcoded "custom".
func TestScaffoldFromThemeDir_ThemeNameInManifest(t *testing.T) {
	mfs := templar.NewMemFS()
	themeFS := fullExternalThemeFS("acme-brand")

	if err := ScaffoldFromThemeDir(mfs, "My Talk", 3, themeFS); err != nil {
		t.Fatal(err)
	}

	manifest, err := ReadManifestFS(mfs)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Theme != "acme-brand" {
		t.Errorf("manifest.Theme = %q, want %q", manifest.Theme, "acme-brand")
	}
}

// TestScaffoldFromThemeDir_ThemeNameInIndexHTML verifies that index.html
// uses the actual theme name from theme.yaml in data-theme, not "custom".
func TestScaffoldFromThemeDir_ThemeNameInIndexHTML(t *testing.T) {
	mfs := templar.NewMemFS()
	themeFS := fullExternalThemeFS("neon-glow")

	if err := ScaffoldFromThemeDir(mfs, "Neon Talk", 3, themeFS); err != nil {
		t.Fatal(err)
	}

	indexHTML := readFile(t, mfs, "index.html")
	if !strings.Contains(indexHTML, `data-theme="neon-glow"`) {
		t.Errorf("index.html missing data-theme=\"neon-glow\":\n%s", indexHTML)
	}
}

// TestScaffoldFromThemeDir_FallbackIndexTemplate verifies that when the
// external theme does NOT provide index.html.tmpl, ScaffoldFromThemeDir
// falls back to the shared embedded template instead of failing.
func TestScaffoldFromThemeDir_FallbackIndexTemplate(t *testing.T) {
	mfs := templar.NewMemFS()
	themeFS := minimalExternalThemeFS("minimal-ext", "A minimal external theme")

	err := ScaffoldFromThemeDir(mfs, "Fallback Test", 3, themeFS)
	if err != nil {
		t.Fatalf("ScaffoldFromThemeDir should fall back to embedded index.html.tmpl, got: %v", err)
	}

	// Should have generated a valid index.html
	if !hasFile(mfs, "index.html") {
		t.Fatal("index.html not generated")
	}
	indexHTML := readFile(t, mfs, "index.html")
	if !strings.Contains(indexHTML, "slideshow-container") {
		t.Error("fallback index.html missing slideshow-container — not using embedded template")
	}
	if !strings.Contains(indexHTML, `data-theme="minimal-ext"`) {
		t.Errorf("fallback index.html missing data-theme=\"minimal-ext\":\n%s", indexHTML)
	}
}

// TestScaffoldFromThemeDir_SlideContent verifies that slide files are
// generated from the external theme's templates.
func TestScaffoldFromThemeDir_SlideContent(t *testing.T) {
	mfs := templar.NewMemFS()
	themeFS := fullExternalThemeFS("test-theme")

	if err := ScaffoldFromThemeDir(mfs, "Slide Test", 3, themeFS); err != nil {
		t.Fatal(err)
	}

	// Should have 3 slide files
	entries, _ := mfs.ReadDir("slides")
	if len(entries) != 3 {
		t.Fatalf("got %d slide files, want 3", len(entries))
	}

	// Title slide should have our title
	d, err := OpenDeck(mfs)
	if err != nil {
		t.Fatal(err)
	}
	content, _ := d.GetSlideContent(1)
	if !strings.Contains(content, "Slide Test") {
		t.Errorf("title slide missing title: %s", content)
	}
}

// TestScaffoldFromThemeDir_EngineFiles verifies engine CSS/JS are written.
func TestScaffoldFromThemeDir_EngineFiles(t *testing.T) {
	mfs := templar.NewMemFS()
	themeFS := fullExternalThemeFS("test")

	ScaffoldFromThemeDir(mfs, "T", 1, themeFS)

	for _, f := range []string{"slyds.css", "slyds.js", "slyds-export.js", "theme.css"} {
		if !hasFile(mfs, f) {
			t.Errorf("missing engine file: %s", f)
		}
	}
}

// TestScaffoldFromThemeDir_ThemeCSS verifies theme.css is rendered from
// the external theme's template.
func TestScaffoldFromThemeDir_ThemeCSS(t *testing.T) {
	mfs := templar.NewMemFS()
	themeFS := fullExternalThemeFS("brand-x")

	ScaffoldFromThemeDir(mfs, "CSS Test", 3, themeFS)

	css := readFile(t, mfs, "theme.css")
	if !strings.Contains(css, "brand-x theme") {
		t.Errorf("theme.css not rendered from external template: %s", css)
	}
}

// TestScaffoldFromThemeDir_StaticAssets verifies that non-template files
// (images, fonts) from the theme FS are copied to the deck.
func TestScaffoldFromThemeDir_StaticAssets(t *testing.T) {
	themeFS := fullExternalThemeFS("with-assets")
	themeFS["images/logo.png"] = &fstest.MapFile{Data: []byte("fake-png-data")}

	mfs := templar.NewMemFS()
	ScaffoldFromThemeDir(mfs, "Assets Test", 1, themeFS)

	if !hasFile(mfs, "images/logo.png") {
		t.Error("static asset images/logo.png not copied")
	}
}
