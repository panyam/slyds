package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/user/slyds/assets"
)

// Create scaffolds a new presentation directory using the default theme.
func Create(title string, slideCount int) (string, error) {
	return CreateWithTheme(title, slideCount, "default")
}

// CreateWithTheme scaffolds a new presentation using the given theme name.
// Themes are embedded template sets under assets/templates/<theme>/.
func CreateWithTheme(title string, slideCount int, theme string) (string, error) {
	slug := Slugify(title)
	dir, err := filepath.Abs(slug)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(dir); err == nil {
		return "", fmt.Errorf("directory %q already exists", slug)
	}

	if err := os.MkdirAll(filepath.Join(dir, "slides"), 0755); err != nil {
		return "", err
	}

	// Write slyds.css and slyds.js from embedded assets
	if err := os.WriteFile(filepath.Join(dir, "slyds.css"), []byte(assets.SlydsCSS), 0644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "slyds.js"), []byte(assets.SlydsJS), 0644); err != nil {
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
		"Title":    title,
		"Includes": includes.String(),
	}
	if err := renderEmbeddedTemplate(theme, "index.html.tmpl", indexData, filepath.Join(dir, "index.html")); err != nil {
		return "", fmt.Errorf("failed to render index.html: %w", err)
	}

	return slug, nil
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

// renderEmbeddedTemplate reads a template from the embedded FS, renders it, and writes to outPath.
func renderEmbeddedTemplate(theme, tmplName string, data any, outPath string) error {
	tmplPath := fmt.Sprintf("templates/%s/%s", theme, tmplName)
	content, err := assets.TemplatesFS.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("template %q not found in theme %q: %w", tmplName, theme, err)
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
