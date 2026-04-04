package core

import (
	"regexp"
	"strings"
	"testing"
)

// coreVariables are the stable API — every base MUST define these.
var coreVariables = []string{
	"--slyds-bg",
	"--slyds-fg",
	"--slyds-accent1",
	"--slyds-heading-font",
	"--slyds-body-font",
	"--slyds-code-font",
	"--slyds-code-bg",
	"--slyds-radius",
}

// varDecl matches CSS custom property declarations like "  --slyds-bg: #fff;"
var varDecl = regexp.MustCompile(`(--slyds-[\w-]+)\s*:`)

// parseVariables extracts all --slyds-* variable names from CSS content.
func parseVariables(css string) []string {
	matches := varDecl.FindAllStringSubmatch(css, -1)
	seen := map[string]bool{}
	var vars []string
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			vars = append(vars, name)
		}
	}
	return vars
}

// parseVariableSet returns a set of variable names from CSS content.
func parseVariableSet(css string) map[string]bool {
	set := map[string]bool{}
	for _, v := range parseVariables(css) {
		set[v] = true
	}
	return set
}

func TestBaseDefinesAllCoreVariables(t *testing.T) {
	files := ThemeFiles()
	baseCSS, ok := files["_base.css"]
	if !ok {
		t.Fatal("_base.css not found in embedded themes")
	}
	baseVars := parseVariableSet(baseCSS)

	for _, v := range coreVariables {
		if !baseVars[v] {
			t.Errorf("core variable %s not defined in _base.css", v)
		}
	}
}

func TestThemeOverridesAreValidBaseVariables(t *testing.T) {
	files := ThemeFiles()
	baseVars := parseVariableSet(files["_base.css"])

	for name, css := range files {
		if name == "_base.css" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			themeVars := parseVariables(css)
			for _, v := range themeVars {
				if !baseVars[v] {
					t.Errorf("theme %s overrides %s which is not defined in _base.css (typo?)", name, v)
				}
			}
		})
	}
}

func TestAllBuiltInThemesExist(t *testing.T) {
	files := ThemeFiles()
	expected := []string{"default", "dark", "hacker", "corporate", "minimal"}
	for _, theme := range expected {
		if _, ok := files[theme+".css"]; !ok {
			t.Errorf("built-in theme file missing: %s.css", theme)
		}
	}
}

func TestThemeFilesHaveDataThemeSelector(t *testing.T) {
	selectorPattern := regexp.MustCompile(`\[data-theme="(\w+)"\]`)

	for name, css := range ThemeFiles() {
		if name == "_base.css" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			matches := selectorPattern.FindStringSubmatch(css)
			if matches == nil {
				t.Errorf("theme %s has no [data-theme=\"...\"] selector", name)
				return
			}
			expectedName := strings.TrimSuffix(name, ".css")
			if matches[1] != expectedName {
				t.Errorf("theme %s has selector [data-theme=\"%s\"] — expected [data-theme=\"%s\"]", name, matches[1], expectedName)
			}
		})
	}
}

func TestBaseHasNoNamedThemeSelector(t *testing.T) {
	baseCSS := ThemeFiles()["_base.css"]
	namedTheme := regexp.MustCompile(`\[data-theme="[\w]+"\]`)
	if namedTheme.MatchString(baseCSS) {
		t.Error("_base.css should only contain [data-theme] (no named themes) — named overrides belong in per-theme files")
	}
}

func TestNoDuplicateVariablesInBase(t *testing.T) {
	baseCSS := ThemeFiles()["_base.css"]
	matches := varDecl.FindAllStringSubmatch(baseCSS, -1)
	seen := map[string]bool{}
	for _, m := range matches {
		name := m[1]
		if seen[name] {
			t.Errorf("duplicate variable %s in _base.css", name)
		}
		seen[name] = true
	}
}
