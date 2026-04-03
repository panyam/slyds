package core

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ThemeConfig represents the theme.yaml configuration for a theme.
// It maps slide type names to their template file paths within the theme.
type ThemeConfig struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	SlideTypes  map[string]string `yaml:"slide_types"`
}

// LoadThemeConfig reads and parses the theme.yaml for the given theme name
// from the embedded filesystem.
func LoadThemeConfig(theme string) (*ThemeConfig, error) {
	path := fmt.Sprintf("templates/%s/theme.yaml", theme)
	data, err := TemplatesFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("theme config not found for %q: %w", theme, err)
	}

	var cfg ThemeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse theme.yaml for %q: %w", theme, err)
	}
	return &cfg, nil
}

// LoadThemeConfigFromDir reads and parses the theme.yaml from a theme
// directory on disk. Used for previewing external/community themes.
func LoadThemeConfigFromDir(themeDir string) (*ThemeConfig, error) {
	data, err := os.ReadFile(filepath.Join(themeDir, "theme.yaml"))
	if err != nil {
		return nil, fmt.Errorf("theme.yaml not found in %q: %w", themeDir, err)
	}

	var cfg ThemeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse theme.yaml in %q: %w", themeDir, err)
	}
	return &cfg, nil
}

// TemplateForType returns the template path for a given slide type,
// or an error if the type is not defined in the theme.
func (c *ThemeConfig) TemplateForType(slideType string) (string, error) {
	tmpl, ok := c.SlideTypes[slideType]
	if !ok {
		var available []string
		for k := range c.SlideTypes {
			available = append(available, k)
		}
		return "", fmt.Errorf("slide type %q not found in theme %q (available: %v)", slideType, c.Name, available)
	}
	return tmpl, nil
}
