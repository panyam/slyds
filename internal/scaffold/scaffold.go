package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/panyam/slyds/assets"
)

var includeRe = regexp.MustCompile(`\{\{#\s*include\s+"(slides/[^"]+)"\s*#\}\}`)

// Create scaffolds a new presentation directory using the default theme.
// The output directory is derived from the slugified title.
func Create(title string, slideCount int) (string, error) {
	return CreateInDir(title, slideCount, "default", Slugify(title))
}

// CreateWithTheme scaffolds a new presentation using the given built-in theme name.
// The output directory is derived from the slugified title.
func CreateWithTheme(title string, slideCount int, theme string) (string, error) {
	return CreateInDir(title, slideCount, theme, Slugify(title))
}

// CreateInDir scaffolds a new presentation in the specified output directory
// using the given built-in theme. The outDir can be a relative or absolute path.
func CreateInDir(title string, slideCount int, theme string, outDir string) (string, error) {
	if !ThemeExists(theme) {
		available, _ := ListThemes()
		return "", fmt.Errorf("theme %q not found (available: %s)", theme, strings.Join(available, ", "))
	}

	dir, err := filepath.Abs(outDir)
	if err != nil {
		return "", err
	}

	if info, err := os.Stat(dir); err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("%q exists and is not a directory", outDir)
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) > 0 {
			return "", fmt.Errorf("directory %q already exists and is not empty", outDir)
		}
	}

	if err := os.MkdirAll(filepath.Join(dir, "slides"), 0755); err != nil {
		return "", err
	}

	// Write engine files from embedded assets
	if err := os.WriteFile(filepath.Join(dir, "slyds.css"), []byte(assets.SlydsCSS), 0644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "slyds.js"), []byte(assets.SlydsJS), 0644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "slyds-export.js"), []byte(assets.SlydsExportJS), 0644); err != nil {
		return "", err
	}

	// Write theme CSS files into themes/ subdirectory
	if err := writeThemeFiles(dir); err != nil {
		return "", err
	}

	// Render and write theme.css from template
	themeData := map[string]any{"Title": title}
	if err := renderEmbeddedTemplate(theme, "theme.css.tmpl", themeData, filepath.Join(dir, "theme.css")); err != nil {
		return "", fmt.Errorf("failed to render theme.css: %w", err)
	}

	// Generate slide files
	slideFiles, err := generateSlides(theme, title, slideCount, dir)
	if err != nil {
		return "", err
	}

	// Render and write index.html with templar include directives
	var includes strings.Builder
	sort.Strings(slideFiles)
	for _, name := range slideFiles {
		fmt.Fprintf(&includes, "    {{# include \"slides/%s\" #}}\n", name)
	}
	indexData := map[string]any{
		"Title":      title,
		"Theme":      theme,
		"ThemeLinks": themeLinksHTML(),
		"Includes":   includes.String(),
	}
	if err := renderEmbeddedTemplate(theme, "index.html.tmpl", indexData, filepath.Join(dir, "index.html")); err != nil {
		return "", fmt.Errorf("failed to render index.html: %w", err)
	}

	// Copy static assets (images, fonts, etc.) from theme
	if err := copyEmbeddedAssets(theme, dir); err != nil {
		return "", fmt.Errorf("failed to copy theme assets: %w", err)
	}

	// Write .slyds.yaml manifest with default core source
	manifest := Manifest{
		Theme: theme,
		Title: title,
		Sources: map[string]SourceConfig{
			"core": {
				URL:  DefaultCoreURL,
				Path: DefaultCorePath,
			},
		},
	}
	if err := WriteManifest(dir, manifest); err != nil {
		return "", fmt.Errorf("failed to write manifest: %w", err)
	}

	return outDir, nil
}

// copyEmbeddedAssets copies non-template files (images, etc.) from an embedded theme directory.
func copyEmbeddedAssets(theme, outDir string) error {
	themeRoot := fmt.Sprintf("templates/%s", theme)
	return copyEmbeddedDir(themeRoot, themeRoot, outDir)
}

func copyEmbeddedDir(base, dir, outDir string) error {
	entries, err := assets.TemplatesFS.ReadDir(dir)
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
		data, err := assets.TemplatesFS.ReadFile(entryPath)
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

// generateSlides creates slide files from theme templates.
func generateSlides(theme, title string, count int, dir string) ([]string, error) {
	var slideFiles []string

	// Title slide
	name := "01-title.html"
	data := map[string]any{"Title": title, "Number": 1}
	if err := renderEmbeddedTemplate(theme, "slides/title.html.tmpl", data, filepath.Join(dir, "slides", name)); err != nil {
		return nil, fmt.Errorf("failed to render title slide: %w", err)
	}
	slideFiles = append(slideFiles, name)

	// Content slides
	for i := 2; i < count; i++ {
		name := fmt.Sprintf("%02d-slide.html", i)
		data := map[string]any{"Title": title, "Number": i}
		if err := renderEmbeddedTemplate(theme, "slides/content.html.tmpl", data, filepath.Join(dir, "slides", name)); err != nil {
			return nil, fmt.Errorf("failed to render slide %d: %w", i, err)
		}
		slideFiles = append(slideFiles, name)
	}

	// Closing slide
	name = fmt.Sprintf("%02d-closing.html", count)
	data = map[string]any{"Title": title, "Number": count}
	if err := renderEmbeddedTemplate(theme, "slides/closing.html.tmpl", data, filepath.Join(dir, "slides", name)); err != nil {
		return nil, fmt.Errorf("failed to render closing slide: %w", err)
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

	if err := os.WriteFile(filepath.Join(outDir, "slyds.css"), []byte(assets.SlydsCSS), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "slyds.js"), []byte(assets.SlydsJS), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "slyds-export.js"), []byte(assets.SlydsExportJS), 0644); err != nil {
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
	content, err := assets.TemplatesFS.ReadFile(tmplPath)
	if err != nil {
		// Fall back to shared template
		tmplPath = fmt.Sprintf("templates/%s", tmplName)
		content, err = assets.TemplatesFS.ReadFile(tmplPath)
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
	entries, err := assets.TemplatesFS.ReadDir("templates")
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

// Update refreshes engine and theme files in an existing presentation directory
// without touching the slides/ directory.
func Update(dir, theme, title string) error {
	if !ThemeExists(theme) {
		available, _ := ListThemes()
		return fmt.Errorf("theme %q not found (available: %s)", theme, strings.Join(available, ", "))
	}

	// Overwrite engine files
	if err := os.WriteFile(filepath.Join(dir, "slyds.css"), []byte(assets.SlydsCSS), 0644); err != nil {
		return fmt.Errorf("failed to write slyds.css: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "slyds.js"), []byte(assets.SlydsJS), 0644); err != nil {
		return fmt.Errorf("failed to write slyds.js: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "slyds-export.js"), []byte(assets.SlydsExportJS), 0644); err != nil {
		return fmt.Errorf("failed to write slyds-export.js: %w", err)
	}

	// Update theme CSS files
	if err := writeThemeFiles(dir); err != nil {
		return fmt.Errorf("failed to write theme files: %w", err)
	}

	// Re-render theme.css
	themeData := map[string]any{"Title": title}
	if err := renderEmbeddedTemplate(theme, "theme.css.tmpl", themeData, filepath.Join(dir, "theme.css")); err != nil {
		return fmt.Errorf("failed to render theme.css: %w", err)
	}

	// Parse existing includes from index.html
	indexPath := filepath.Join(dir, "index.html")
	indexBytes, err := os.ReadFile(indexPath)
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
	if err := renderEmbeddedTemplate(theme, "index.html.tmpl", indexData, indexPath); err != nil {
		return fmt.Errorf("failed to render index.html: %w", err)
	}

	// Re-copy theme static assets
	if err := copyEmbeddedAssets(theme, dir); err != nil {
		return fmt.Errorf("failed to copy theme assets: %w", err)
	}

	// Write/update manifest — add core source if missing (migration)
	manifest := Manifest{Theme: theme, Title: title}
	existing, err := ReadManifest(dir)
	if err == nil && existing.HasSources() {
		manifest.Sources = existing.Sources
		manifest.ModulesDir = existing.ModulesDir
	}
	if manifest.Sources == nil {
		manifest.Sources = map[string]SourceConfig{
			"core": {
				URL:  DefaultCoreURL,
				Path: DefaultCorePath,
			},
		}
	}
	if err := WriteManifest(dir, manifest); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}

// writeThemeFiles writes all theme CSS files into a themes/ subdirectory.
func writeThemeFiles(dir string) error {
	themesDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themesDir, 0755); err != nil {
		return fmt.Errorf("failed to create themes dir: %w", err)
	}
	for name, content := range assets.ThemeFiles() {
		if err := os.WriteFile(filepath.Join(themesDir, name), []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write themes/%s: %w", name, err)
		}
	}
	return nil
}

// themeLinksHTML generates <link> tags for all theme CSS files in load order.
func themeLinksHTML() string {
	var buf strings.Builder
	for _, name := range assets.ThemeFileNames() {
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
