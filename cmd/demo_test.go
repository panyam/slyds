package cmd

import (
	"path/filepath"
	"testing"

	"github.com/panyam/slyds/core"
)

// TestMCPTokenEnvFallback verifies that resolveMCPToken returns the SLYDS_MCP_TOKEN
// environment variable when the flag value is empty, and prefers the flag
// value when both are set. This ensures `SLYDS_MCP_TOKEN=secret slyds mcp` works
// for container and CI deployments without requiring the --token flag.
func TestMCPTokenEnvFallback(t *testing.T) {
	// Flag takes precedence over env.
	t.Setenv("SLYDS_MCP_TOKEN", "env-token")
	if got := resolveMCPToken("flag-token"); got != "flag-token" {
		t.Errorf("resolveMCPToken(flag-token) = %q, want flag-token", got)
	}

	// Env used when flag is empty.
	if got := resolveMCPToken(""); got != "env-token" {
		t.Errorf("resolveMCPToken('') = %q, want env-token", got)
	}

	// Both empty → empty.
	t.Setenv("SLYDS_MCP_TOKEN", "")
	if got := resolveMCPToken(""); got != "" {
		t.Errorf("resolveMCPToken('') with empty env = %q, want ''", got)
	}
}

// TestDemoScaffolding verifies that the demo deck scaffold logic creates
// 3 decks with correct themes, slide counts, and directory names. This
// mirrors the `make demo` Makefile target and ensures the demo setup
// produces a consistent, testable baseline for all dev-* targets.
func TestDemoScaffolding(t *testing.T) {
	root := t.TempDir()
	configs := []struct {
		title string
		theme string
		slides int
	}{
		{"Getting Started", "default", 3},
		{"Dark Mode Talk", "dark", 5},
		{"Corporate Review", "corporate", 4},
	}

	for _, c := range configs {
		slug := core.Slugify(c.title)
		dir := filepath.Join(root, slug)
		_, err := core.CreateInDir(c.title, c.slides, c.theme, dir, true)
		if err != nil {
			t.Fatalf("CreateInDir(%s): %v", c.title, err)
		}
	}

	// Verify each deck was created with correct properties.
	for _, c := range configs {
		slug := core.Slugify(c.title)
		d, err := core.OpenDeckDir(filepath.Join(root, slug))
		if err != nil {
			t.Fatalf("OpenDeckDir(%s): %v", slug, err)
		}
		if d.Title() != c.title {
			t.Errorf("%s: title = %q, want %q", slug, d.Title(), c.title)
		}
		if d.Theme() != c.theme {
			t.Errorf("%s: theme = %q, want %q", slug, d.Theme(), c.theme)
		}
		count, _ := d.SlideCount()
		if count != c.slides {
			t.Errorf("%s: slides = %d, want %d", slug, count, c.slides)
		}
	}
}

// TestMaskToken verifies that maskToken redacts tokens correctly, showing
// only the first 2 and last 2 characters with asterisks in between.
// Short tokens (4 chars or fewer) are fully masked.
func TestMaskToken(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "****"},
		{"ab", "****"},
		{"abcd", "****"},
		{"abcde", "ab****de"},
		{"my-secret-token-123", "my****23"},
	}
	for _, tc := range cases {
		if got := maskToken(tc.in); got != tc.want {
			t.Errorf("maskToken(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
