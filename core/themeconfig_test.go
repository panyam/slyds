package core

import (
	"testing"
)

// TestLoadThemeConfig verifies that theme.yaml can be loaded for all built-in
// themes and contains the expected slide type mappings.
func TestLoadThemeConfig(t *testing.T) {
	themes := []string{"default", "minimal", "dark", "corporate"}

	for _, theme := range themes {
		cfg, err := LoadThemeConfig(theme)
		if err != nil {
			t.Fatalf("LoadThemeConfig(%q) failed: %v", theme, err)
		}

		if cfg.Name != theme {
			t.Errorf("theme %q: Name = %q, want %q", theme, cfg.Name, theme)
		}

		// All themes must have at least these core slide types
		requiredTypes := []string{"title", "content", "closing", "two-column", "section"}
		for _, st := range requiredTypes {
			if _, ok := cfg.SlideTypes[st]; !ok {
				t.Errorf("theme %q missing slide type %q", theme, st)
			}
		}
	}
}

// TestLoadThemeConfigInvalid verifies that loading a non-existent theme
// returns an error.
func TestLoadThemeConfigInvalid(t *testing.T) {
	_, err := LoadThemeConfig("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent theme config, got nil")
	}
}

// TestTemplateForType verifies that TemplateForType correctly resolves
// known types and returns errors for unknown types.
func TestTemplateForType(t *testing.T) {
	cfg, err := LoadThemeConfig("default")
	if err != nil {
		t.Fatalf("LoadThemeConfig failed: %v", err)
	}

	// Known type
	tmpl, err := cfg.TemplateForType("two-column")
	if err != nil {
		t.Fatalf("TemplateForType(two-column) failed: %v", err)
	}
	if tmpl != "slides/two-column.html.tmpl" {
		t.Errorf("TemplateForType(two-column) = %q, want %q", tmpl, "slides/two-column.html.tmpl")
	}

	// Unknown type
	_, err = cfg.TemplateForType("nonexistent")
	if err == nil {
		t.Error("expected error for unknown slide type, got nil")
	}
}
