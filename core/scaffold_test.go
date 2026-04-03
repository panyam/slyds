package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"My Talk", "my-talk"},
		{"Hello World 2024", "hello-world-2024"},
		{"  Spaces  Everywhere  ", "spaces-everywhere"},
		{"Special!@#Characters$%^", "special-characters"},
		{"already-slugged", "already-slugged"},
		{"UPPERCASE", "uppercase"},
		{"a", "a"},
	}
	for _, tt := range tests {
		got := Slugify(tt.input)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCreate(t *testing.T) {
	// Work in a temp directory
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	slug, err := Create("Test Talk", 3)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if slug != "test-talk" {
		t.Errorf("slug = %q, want %q", slug, "test-talk")
	}

	dir := filepath.Join(tmp, "test-talk")

	// Check required files exist
	requiredFiles := []string{
		"index.html",
		"slyds.css",
		"slyds.js",
		"slyds-export.js",
		"theme.css",
		"slides/01-title.html",
		"slides/02-slide.html",
		"slides/03-closing.html",
	}
	for _, f := range requiredFiles {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("missing file: %s", f)
		}
	}

	// Check index.html has templar include directives
	indexHTML, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("failed to read index.html: %v", err)
	}
	indexStr := string(indexHTML)

	if !strings.Contains(indexStr, `{{# include "slides/01-title.html" #}}`) {
		t.Error("index.html missing include for 01-title.html")
	}
	if !strings.Contains(indexStr, `{{# include "slides/03-closing.html" #}}`) {
		t.Error("index.html missing include for 03-closing.html")
	}
	if !strings.Contains(indexStr, "<title>Test Talk</title>") {
		t.Error("index.html missing title")
	}

	// Check slide content
	titleSlide, _ := os.ReadFile(filepath.Join(dir, "slides", "01-title.html"))
	if !strings.Contains(string(titleSlide), "Test Talk") {
		t.Error("title slide missing presentation title")
	}
	if !strings.Contains(string(titleSlide), `class="slide`) {
		t.Error("title slide missing slide class")
	}
}

func TestCreateMinSlides(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	slug, err := Create("Min Slides", 2)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	dir := filepath.Join(tmp, slug)

	// Should have exactly title + closing
	slides, err := os.ReadDir(filepath.Join(dir, "slides"))
	if err != nil {
		t.Fatalf("failed to read slides dir: %v", err)
	}
	if len(slides) != 2 {
		t.Errorf("expected 2 slides, got %d", len(slides))
	}
}

func TestCreateMoreSlides(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	_, err := Create("Many Slides", 6)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	dir := filepath.Join(tmp, "many-slides")
	slides, err := os.ReadDir(filepath.Join(dir, "slides"))
	if err != nil {
		t.Fatalf("failed to read slides dir: %v", err)
	}
	if len(slides) != 6 {
		t.Errorf("expected 6 slides, got %d", len(slides))
	}
}

func TestCreateDuplicateDir(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	_, err := Create("Dup Test", 3)
	if err != nil {
		t.Fatalf("first Create failed: %v", err)
	}

	_, err = Create("Dup Test", 3)
	if err == nil {
		t.Error("expected error for duplicate directory, got nil")
	}
}

// TestCreateInDir verifies that --dir flag routes output to the specified
// directory instead of deriving it from the slugified title.
func TestCreateInDir(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	outDir := filepath.Join(tmp, "custom", "path")
	result, err := CreateInDir("My Talk", 3, "default", outDir, true)
	if err != nil {
		t.Fatalf("CreateInDir failed: %v", err)
	}
	if result != outDir {
		t.Errorf("returned dir = %q, want %q", result, outDir)
	}

	// Verify files exist in the custom path
	if _, err := os.Stat(filepath.Join(outDir, "index.html")); os.IsNotExist(err) {
		t.Error("index.html not found in custom dir")
	}
	if _, err := os.Stat(filepath.Join(outDir, "slides", "01-title.html")); os.IsNotExist(err) {
		t.Error("slides not found in custom dir")
	}
}

// TestCreateInDirNonEmpty verifies that creating a presentation in a
// non-empty directory returns an error to prevent overwriting existing files.
func TestCreateInDirNonEmpty(t *testing.T) {
	tmp := t.TempDir()

	// Create a non-empty directory
	targetDir := filepath.Join(tmp, "existing")
	os.MkdirAll(targetDir, 0755)
	os.WriteFile(filepath.Join(targetDir, "file.txt"), []byte("existing"), 0644)

	_, err := CreateInDir("My Talk", 3, "default", targetDir, true)
	if err == nil {
		t.Error("expected error for non-empty directory, got nil")
	}
	if !strings.Contains(err.Error(), "not empty") {
		t.Errorf("error should mention 'not empty', got: %v", err)
	}
}

// TestCreateInDirEmpty verifies that creating a presentation in an
// existing but empty directory succeeds.
func TestCreateInDirEmpty(t *testing.T) {
	tmp := t.TempDir()

	targetDir := filepath.Join(tmp, "empty-dir")
	os.MkdirAll(targetDir, 0755)

	_, err := CreateInDir("My Talk", 3, "default", targetDir, true)
	if err != nil {
		t.Fatalf("CreateInDir into empty dir failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "index.html")); os.IsNotExist(err) {
		t.Error("index.html not found")
	}
}

func TestCreateInDirAgentMCPInAgentMD(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	dir := filepath.Join(tmp, "mcp-on")
	if _, err := CreateInDir("MCP On", 3, "default", dir, true); err != nil {
		t.Fatalf("CreateInDir mcp on: %v", err)
	}
	agentOn, _ := os.ReadFile(filepath.Join(dir, "AGENT.md"))
	if !strings.Contains(string(agentOn), "## MCP (Model Context Protocol)") {
		t.Error("AGENT.md should include MCP section when includeMCPInAgent is true")
	}
	manOn, err := ReadManifest(dir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if manOn.AgentIncludeMCP != nil {
		t.Errorf("manifest agent_include_mcp = %v, want nil (default include)", manOn.AgentIncludeMCP)
	}

	dirOff := filepath.Join(tmp, "mcp-off")
	if _, err := CreateInDir("MCP Off", 3, "default", dirOff, false); err != nil {
		t.Fatalf("CreateInDir mcp off: %v", err)
	}
	agentOff, _ := os.ReadFile(filepath.Join(dirOff, "AGENT.md"))
	if strings.Contains(string(agentOff), "## MCP (Model Context Protocol)") {
		t.Error("AGENT.md should omit MCP section when includeMCPInAgent is false")
	}
	manOff, err := ReadManifest(dirOff)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if manOff.AgentIncludeMCP == nil || *manOff.AgentIncludeMCP {
		t.Errorf("manifest agent_include_mcp = %v, want false", manOff.AgentIncludeMCP)
	}
}

func TestUpdatePreservesAgentIncludeMCPFalse(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	dir := filepath.Join(tmp, "preserve-mcp-flag")
	if _, err := CreateInDir("Preserve", 3, "default", dir, false); err != nil {
		t.Fatalf("CreateInDir: %v", err)
	}
	if err := Update(dir, "default", "Preserve"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	m, err := ReadManifest(dir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if m.AgentIncludeMCP == nil || *m.AgentIncludeMCP {
		t.Errorf("agent_include_mcp after update = %v, want false", m.AgentIncludeMCP)
	}
	agent, _ := os.ReadFile(filepath.Join(dir, "AGENT.md"))
	if strings.Contains(string(agent), "## MCP (Model Context Protocol)") {
		t.Error("AGENT.md should still omit MCP after update when agent_include_mcp is false")
	}
}

func TestCreateWritesManifest(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	_, err := CreateWithTheme("Manifest Test", 3, "dark")
	if err != nil {
		t.Fatalf("CreateWithTheme failed: %v", err)
	}

	dir := filepath.Join(tmp, "manifest-test")
	m, err := ReadManifest(dir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if m.Theme != "dark" {
		t.Errorf("theme = %q, want %q", m.Theme, "dark")
	}
	if m.Title != "Manifest Test" {
		t.Errorf("title = %q, want %q", m.Title, "Manifest Test")
	}
}

func TestParseIncludeDirectives(t *testing.T) {
	indexHTML := `<!DOCTYPE html>
<html>
<body>
  <div class="slideshow-container">
    {{# include "slides/01-title.html" #}}
    {{# include "slides/02-slide.html" #}}
    {{# include "slides/03-closing.html" #}}
    <div class="navigation">
    </div>
  </div>
</body>
</html>`

	got := ParseIncludeDirectives(indexHTML)

	if !strings.Contains(got, `{{# include "slides/01-title.html" #}}`) {
		t.Error("missing 01-title.html include")
	}
	if !strings.Contains(got, `{{# include "slides/02-slide.html" #}}`) {
		t.Error("missing 02-slide.html include")
	}
	if !strings.Contains(got, `{{# include "slides/03-closing.html" #}}`) {
		t.Error("missing 03-closing.html include")
	}
	// Should have exactly 3 lines (each ending with \n)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 include lines, got %d", len(lines))
	}
}

func TestParseIncludeDirectivesEmpty(t *testing.T) {
	got := ParseIncludeDirectives("<html><body>no includes here</body></html>")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestUpdate(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	// Create initial presentation
	dir := filepath.Join(tmp, "update-test")
	_, err := CreateInDir("Update Test", 3, "default", dir, true)
	if err != nil {
		t.Fatalf("CreateInDir failed: %v", err)
	}

	// Modify a slide to simulate user content
	slideFile := filepath.Join(dir, "slides", "02-slide.html")
	customContent := []byte("<div class=\"slide\"><h1>My Custom Content</h1></div>")
	if err := os.WriteFile(slideFile, customContent, 0644); err != nil {
		t.Fatalf("failed to write custom slide: %v", err)
	}

	// Corrupt slyds.css to prove update refreshes it
	if err := os.WriteFile(filepath.Join(dir, "slyds.css"), []byte("/* old */"), 0644); err != nil {
		t.Fatalf("failed to corrupt slyds.css: %v", err)
	}

	// Run update
	if err := Update(dir, "default", "Update Test"); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify slide content preserved
	got, err := os.ReadFile(slideFile)
	if err != nil {
		t.Fatalf("failed to read slide: %v", err)
	}
	if string(got) != string(customContent) {
		t.Errorf("slide content changed after update:\ngot:  %s\nwant: %s", got, customContent)
	}

	// Verify slyds.css was refreshed
	cssData, _ := os.ReadFile(filepath.Join(dir, "slyds.css"))
	if string(cssData) == "/* old */" {
		t.Error("slyds.css was not refreshed")
	}

	// Verify slyds-export.js was written during update
	exportJS, err := os.ReadFile(filepath.Join(dir, "slyds-export.js"))
	if err != nil {
		t.Fatalf("slyds-export.js not found after update: %v", err)
	}
	if len(exportJS) == 0 {
		t.Error("slyds-export.js is empty after update")
	}

	// Verify index.html still has includes
	indexData, _ := os.ReadFile(filepath.Join(dir, "index.html"))
	indexStr := string(indexData)
	if !strings.Contains(indexStr, `{{# include "slides/01-title.html" #}}`) {
		t.Error("index.html missing include for 01-title.html after update")
	}
	if !strings.Contains(indexStr, `{{# include "slides/03-closing.html" #}}`) {
		t.Error("index.html missing include for 03-closing.html after update")
	}

	// Verify manifest updated
	m, err := ReadManifest(dir)
	if err != nil {
		t.Fatalf("ReadManifest after update: %v", err)
	}
	if m.Theme != "default" || m.Title != "Update Test" {
		t.Errorf("manifest = %+v, want theme=default title=Update Test", m)
	}
}

// TestCreateHasExportButton verifies that a scaffolded presentation's index.html
// contains the export button onclick handler and a script reference to slyds-export.js,
// ensuring the client-side export feature is wired up during init.
func TestCreateHasExportButton(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	_, err := CreateInDir("Export Button Test", 3, "default", filepath.Join(tmp, "export-btn"), true)
	if err != nil {
		t.Fatalf("CreateInDir failed: %v", err)
	}

	indexHTML, err := os.ReadFile(filepath.Join(tmp, "export-btn", "index.html"))
	if err != nil {
		t.Fatalf("failed to read index.html: %v", err)
	}
	indexStr := string(indexHTML)

	if !strings.Contains(indexStr, "exportPresentation()") {
		t.Error("index.html missing exportPresentation() onclick handler")
	}
	if !strings.Contains(indexStr, `slyds-export.js`) {
		t.Error("index.html missing slyds-export.js script reference")
	}
}

// TestAllThemesHaveExportButton verifies that every built-in theme produces an
// index.html with the export button and export JS script reference, ensuring no
// theme is accidentally missing the export feature.
func TestAllThemesHaveExportButton(t *testing.T) {
	themes, err := ListThemes()
	if err != nil {
		t.Fatalf("ListThemes failed: %v", err)
	}

	for _, theme := range themes {
		t.Run(theme, func(t *testing.T) {
			tmp := t.TempDir()
			_, err := CreateInDir("Theme Test", 2, theme, filepath.Join(tmp, "deck"), true)
			if err != nil {
				t.Fatalf("CreateInDir(%s) failed: %v", theme, err)
			}

			indexHTML, err := os.ReadFile(filepath.Join(tmp, "deck", "index.html"))
			if err != nil {
				t.Fatalf("failed to read index.html: %v", err)
			}
			indexStr := string(indexHTML)

			if !strings.Contains(indexStr, "exportPresentation()") {
				t.Errorf("theme %s: index.html missing export button", theme)
			}
			if !strings.Contains(indexStr, `slyds-export.js`) {
				t.Errorf("theme %s: index.html missing export script", theme)
			}
		})
	}
}

func TestUpdateInvalidTheme(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "bad-theme")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0644)

	err := Update(dir, "nonexistent", "Test")
	if err == nil {
		t.Error("expected error for invalid theme, got nil")
	}
}

// TestCreateTitleSlideHasDataLayout verifies that a scaffolded presentation's
// title slide (first slide) has the data-layout="title" attribute, confirming
// that generateSlides uses the layout template system.
func TestCreateTitleSlideHasDataLayout(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	slug, err := Create("Layout Test", 3)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmp, slug, "slides", "01-title.html"))
	if err != nil {
		t.Fatalf("failed to read title slide: %v", err)
	}
	if !strings.Contains(string(content), `data-layout="title"`) {
		t.Error("title slide missing data-layout=\"title\" attribute")
	}
	if !strings.Contains(string(content), `data-slot="title"`) {
		t.Error("title slide missing data-slot=\"title\" attribute")
	}
}

// TestCreateContentSlideHasDataLayout verifies that scaffolded content slides
// (middle slides) have the data-layout="content" attribute.
func TestCreateContentSlideHasDataLayout(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	slug, err := Create("Layout Test", 4)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmp, slug, "slides", "02-slide.html"))
	if err != nil {
		t.Fatalf("failed to read content slide: %v", err)
	}
	if !strings.Contains(string(content), `data-layout="content"`) {
		t.Error("content slide missing data-layout=\"content\" attribute")
	}
	if !strings.Contains(string(content), `data-slot="body"`) {
		t.Error("content slide missing data-slot=\"body\" attribute")
	}
}

// TestCreateClosingSlideHasDataLayout verifies that the scaffolded closing slide
// (last slide) has the data-layout="closing" attribute.
func TestCreateClosingSlideHasDataLayout(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	slug, err := Create("Layout Test", 3)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmp, slug, "slides", "03-closing.html"))
	if err != nil {
		t.Fatalf("failed to read closing slide: %v", err)
	}
	if !strings.Contains(string(content), `data-layout="closing"`) {
		t.Error("closing slide missing data-layout=\"closing\" attribute")
	}
}

// TestCreateHasThemesDirectory verifies that a scaffolded presentation contains
// a themes/ subdirectory with the base CSS and all theme override files.
func TestCreateHasThemesDirectory(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	slug, err := Create("Themes Test", 3)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	dir := filepath.Join(tmp, slug)
	required := []string{"themes/_base.css", "themes/dark.css", "themes/default.css"}
	for _, f := range required {
		if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
			t.Errorf("missing theme file: %s", f)
		}
	}
}
