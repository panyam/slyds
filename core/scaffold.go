package core

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"text/template"

	"github.com/panyam/templar"
)

// ScaffoldFromThemeDir creates a presentation on the given WritableFS using a theme
// loaded from an external fs.FS (e.g., a disk-based theme directory).
// Used by preview for community/external themes.
func ScaffoldFromThemeDir(fsys templar.WritableFS, title string, slideCount int, themeFS fs.FS) error {
	fsys.MkdirAll("slides", 0755)
	fsys.MkdirAll("themes", 0755)

	// Write engine files
	fsys.WriteFile("slyds.css", []byte(SlydsCSS), 0644)
	fsys.WriteFile("slyds.js", []byte(SlydsJS), 0644)
	fsys.WriteFile("slyds-export.js", []byte(SlydsExportJS), 0644)

	// Write all theme CSS files into themes/
	writeThemeFilesFS(fsys)

	// Read templates from theme FS
	readTmpl := func(name string) ([]byte, error) {
		return fs.ReadFile(themeFS, name)
	}

	// Render theme.css from template
	themeData := map[string]any{"Title": title}
	if err := renderTemplateToFS(fsys, readTmpl, "theme.css.tmpl", themeData, "theme.css"); err != nil {
		return fmt.Errorf("failed to render theme.css: %w", err)
	}

	// Generate slides from theme templates
	slideFiles, err := generateSlidesFromThemeFS(fsys, readTmpl, title, slideCount, NumberedScheme{})
	if err != nil {
		return err
	}

	// Render index.html
	var includes strings.Builder
	sort.Strings(slideFiles)
	for _, name := range slideFiles {
		fmt.Fprintf(&includes, "    {{# include \"slides/%s\" #}}\n", name)
	}
	indexData := map[string]any{
		"Title":      title,
		"Theme":      "custom",
		"ThemeLinks": themeLinksHTML(),
		"Includes":   includes.String(),
	}
	if err := renderTemplateToFS(fsys, readTmpl, "index.html.tmpl", indexData, "index.html"); err != nil {
		return fmt.Errorf("failed to render index.html: %w", err)
	}

	// Copy static assets from theme FS (images, fonts, etc.)
	copyThemeAssetsToFS(fsys, themeFS)

	// Write manifest
	themeName := "custom"
	// Try to get theme name from theme.yaml
	if theme, err := LoadTheme(themeFS); err == nil && theme.Config.Name != "" {
		themeName = theme.Config.Name
	}
	if err := WriteManifestFS(fsys, Manifest{Theme: themeName, Title: title}); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}

// renderTemplateToFS reads a template via readTmpl, renders it, and writes to FS.
func renderTemplateToFS(fsys templar.WritableFS, readTmpl func(string) ([]byte, error), tmplName string, data any, outName string) error {
	content, err := readTmpl(tmplName)
	if err != nil {
		return fmt.Errorf("template %q not found: %w", tmplName, err)
	}

	tmpl, err := template.New(tmplName).Parse(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse template %q: %w", tmplName, err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	return fsys.WriteFile(outName, []byte(buf.String()), 0644)
}

// generateSlidesFromThemeFS creates slide files from a theme's templates via FS.
func generateSlidesFromThemeFS(fsys templar.WritableFS, readTmpl func(string) ([]byte, error), title string, count int, scheme NamingScheme) ([]string, error) {
	if count < 1 {
		count = 3
	}
	var files []string

	render := func(tmplPath string, data map[string]any, outName string) error {
		return renderTemplateToFS(fsys, readTmpl, tmplPath, data, "slides/"+outName)
	}

	// Title slide
	name := scheme.Format(1, "title")
	if err := render("slides/title.html.tmpl", map[string]any{"Title": title, "Number": 1}, name); err != nil {
		return nil, fmt.Errorf("title slide: %w", err)
	}
	files = append(files, name)

	// Content slides — unique slugs per slide (see generateSlidesFS comment).
	for i := 2; i < count; i++ {
		slug := "slide"
		if i > 2 {
			slug = fmt.Sprintf("slide-%d", i-1)
		}
		name = scheme.Format(i, slug)
		if err := render("slides/content.html.tmpl", map[string]any{"Title": title, "Number": i}, name); err != nil {
			return nil, fmt.Errorf("content slide %d: %w", i, err)
		}
		files = append(files, name)
	}

	// Closing slide
	name = scheme.Format(count, "closing")
	if err := render("slides/closing.html.tmpl", map[string]any{"Title": title, "Number": count}, name); err != nil {
		return nil, fmt.Errorf("closing slide: %w", err)
	}
	files = append(files, name)

	return files, nil
}

// copyThemeAssetsToFS copies non-template files from a theme FS to the deck FS.
func copyThemeAssetsToFS(fsys templar.WritableFS, themeFS fs.FS) {
	fs.WalkDir(themeFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path == "." {
			return nil
		}
		// Skip templates and config
		if strings.HasSuffix(path, ".tmpl") || strings.HasSuffix(path, ".yaml") {
			return nil
		}
		// Skip slides/ — generated separately
		if strings.HasPrefix(path, "slides/") || path == "slides" {
			return nil
		}
		data, err := fs.ReadFile(themeFS, path)
		if err != nil {
			return nil
		}
		fsys.WriteFile(path, data, 0644)
		return nil
	})
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
