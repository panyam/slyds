package core

import (
	"testing"

	"github.com/panyam/templar"
)

// scaffoldMem creates a deck on a MemFS and returns both.
// Defaults: theme="default", slides=3, MCP agent included.
func scaffoldMem(t *testing.T, title string, opts ...func(*ScaffoldOpts)) (*Deck, *templar.MemFS) {
	t.Helper()
	mfs := templar.NewMemFS()
	so := ScaffoldOpts{
		Title:           title,
		SlideCount:      3,
		ThemeName:       "default",
		IncludeMCPAgent: true,
	}
	for _, o := range opts {
		o(&so)
	}
	d, err := ScaffoldDeck(mfs, so)
	if err != nil {
		t.Fatalf("ScaffoldDeck(%q) failed: %v", title, err)
	}
	return d, mfs
}

// Option helpers for scaffoldMem.

func withTheme(theme string) func(*ScaffoldOpts) {
	return func(o *ScaffoldOpts) { o.ThemeName = theme }
}

func withSlides(n int) func(*ScaffoldOpts) {
	return func(o *ScaffoldOpts) { o.SlideCount = n }
}

func withMCP(include bool) func(*ScaffoldOpts) {
	return func(o *ScaffoldOpts) { o.IncludeMCPAgent = include }
}

// readFile reads a file from MemFS, failing the test on error.
func readFile(t *testing.T, mfs *templar.MemFS, path string) string {
	t.Helper()
	data, err := mfs.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) failed: %v", path, err)
	}
	return string(data)
}

// hasFile checks if a file exists on MemFS.
func hasFile(mfs *templar.MemFS, path string) bool {
	return mfs.HasFile(path)
}
