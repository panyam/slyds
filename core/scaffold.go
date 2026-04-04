package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/panyam/templar"
)


// Create scaffolds a new presentation directory using the default theme.
// The output directory is derived from the slugified title.
func Create(title string, slideCount int) (string, error) {
	return CreateInDir(title, slideCount, "default", Slugify(title), true)
}

// CreateWithTheme scaffolds a new presentation using the given built-in theme name.
// The output directory is derived from the slugified title.
func CreateWithTheme(title string, slideCount int, theme string) (string, error) {
	return CreateInDir(title, slideCount, theme, Slugify(title), true)
}

// CreateInDir scaffolds a new presentation in the specified output directory.
// This is the path-based convenience function — delegates to ScaffoldDeck(LocalFS).
func CreateInDir(title string, slideCount int, theme string, outDir string, includeMCPInAgent bool) (string, error) {
	dir, err := filepath.Abs(outDir)
	if err != nil {
		return "", err
	}

	// Validate: directory must not exist or be empty
	if info, err := os.Stat(dir); err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("%q exists and is not a directory", outDir)
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) > 0 {
			return "", fmt.Errorf("directory %q already exists and is not empty", outDir)
		}
	}

	// Create the directory and scaffold via FS
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	fsys := templar.NewLocalFS(dir)
	_, err = ScaffoldDeck(fsys, ScaffoldOpts{
		Title:           title,
		SlideCount:      slideCount,
		ThemeName:       theme,
		IncludeMCPAgent: includeMCPInAgent,
	})
	if err != nil {
		return "", err
	}

	// Write CLAUDE.md symlink (OS-specific, can't go through WritableFS)
	claudeLink := filepath.Join(dir, "CLAUDE.md")
	os.Remove(claudeLink)
	os.Symlink("AGENT.md", claudeLink)

	return outDir, nil
}

// copyEmbeddedAssets copies non-template files (images, etc.) from an embedded theme directory.
func copyEmbeddedAssets(theme, outDir string) error {
	themeRoot := fmt.Sprintf("templates/%s", theme)
	return copyEmbeddedDir(themeRoot, themeRoot, outDir)
}

func copyEmbeddedDir(base, dir, outDir string) error {
	entries, err := TemplatesFS.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		entryPath := dir + "/" + e.Name()
		relPath := strings.TrimPrefix(entryPath, base+"/")
		if e.IsDir() {
			if e.Name() == "slides" {
				continue // slides are generated separately
			}
			if err := copyEmbeddedDir(base, entryPath, outDir); err != nil {
				return err
			}
			continue
		}
		// Skip template files — they're rendered, not copied
		if strings.HasSuffix(e.Name(), ".tmpl") || e.Name() == "theme.yaml" {
			continue
		}
		data, err := TemplatesFS.ReadFile(entryPath)
		if err != nil {
			return err
		}
		destPath := filepath.Join(outDir, relPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

// generateSlides creates slide files using layout templates.
// The first slide uses the "title" layout, middle slides use "content",
// and the last slide uses "closing".
func generateSlides(theme, title string, count int, dir string) ([]string, error) {
	var slideFiles []string

	// Title slide
	name := "01-title.html"
	content, err := renderLayout("title", map[string]any{"Title": title, "Number": 1})
	if err != nil {
		return nil, fmt.Errorf("failed to render title slide: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "slides", name), []byte(content), 0644); err != nil {
		return nil, err
	}
	slideFiles = append(slideFiles, name)

	// Content slides
	for i := 2; i < count; i++ {
		name := fmt.Sprintf("%02d-slide.html", i)
		slideTitle := fmt.Sprintf("Slide %d", i)
		content, err := renderLayout("content", map[string]any{"Title": slideTitle, "Number": i})
		if err != nil {
			return nil, fmt.Errorf("failed to render slide %d: %w", i, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "slides", name), []byte(content), 0644); err != nil {
			return nil, err
		}
		slideFiles = append(slideFiles, name)
	}

	// Closing slide
	name = fmt.Sprintf("%02d-closing.html", count)
	content, err = renderLayout("closing", map[string]any{"Title": title, "Number": count})
	if err != nil {
		return nil, fmt.Errorf("failed to render closing slide: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "slides", name), []byte(content), 0644); err != nil {
		return nil, err
	}
	slideFiles = append(slideFiles, name)

	return slideFiles, nil
}

// CreateFromDir scaffolds a presentation using a theme directory on disk.
// Used by slyds preview for external/community themes.
func CreateFromDir(outDir, title string, slideCount int, themeDir string) error {
	if err := os.MkdirAll(filepath.Join(outDir, "slides"), 0755); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(outDir, "slyds.css"), []byte(SlydsCSS), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "slyds.js"), []byte(SlydsJS), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "slyds-export.js"), []byte(SlydsExportJS), 0644); err != nil {
		return err
	}

	// Write theme CSS files into themes/ subdirectory
	if err := writeThemeFiles(outDir); err != nil {
		return err
	}

	readTmpl := func(name string) ([]byte, error) {
		return os.ReadFile(filepath.Join(themeDir, name))
	}

	themeData := map[string]any{"Title": title}
	if err := renderTemplateFrom(readTmpl, "theme.css.tmpl", themeData, filepath.Join(outDir, "theme.css")); err != nil {
		return fmt.Errorf("failed to render theme.css: %w", err)
	}

	slideFiles, err := generateSlidesFrom(readTmpl, title, slideCount, outDir)
	if err != nil {
		return err
	}

	var includes strings.Builder
	sort.Strings(slideFiles)
	for _, name := range slideFiles {
		fmt.Fprintf(&includes, "    {{# include \"slides/%s\" #}}\n", name)
	}
	indexData := map[string]any{
		"Title":      title,
		"Theme":      filepath.Base(themeDir),
		"ThemeLinks": themeLinksHTML(),
		"Includes":   includes.String(),
	}
	if err := renderTemplateFrom(readTmpl, "index.html.tmpl", indexData, filepath.Join(outDir, "index.html")); err != nil {
		return fmt.Errorf("failed to render index.html: %w", err)
	}

	// Copy static assets (images, fonts, etc.) from theme dir
	if err := copyDirAssets(themeDir, outDir); err != nil {
		return fmt.Errorf("failed to copy theme assets: %w", err)
	}

	// Write .slyds.yaml manifest
	themeName := filepath.Base(themeDir)
	if err := WriteManifest(outDir, Manifest{Theme: themeName, Title: title}); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}

// copyDirAssets copies non-template files from a disk-based theme directory.
func copyDirAssets(themeDir, outDir string) error {
	return filepath.WalkDir(themeDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(themeDir, path)
		if d.IsDir() {
			if d.Name() == "slides" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".tmpl") || d.Name() == "theme.yaml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(outDir, relPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}
		return os.WriteFile(destPath, data, 0644)
	})
}

// generateSlidesFrom creates slide files using a custom template reader.
func generateSlidesFrom(readTmpl func(string) ([]byte, error), title string, count int, dir string) ([]string, error) {
	var slideFiles []string

	name := "01-title.html"
	data := map[string]any{"Title": title, "Number": 1}
	if err := renderTemplateFrom(readTmpl, "slides/title.html.tmpl", data, filepath.Join(dir, "slides", name)); err != nil {
		return nil, fmt.Errorf("failed to render title slide: %w", err)
	}
	slideFiles = append(slideFiles, name)

	for i := 2; i < count; i++ {
		name := fmt.Sprintf("%02d-slide.html", i)
		data := map[string]any{"Title": title, "Number": i}
		if err := renderTemplateFrom(readTmpl, "slides/content.html.tmpl", data, filepath.Join(dir, "slides", name)); err != nil {
			return nil, fmt.Errorf("failed to render slide %d: %w", i, err)
		}
		slideFiles = append(slideFiles, name)
	}

	name = fmt.Sprintf("%02d-closing.html", count)
	data = map[string]any{"Title": title, "Number": count}
	if err := renderTemplateFrom(readTmpl, "slides/closing.html.tmpl", data, filepath.Join(dir, "slides", name)); err != nil {
		return nil, fmt.Errorf("failed to render closing slide: %w", err)
	}
	slideFiles = append(slideFiles, name)

	return slideFiles, nil
}

// renderTemplateFrom reads a template using a custom reader, renders it, and writes to outPath.
func renderTemplateFrom(readTmpl func(string) ([]byte, error), tmplName string, data any, outPath string) error {
	content, err := readTmpl(tmplName)
	if err != nil {
		return fmt.Errorf("template %q not found: %w", tmplName, err)
	}

	tmpl, err := template.New(tmplName).Parse(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse template %q: %w", tmplName, err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

// renderEmbeddedTemplate reads a template from the embedded FS, renders it, and writes to outPath.
// It tries the theme-specific path first (templates/<theme>/<name>), then falls back to the
// shared path (templates/<name>). This allows themes to override any template while sharing
// common ones like index.html.tmpl.
func renderEmbeddedTemplate(theme, tmplName string, data any, outPath string) error {
	// Try theme-specific first
	tmplPath := fmt.Sprintf("templates/%s/%s", theme, tmplName)
	content, err := TemplatesFS.ReadFile(tmplPath)
	if err != nil {
		// Fall back to shared template
		tmplPath = fmt.Sprintf("templates/%s", tmplName)
		content, err = TemplatesFS.ReadFile(tmplPath)
		if err != nil {
			return fmt.Errorf("template %q not found in theme %q or shared templates", tmplName, theme)
		}
	}

	tmpl, err := template.New(tmplName).Parse(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse template %q: %w", tmplName, err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

// ListThemes returns the names of all available themes from the embedded filesystem.
func ListThemes() ([]string, error) {
	entries, err := TemplatesFS.ReadDir("templates")
	if err != nil {
		return nil, fmt.Errorf("failed to read templates dir: %w", err)
	}
	var themes []string
	for _, e := range entries {
		if e.IsDir() {
			themes = append(themes, e.Name())
		}
	}
	sort.Strings(themes)
	return themes, nil
}

// ThemeExists checks if the given theme name is available in the embedded filesystem.
func ThemeExists(theme string) bool {
	themes, err := ListThemes()
	if err != nil {
		return false
	}
	for _, t := range themes {
		if t == theme {
			return true
		}
	}
	return false
}

// ParseIncludeDirectives extracts templar include lines from an index.html file
// and returns them formatted for the Includes template field.
func ParseIncludeDirectives(indexHTML string) string {
	matches := includeRe.FindAllString(indexHTML, -1)
	var buf strings.Builder
	for _, m := range matches {
		fmt.Fprintf(&buf, "    %s\n", strings.TrimSpace(m))
	}
	return buf.String()
}

// Update refreshes engine and theme files in an existing presentation.
// Path-based convenience — delegates to UpdateDeck via LocalFS.
func Update(dir, theme, title string) error {
	fsys := templar.NewLocalFS(dir)
	return UpdateDeck(fsys, theme, title)
}

// UpdateDeck refreshes engine and theme files on the given FS
// without touching slides/. Preserves existing manifest sources.
func UpdateDeck(fsys templar.WritableFS, theme, title string) error {
	if !ThemeExists(theme) {
		available, _ := ListThemes()
		return fmt.Errorf("theme %q not found (available: %s)", theme, strings.Join(available, ", "))
	}

	// Overwrite engine files
	fsys.WriteFile("slyds.css", []byte(SlydsCSS), 0644)
	fsys.WriteFile("slyds.js", []byte(SlydsJS), 0644)
	fsys.WriteFile("slyds-export.js", []byte(SlydsExportJS), 0644)

	// Update theme CSS files
	writeThemeFilesFS(fsys)

	// Re-render theme.css
	themeData := map[string]any{"Title": title}
	renderEmbeddedTemplateFS(fsys, theme, "theme.css.tmpl", themeData, "theme.css")

	// Parse existing includes from index.html
	indexBytes, err := fsys.ReadFile("index.html")
	if err != nil {
		return fmt.Errorf("failed to read index.html: %w", err)
	}
	includes := ParseIncludeDirectives(string(indexBytes))

	// Re-render index.html with preserved includes
	indexData := map[string]any{
		"Title":      title,
		"Theme":      theme,
		"ThemeLinks": themeLinksHTML(),
		"Includes":   includes,
	}
	renderEmbeddedTemplateFS(fsys, theme, "index.html.tmpl", indexData, "index.html")

	// Re-copy theme static assets
	copyEmbeddedAssetsFS(fsys, theme)

	// Write/update manifest — preserve existing sources
	manifest := Manifest{Theme: theme, Title: title}
	if existing, _ := readFullManifestFS(fsys); existing != nil {
		manifest.Sources = existing.Sources
		manifest.ModulesDir = existing.ModulesDir
		manifest.AgentIncludeMCP = existing.AgentIncludeMCP
	}
	writeManifestFS(fsys, manifest)
	writeAgentMDFS(fsys, manifest)

	return nil
}

// renderLayout renders a layout template by name with the given data.
func renderLayout(name string, data map[string]any) (string, error) {
	return Render(name, data)
}

// writeThemeFiles writes all theme CSS files into a themes/ subdirectory.
func writeThemeFiles(dir string) error {
	themesDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themesDir, 0755); err != nil {
		return fmt.Errorf("failed to create themes dir: %w", err)
	}
	for name, content := range ThemeFiles() {
		if err := os.WriteFile(filepath.Join(themesDir, name), []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write themes/%s: %w", name, err)
		}
	}
	return nil
}

// themeLinksHTML generates <link> tags for all theme CSS files in load order.
func themeLinksHTML() string {
	var buf strings.Builder
	for _, name := range ThemeFileNames() {
		fmt.Fprintf(&buf, "  <link rel=\"stylesheet\" href=\"themes/%s\">\n", name)
	}
	return buf.String()
}

// Slugify converts a title to a directory-safe slug.
func Slugify(text string) string {
	s := strings.ToLower(text)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	result := b.String()
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}
