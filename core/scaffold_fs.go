package core

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"text/template"

	"github.com/panyam/templar"
	"gopkg.in/yaml.v3"
)

// ScaffoldOpts configures deck scaffolding.
type ScaffoldOpts struct {
	Title           string
	SlideCount      int
	Theme           *Theme // nil = use default embedded theme
	ThemeName       string // used if Theme is nil (e.g., "dark", "corporate")
	IncludeMCPAgent bool   // include MCP section in AGENT.md
	FilenameStyle   string // "numbered" (default) or "slug-only"
}

// ScaffoldDeck creates a new presentation on the given WritableFS.
// Returns an opened Deck ready for use. All files are written through the FS.
func ScaffoldDeck(fsys templar.WritableFS, opts ScaffoldOpts) (*Deck, error) {
	if opts.SlideCount < 1 {
		opts.SlideCount = 3
	}
	themeName := opts.ThemeName
	if themeName == "" {
		themeName = "default"
	}

	// Load theme if not provided
	theme := opts.Theme
	if theme == nil {
		var err error
		theme, err = LoadEmbeddedTheme(themeName)
		if err != nil {
			// Theme may not have theme.yaml (use ThemeExists check)
			if !ThemeExists(themeName) {
				available, _ := ListThemes()
				return nil, fmt.Errorf("theme %q not found (available: %s)", themeName, strings.Join(available, ", "))
			}
		}
	}

	// Create directory structure
	fsys.MkdirAll("slides", 0755)
	fsys.MkdirAll("themes", 0755)

	// Write engine files
	fsys.WriteFile("slyds.css", []byte(SlydsCSS), 0644)
	fsys.WriteFile("slyds.js", []byte(SlydsJS), 0644)
	fsys.WriteFile("slyds-export.js", []byte(SlydsExportJS), 0644)

	// Write all theme CSS files into themes/
	writeThemeFilesFS(fsys)

	// Render theme.css from template
	themeData := map[string]any{"Title": opts.Title}
	renderEmbeddedTemplateFS(fsys, themeName, "theme.css.tmpl", themeData, "theme.css")

	// Generate slide files
	scheme := SchemeForStyle(opts.FilenameStyle)
	slideFiles, err := generateSlidesFS(fsys, themeName, opts.Title, opts.SlideCount, scheme)
	if err != nil {
		return nil, err
	}

	// Render index.html. For numbered scheme, sorting alphabetically gives
	// correct order (01-title < 02-slide < 03-closing). For slug-only,
	// we preserve generation order (title, slide, closing) since
	// alphabetical would put closing before slide.
	var includes strings.Builder
	if scheme.ShouldRenumber() {
		sort.Strings(slideFiles)
	}
	for _, name := range slideFiles {
		fmt.Fprintf(&includes, "    {{# include \"slides/%s\" #}}\n", name)
	}
	indexData := map[string]any{
		"Title":      opts.Title,
		"Theme":      themeName,
		"ThemeLinks": themeLinksHTML(),
		"Includes":   includes.String(),
	}
	renderEmbeddedTemplateFS(fsys, themeName, "index.html.tmpl", indexData, "index.html")

	// Copy static assets from embedded theme
	copyEmbeddedAssetsFS(fsys, themeName)

	// Write manifest — assign slide_ids to the initial slides so every
	// scaffolded deck has full id coverage from day one.
	usedIDs := make(map[string]bool)
	var slideRecords []SlideRecord
	for _, f := range slideFiles {
		id := uniqueSlideID(usedIDs)
		slideRecords = append(slideRecords, SlideRecord{ID: id, File: f})
	}
	manifest := Manifest{
		Theme:         themeName,
		Title:         opts.Title,
		Slides:        slideRecords,
		FilenameStyle: opts.FilenameStyle,
	}
	if !opts.IncludeMCPAgent {
		f := false
		manifest.AgentIncludeMCP = &f
	}
	writeManifestFS(fsys, manifest)

	// Write AGENT.md
	writeAgentMDFS(fsys, manifest)

	// Write Claude Code skills
	writeSkillsFS(fsys)

	// Open as Deck and return
	return OpenDeck(fsys)
}

// --- FS-based internal helpers ---

// writeThemeFilesFS writes all built-in theme CSS files into themes/ via FS.
func writeThemeFilesFS(fsys templar.WritableFS) {
	for name, content := range ThemeFiles() {
		fsys.WriteFile("themes/"+name, []byte(content), 0644)
	}
}

// renderEmbeddedTemplateFS renders an embedded theme template and writes to FS.
func renderEmbeddedTemplateFS(fsys templar.WritableFS, theme, tmplName string, data any, outName string) error {
	// Try theme-specific template first, then shared
	content, err := readEmbeddedTemplate(theme, tmplName)
	if err != nil {
		return err
	}

	tmpl, err := template.New(tmplName).Parse(string(content))
	if err != nil {
		return fmt.Errorf("parse template %q: %w", tmplName, err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	return fsys.WriteFile(outName, []byte(buf.String()), 0644)
}

// readEmbeddedTemplate reads a template from the embedded FS, trying theme-specific first.
func readEmbeddedTemplate(theme, name string) ([]byte, error) {
	// Try theme-specific
	data, err := TemplatesFS.ReadFile(fmt.Sprintf("templates/%s/%s", theme, name))
	if err == nil {
		return data, nil
	}
	// Fall back to shared
	return TemplatesFS.ReadFile(fmt.Sprintf("templates/%s", name))
}

// generateSlidesFS creates slide files via FS. Returns the list of filenames.
func generateSlidesFS(fsys templar.WritableFS, theme, title string, count int, scheme NamingScheme) ([]string, error) {
	if count < 1 {
		count = 3
	}
	var files []string

	// Title slide — prefer layout system (has data-layout, data-slot)
	name := scheme.Format(1, "title")
	data := map[string]any{"Title": title, "Number": 1}
	content, err := Render("title", data)
	if err != nil {
		// Fallback to theme template
		content, err = renderSlideTemplate(theme, "slides/title.html.tmpl", data)
		if err != nil {
			return nil, fmt.Errorf("title slide: %w", err)
		}
	}
	fsys.WriteFile("slides/"+name, []byte(content), 0644)
	files = append(files, name)

	// Content slides — each placeholder gets a unique slug so slug-based
	// references are unambiguous from day one. The first content slide
	// keeps the bare "slide" slug; subsequent ones follow the same -N
	// suffix convention SlugifySlides and InsertSlide use.
	for i := 2; i < count; i++ {
		slug := "slide"
		if i > 2 {
			slug = fmt.Sprintf("slide-%d", i-1)
		}
		name = scheme.Format(i, slug)
		data = map[string]any{"Title": fmt.Sprintf("Slide %d", i), "Number": i}
		content, err = Render("content", data)
		if err != nil {
			content, err = renderSlideTemplate(theme, "slides/content.html.tmpl", data)
			if err != nil {
				return nil, fmt.Errorf("content slide %d: %w", i, err)
			}
		}
		fsys.WriteFile("slides/"+name, []byte(content), 0644)
		files = append(files, name)
	}

	// Closing slide
	name = scheme.Format(count, "closing")
	data = map[string]any{"Title": "Thank You", "Number": count}
	content, err = Render("closing", data)
	if err != nil {
		content, err = renderSlideTemplate(theme, "slides/closing.html.tmpl", data)
		if err != nil {
			return nil, fmt.Errorf("closing slide: %w", err)
		}
	}
	fsys.WriteFile("slides/"+name, []byte(content), 0644)
	files = append(files, name)

	return files, nil
}

// renderSlideTemplate renders a slide from an embedded theme template.
func renderSlideTemplate(theme, tmplPath string, data map[string]any) (string, error) {
	content, err := TemplatesFS.ReadFile(fmt.Sprintf("templates/%s/%s", theme, tmplPath))
	if err != nil {
		return "", err
	}
	tmpl, err := template.New(tmplPath).Parse(string(content))
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// copyEmbeddedAssetsFS copies non-template files from an embedded theme to FS.
func copyEmbeddedAssetsFS(fsys templar.WritableFS, theme string) {
	themeRoot := fmt.Sprintf("templates/%s", theme)
	fs.WalkDir(TemplatesFS, themeRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		// Skip template files
		if strings.HasSuffix(path, ".tmpl") || strings.HasSuffix(path, ".yaml") {
			return nil
		}
		relPath := strings.TrimPrefix(path, themeRoot+"/")
		data, err := TemplatesFS.ReadFile(path)
		if err != nil {
			return nil
		}
		fsys.WriteFile(relPath, data, 0644)
		return nil
	})
}

// writeManifestFS writes .slyds.yaml via FS. Delegates to WriteManifestFS
// (the exported yaml.Marshal path) so new Manifest fields — including the
// Slides slice from #83 — are serialized correctly. Previously this was a
// hand-formatted string that silently dropped any field not explicitly
// named; the switch to yaml.Marshal is a strict correctness improvement.
func writeManifestFS(fsys templar.WritableFS, m Manifest) error {
	return WriteManifestFS(fsys, m)
}

// readFullManifestFS reads the full Manifest (including Sources) from FS.
func readFullManifestFS(fsys templar.WritableFS) (*Manifest, error) {
	data, err := fsys.ReadFile(".slyds.yaml")
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// writeAgentMDFS writes AGENT.md via FS.
func writeAgentMDFS(fsys templar.WritableFS, m Manifest) error {
	content, err := renderAgentMD(m)
	if err != nil {
		return err
	}
	return fsys.WriteFile("AGENT.md", []byte(content), 0644)
}

