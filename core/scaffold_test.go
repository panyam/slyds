package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/templar"
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
	_, mfs := scaffoldMem(t, "Test Talk")

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
		if !hasFile(mfs, f) {
			t.Errorf("missing file: %s", f)
		}
	}

	// Check index.html has templar include directives
	indexStr := readFile(t, mfs, "index.html")
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
	titleSlide := readFile(t, mfs, "slides/01-title.html")
	if !strings.Contains(titleSlide, "Test Talk") {
		t.Error("title slide missing presentation title")
	}
	if !strings.Contains(titleSlide, `class="slide`) {
		t.Error("title slide missing slide class")
	}
}

func TestCreateMinSlides(t *testing.T) {
	d, _ := scaffoldMem(t, "Min Slides", withSlides(2))
	count, _ := d.SlideCount()
	if count != 2 {
		t.Errorf("expected 2 slides, got %d", count)
	}
}

func TestCreateMoreSlides(t *testing.T) {
	d, _ := scaffoldMem(t, "Many Slides", withSlides(6))
	count, _ := d.SlideCount()
	if count != 6 {
		t.Errorf("expected 6 slides, got %d", count)
	}
}

// TestCreateDuplicateDir tests OS-level validation — must use disk.
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

// TestCreateInDir tests OS-level dir routing — must use disk.
func TestCreateInDir(t *testing.T) {
	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "custom", "path")
	result, err := CreateInDir("My Talk", 3, "default", outDir, true)
	if err != nil {
		t.Fatalf("CreateInDir failed: %v", err)
	}
	if result != outDir {
		t.Errorf("returned dir = %q, want %q", result, outDir)
	}
	if _, err := os.Stat(filepath.Join(outDir, "index.html")); os.IsNotExist(err) {
		t.Error("index.html not found in custom dir")
	}
	if _, err := os.Stat(filepath.Join(outDir, "slides", "01-title.html")); os.IsNotExist(err) {
		t.Error("slides not found in custom dir")
	}
}

// TestCreateInDirNonEmpty tests OS-level validation — must use disk.
func TestCreateInDirNonEmpty(t *testing.T) {
	tmp := t.TempDir()
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

// TestCreateInDirEmpty tests OS-level validation — must use disk.
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
	// MCP on
	_, mfsOn := scaffoldMem(t, "MCP On", withMCP(true))
	agentOn := readFile(t, mfsOn, "AGENT.md")
	if !strings.Contains(agentOn, "## MCP (Model Context Protocol)") {
		t.Error("AGENT.md should include MCP section when includeMCPInAgent is true")
	}
	manOn, err := ReadManifestFS(mfsOn)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if manOn.AgentIncludeMCP != nil {
		t.Errorf("manifest agent_include_mcp = %v, want nil (default include)", manOn.AgentIncludeMCP)
	}

	// MCP off
	_, mfsOff := scaffoldMem(t, "MCP Off", withMCP(false))
	agentOff := readFile(t, mfsOff, "AGENT.md")
	if strings.Contains(agentOff, "## MCP (Model Context Protocol)") {
		t.Error("AGENT.md should omit MCP section when includeMCPInAgent is false")
	}
	manOff, err := ReadManifestFS(mfsOff)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if manOff.AgentIncludeMCP == nil || *manOff.AgentIncludeMCP {
		t.Errorf("manifest agent_include_mcp = %v, want false", manOff.AgentIncludeMCP)
	}
}

func TestUpdatePreservesAgentIncludeMCPFalse(t *testing.T) {
	_, mfs := scaffoldMem(t, "Preserve", withMCP(false))

	if err := UpdateDeck(mfs, "default", "Preserve"); err != nil {
		t.Fatalf("UpdateDeck: %v", err)
	}
	m, err := ReadManifestFS(mfs)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if m.AgentIncludeMCP == nil || *m.AgentIncludeMCP {
		t.Errorf("agent_include_mcp after update = %v, want false", m.AgentIncludeMCP)
	}
	agent := readFile(t, mfs, "AGENT.md")
	if strings.Contains(agent, "## MCP (Model Context Protocol)") {
		t.Error("AGENT.md should still omit MCP after update when agent_include_mcp is false")
	}
}

func TestCreateWritesManifest(t *testing.T) {
	_, mfs := scaffoldMem(t, "Manifest Test", withTheme("dark"))
	m, err := ReadManifestFS(mfs)
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
	_, mfs := scaffoldMem(t, "Update Test")

	// Modify a slide to simulate user content
	customContent := "<div class=\"slide\"><h1>My Custom Content</h1></div>"
	mfs.WriteFile("slides/02-slide.html", []byte(customContent), 0644)

	// Corrupt slyds.css to prove update refreshes it
	mfs.WriteFile("slyds.css", []byte("/* old */"), 0644)

	// Run update
	if err := UpdateDeck(mfs, "default", "Update Test"); err != nil {
		t.Fatalf("UpdateDeck failed: %v", err)
	}

	// Verify slide content preserved
	got := readFile(t, mfs, "slides/02-slide.html")
	if got != customContent {
		t.Errorf("slide content changed after update:\ngot:  %s\nwant: %s", got, customContent)
	}

	// Verify slyds.css was refreshed
	css := readFile(t, mfs, "slyds.css")
	if css == "/* old */" {
		t.Error("slyds.css was not refreshed")
	}

	// Verify slyds-export.js was written during update
	exportJS := readFile(t, mfs, "slyds-export.js")
	if len(exportJS) == 0 {
		t.Error("slyds-export.js is empty after update")
	}

	// Verify index.html still has includes
	indexStr := readFile(t, mfs, "index.html")
	if !strings.Contains(indexStr, `{{# include "slides/01-title.html" #}}`) {
		t.Error("index.html missing include for 01-title.html after update")
	}
	if !strings.Contains(indexStr, `{{# include "slides/03-closing.html" #}}`) {
		t.Error("index.html missing include for 03-closing.html after update")
	}

	// Verify manifest updated
	m, err := ReadManifestFS(mfs)
	if err != nil {
		t.Fatalf("ReadManifest after update: %v", err)
	}
	if m.Theme != "default" || m.Title != "Update Test" {
		t.Errorf("manifest = %+v, want theme=default title=Update Test", m)
	}
}

func TestCreateHasExportButton(t *testing.T) {
	_, mfs := scaffoldMem(t, "Export Button Test")

	indexStr := readFile(t, mfs, "index.html")
	if !strings.Contains(indexStr, "exportPresentation()") {
		t.Error("index.html missing exportPresentation() onclick handler")
	}
	if !strings.Contains(indexStr, `slyds-export.js`) {
		t.Error("index.html missing slyds-export.js script reference")
	}
}

func TestAllThemesHaveExportButton(t *testing.T) {
	themes, err := ListThemes()
	if err != nil {
		t.Fatalf("ListThemes failed: %v", err)
	}

	for _, theme := range themes {
		t.Run(theme, func(t *testing.T) {
			_, mfs := scaffoldMem(t, "Theme Test", withTheme(theme), withSlides(2))
			indexStr := readFile(t, mfs, "index.html")

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
	mfs := templar.NewMemFS()
	mfs.WriteFile("index.html", []byte("<html></html>"), 0644)

	err := UpdateDeck(mfs, "nonexistent", "Test")
	if err == nil {
		t.Error("expected error for invalid theme, got nil")
	}
}

func TestCreateTitleSlideHasDataLayout(t *testing.T) {
	_, mfs := scaffoldMem(t, "Layout Test")
	content := readFile(t, mfs, "slides/01-title.html")
	if !strings.Contains(content, `data-layout="title"`) {
		t.Error("title slide missing data-layout=\"title\" attribute")
	}
	if !strings.Contains(content, `data-slot="title"`) {
		t.Error("title slide missing data-slot=\"title\" attribute")
	}
}

func TestCreateContentSlideHasDataLayout(t *testing.T) {
	_, mfs := scaffoldMem(t, "Layout Test", withSlides(4))
	content := readFile(t, mfs, "slides/02-slide.html")
	if !strings.Contains(content, `data-layout="content"`) {
		t.Error("content slide missing data-layout=\"content\" attribute")
	}
	if !strings.Contains(content, `data-slot="body"`) {
		t.Error("content slide missing data-slot=\"body\" attribute")
	}
}

func TestCreateClosingSlideHasDataLayout(t *testing.T) {
	_, mfs := scaffoldMem(t, "Layout Test")
	content := readFile(t, mfs, "slides/03-closing.html")
	if !strings.Contains(content, `data-layout="closing"`) {
		t.Error("closing slide missing data-layout=\"closing\" attribute")
	}
}

func TestCreateHasThemesDirectory(t *testing.T) {
	_, mfs := scaffoldMem(t, "Themes Test")
	required := []string{"themes/_base.css", "themes/dark.css", "themes/default.css"}
	for _, f := range required {
		if !hasFile(mfs, f) {
			t.Errorf("missing theme file: %s", f)
		}
	}
}
