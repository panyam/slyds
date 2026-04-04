package core

import (
	"fmt"
	"io/fs"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Theme represents a slyds presentation theme loaded from any fs.FS.
// A theme is a read-only filesystem containing theme.yaml and slide templates.
// It can be backed by embedded assets, a local directory, or a remote source.
type Theme struct {
	// FS is the read-only filesystem containing theme files.
	FS fs.FS

	// Config is the parsed theme.yaml.
	Config ThemeConfig
}

// LoadTheme loads a theme from any fs.FS. The FS root should contain theme.yaml.
func LoadTheme(fsys fs.FS) (*Theme, error) {
	data, err := fs.ReadFile(fsys, "theme.yaml")
	if err != nil {
		return nil, fmt.Errorf("theme.yaml not found: %w", err)
	}

	var cfg ThemeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse theme.yaml: %w", err)
	}

	return &Theme{FS: fsys, Config: cfg}, nil
}

// LoadEmbeddedTheme loads a built-in theme by name from the embedded filesystem.
func LoadEmbeddedTheme(name string) (*Theme, error) {
	sub, err := fs.Sub(TemplatesFS, "templates/"+name)
	if err != nil {
		return nil, fmt.Errorf("embedded theme %q not found: %w", name, err)
	}
	return LoadTheme(sub)
}

// RenderSlide renders a slide template for the given slide type.
// The data map is passed to the Go template (typically Title and Number).
func (t *Theme) RenderSlide(slideType string, data map[string]any) (string, error) {
	tmplFile, ok := t.Config.SlideTypes[slideType]
	if !ok {
		var available []string
		for k := range t.Config.SlideTypes {
			available = append(available, k)
		}
		return "", fmt.Errorf("slide type %q not found in theme %q (available: %v)", slideType, t.Config.Name, available)
	}

	content, err := fs.ReadFile(t.FS, tmplFile)
	if err != nil {
		return "", fmt.Errorf("slide template %q not found: %w", tmplFile, err)
	}

	tmpl, err := template.New(tmplFile).Parse(string(content))
	if err != nil {
		return "", fmt.Errorf("failed to parse slide template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderSlideWithTitle renders a slide using a name (slugified) as the title,
// with an optional explicit title override.
func (t *Theme) RenderSlideWithTitle(slideType, name string, number int, titleOverride string) (string, error) {
	displayName := slugToTitle(name)
	if titleOverride != "" {
		displayName = titleOverride
	}
	return t.RenderSlide(slideType, map[string]any{
		"Title":  displayName,
		"Number": number,
	})
}

// SlideTypes returns the list of slide type names defined in this theme.
func (t *Theme) SlideTypes() []string {
	var types []string
	for k := range t.Config.SlideTypes {
		types = append(types, k)
	}
	return types
}
