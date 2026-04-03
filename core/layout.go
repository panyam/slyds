// Package layout provides the structural layout system for slyds slides.
// Layouts define the structural arrangement (title, two-col, section, etc.)
// independently of visual themes. Each layout is a template with named slots
// identified by data-slot attributes.
package core

import (
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// LayoutEntry describes a single layout in the registry.
type LayoutEntry struct {
	Template    string   `yaml:"template"`
	Description string   `yaml:"description"`
	Slots       []string `yaml:"slots"`
}

// LayoutRegistry holds all available layouts parsed from layouts.yaml.
type LayoutRegistry struct {
	Layouts map[string]LayoutEntry `yaml:"layouts"`
}

// dataLayoutRe matches data-layout="value" in HTML.
var dataLayoutRe = regexp.MustCompile(`data-layout="([^"]+)"`)

// classLayoutRe matches legacy CSS class-based layout indicators.
var classLayoutRe = regexp.MustCompile(`class="[^"]*\b(title-slide|layout-two-column|section-slide|conclusion-slide)\b`)

// typeToLayoutMap maps legacy --type values to layout names.
var typeToLayoutMap = map[string]string{
	"title":      "title",
	"content":    "content",
	"closing":    "closing",
	"two-column": "two-col",
	"section":    "section",
}

// LoadRegistry reads and parses layouts.yaml from the embedded filesystem.
func LoadRegistry() (*LayoutRegistry, error) {
	data, err := fs.ReadFile(LayoutsFS, "layouts/layouts.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read layouts.yaml: %w", err)
	}
	var reg LayoutRegistry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("failed to parse layouts.yaml: %w", err)
	}
	return &reg, nil
}

// ListLayouts returns sorted names of all available layouts.
func ListLayouts() ([]string, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(reg.Layouts))
	for name := range reg.Layouts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// LayoutExists checks if a layout name is available.
func LayoutExists(name string) bool {
	reg, err := LoadRegistry()
	if err != nil {
		return false
	}
	_, ok := reg.Layouts[name]
	return ok
}

// Render renders a layout template with the given data and returns the HTML.
// Data should include "Title" and "Number" keys at minimum.
func Render(name string, data map[string]any) (string, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return "", err
	}
	entry, ok := reg.Layouts[name]
	if !ok {
		available, _ := ListLayouts()
		return "", fmt.Errorf("layout %q not found (available: %s)", name, strings.Join(available, ", "))
	}

	tmplPath := "layouts/" + entry.Template
	content, err := fs.ReadFile(LayoutsFS, tmplPath)
	if err != nil {
		return "", fmt.Errorf("layout template %q not found: %w", entry.Template, err)
	}

	tmpl, err := template.New(entry.Template).Parse(string(content))
	if err != nil {
		return "", fmt.Errorf("failed to parse layout template %q: %w", entry.Template, err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// DetectLayout extracts the layout name from slide HTML.
// It first looks for a data-layout attribute, then falls back to
// heuristic detection from legacy CSS classes. Returns "content"
// if no layout can be detected (migration default).
func DetectLayout(html string) string {
	// Check for data-layout attribute
	if m := dataLayoutRe.FindStringSubmatch(html); m != nil {
		return m[1]
	}

	// Heuristic fallback for legacy slides
	if m := classLayoutRe.FindStringSubmatch(html); m != nil {
		switch m[1] {
		case "title-slide":
			return "title"
		case "layout-two-column":
			return "two-col"
		case "section-slide":
			return "section"
		case "conclusion-slide":
			return "closing"
		}
	}

	return "content"
}

// ResolveType maps a legacy --type value to a layout name.
// Returns the layout name and true if mapping exists, or the input and false otherwise.
func ResolveType(typeName string) (string, bool) {
	if layout, ok := typeToLayoutMap[typeName]; ok {
		return layout, true
	}
	// Maybe they passed a layout name directly
	if LayoutExists(typeName) {
		return typeName, true
	}
	return typeName, false
}
