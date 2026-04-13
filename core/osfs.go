package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/panyam/templar"
)

// FindDeckRoot resolves an absolute path to a deck root directory on the local filesystem.
// Returns an error if dir does not contain index.html.
// This is the entry point for CLI usage — after this, use OpenDeck(templar.NewLocalFS(root)).
func FindDeckRoot(dir string) (string, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(root, "index.html")); os.IsNotExist(err) {
		return "", fmt.Errorf("no index.html found in %s — is this a slyds presentation? Run 'slyds init' to create one", root)
	}
	return root, nil
}

// OpenDeckDir is a convenience for opening a deck from a local directory path.
func OpenDeckDir(dir string) (*Deck, error) {
	root, err := FindDeckRoot(dir)
	if err != nil {
		return nil, err
	}
	return OpenDeck(templar.NewLocalFS(root))
}

// OpenDeckCwd is a convenience for opening a deck from the current working directory.
func OpenDeckCwd() (*Deck, error) {
	return OpenDeckDir(".")
}

// --- Path-based scaffold convenience functions (OS boundary) ---

// Create scaffolds a new presentation directory using the default theme.
// The output directory is derived from the slugified title.
func Create(title string, slideCount int) (string, error) {
	return CreateInDir(title, slideCount, "default", Slugify(title), true)
}

// CreateWithTheme scaffolds a new presentation using the given built-in theme name.
// The output directory is derived from the slugified title.
func CreateWithTheme(title string, slideCount int, theme string) (string, error) {
	return CreateInDir(title, slideCount, theme, Slugify(title), true)
}

// CreateInDir scaffolds a new presentation in the specified output directory.
// Validates the directory, creates it, delegates to ScaffoldDeck(LocalFS),
// and creates the CLAUDE.md symlink. This is the common convenience entry
// point; callers that need full control (e.g., filename_style) should use
// CreateInDirWithOpts instead.
func CreateInDir(title string, slideCount int, theme string, outDir string, includeMCPInAgent bool) (string, error) {
	return CreateInDirWithOpts(outDir, ScaffoldOpts{
		Title:           title,
		SlideCount:      slideCount,
		ThemeName:       theme,
		IncludeMCPAgent: includeMCPInAgent,
	})
}

// CreateInDirWithOpts scaffolds a new presentation in the specified output
// directory using the full ScaffoldOpts. Validates the directory, creates
// it, delegates to ScaffoldDeck(LocalFS), and creates the CLAUDE.md symlink.
// This is the entry point for callers that need to set filename_style or
// other scaffold options that CreateInDir doesn't expose.
func CreateInDirWithOpts(outDir string, opts ScaffoldOpts) (string, error) {
	dir, err := filepath.Abs(outDir)
	if err != nil {
		return "", err
	}

	// Validate: directory must not exist or be empty
	if info, err := os.Stat(dir); err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("%q exists and is not a directory", outDir)
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) > 0 {
			return "", fmt.Errorf("directory %q already exists and is not empty", outDir)
		}
	}

	// Create the directory and scaffold via FS
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	fsys := templar.NewLocalFS(dir)
	_, err = ScaffoldDeck(fsys, opts)
	if err != nil {
		return "", err
	}

	// Write CLAUDE.md symlink (OS-specific, can't go through WritableFS)
	claudeLink := filepath.Join(dir, "CLAUDE.md")
	os.Remove(claudeLink)
	os.Symlink("AGENT.md", claudeLink)

	return outDir, nil
}

// CreateFromDir scaffolds a presentation from a disk-based theme directory.
// Used by slyds preview for external/community themes.
func CreateFromDir(outDir, title string, slideCount int, themeDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	fsys := templar.NewLocalFS(outDir)
	themeFS := os.DirFS(themeDir)
	return ScaffoldFromThemeDir(fsys, title, slideCount, themeFS)
}

// WriteAgentMD generates AGENT.md + CLAUDE.md symlink on the local filesystem.
func WriteAgentMD(dir string, manifest Manifest) error {
	content, err := renderAgentMD(manifest)
	if err != nil {
		return err
	}

	agentPath := filepath.Join(dir, "AGENT.md")
	if err := os.WriteFile(agentPath, []byte(content), 0644); err != nil {
		return err
	}

	// Create CLAUDE.md symlink
	claudeLink := filepath.Join(dir, "CLAUDE.md")
	os.Remove(claudeLink)
	return os.Symlink("AGENT.md", claudeLink)
}
