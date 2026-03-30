package core

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
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

// themesDir returns the absolute path to the themes directory.
func themesDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "themes")
}

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
	data, err := os.ReadFile(filepath.Join(themesDir(), "_base.css"))
	if err != nil {
		t.Fatalf("failed to read _base.css: %v", err)
	}
	baseVars := parseVariableSet(string(data))

	for _, v := range coreVariables {
		if !baseVars[v] {
			t.Errorf("core variable %s not defined in _base.css", v)
		}
	}
}

func TestThemeOverridesAreValidBaseVariables(t *testing.T) {
	baseData, err := os.ReadFile(filepath.Join(themesDir(), "_base.css"))
	if err != nil {
		t.Fatalf("failed to read _base.css: %v", err)
	}
	baseVars := parseVariableSet(string(baseData))

	entries, err := os.ReadDir(themesDir())
	if err != nil {
		t.Fatalf("failed to read themes dir: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == "_base.css" || !strings.HasSuffix(name, ".css") {
			continue
		}

		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(themesDir(), name))
			if err != nil {
				t.Fatalf("failed to read %s: %v", name, err)
			}

			themeVars := parseVariables(string(data))
			for _, v := range themeVars {
				if !baseVars[v] {
					t.Errorf("theme %s overrides %s which is not defined in _base.css (typo?)", name, v)
				}
			}
		})
	}
}

func TestAllBuiltInThemesExist(t *testing.T) {
	expected := []string{"default", "dark", "hacker", "corporate", "minimal"}
	for _, theme := range expected {
		path := filepath.Join(themesDir(), theme+".css")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("built-in theme file missing: %s.css", theme)
		}
	}
}

func TestThemeFilesHaveDataThemeSelector(t *testing.T) {
	selectorPattern := regexp.MustCompile(`\[data-theme="(\w+)"\]`)

	entries, err := os.ReadDir(themesDir())
	if err != nil {
		t.Fatalf("failed to read themes dir: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == "_base.css" || !strings.HasSuffix(name, ".css") {
			continue
		}

		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(themesDir(), name))
			if err != nil {
				t.Fatalf("failed to read %s: %v", name, err)
			}

			matches := selectorPattern.FindStringSubmatch(string(data))
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
	data, err := os.ReadFile(filepath.Join(themesDir(), "_base.css"))
	if err != nil {
		t.Fatalf("failed to read _base.css: %v", err)
	}

	namedTheme := regexp.MustCompile(`\[data-theme="[\w]+"\]`)
	if namedTheme.Match(data) {
		t.Error("_base.css should only contain [data-theme] (no named themes) — named overrides belong in per-theme files")
	}
}

func TestNoDuplicateVariablesInBase(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(themesDir(), "_base.css"))
	if err != nil {
		t.Fatalf("failed to read _base.css: %v", err)
	}

	matches := varDecl.FindAllStringSubmatch(string(data), -1)
	seen := map[string]bool{}
	for _, m := range matches {
		name := m[1]
		if seen[name] {
			t.Errorf("duplicate variable %s in _base.css", name)
		}
		seen[name] = true
	}
}
