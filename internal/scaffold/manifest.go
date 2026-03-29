package scaffold

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ErrManifestNotFound is returned when .slyds.yaml does not exist.
var ErrManifestNotFound = errors.New("manifest not found")

// SourceConfig represents an external template/theme dependency.
// Maps directly to templar's SourceConfig structure.
type SourceConfig struct {
	URL     string   `yaml:"url"`               // Repository URL (e.g., github.com/user/repo)
	Path    string   `yaml:"path,omitempty"`     // Directory within repo to fetch
	Version string   `yaml:"version,omitempty"`  // Semantic version tag (e.g., v1.2.0)
	Ref     string   `yaml:"ref,omitempty"`      // Git ref — branch or commit (fallback if no version)
	Include []string `yaml:"include,omitempty"`  // Glob patterns to include
	Exclude []string `yaml:"exclude,omitempty"`  // Glob patterns to exclude
}

// Manifest represents the .slyds.yaml file stored in a presentation directory.
type Manifest struct {
	Theme      string                  `yaml:"theme"`
	Title      string                  `yaml:"title"`
	Sources    map[string]SourceConfig `yaml:"sources,omitempty"`
	ModulesDir string                  `yaml:"modules_dir,omitempty"`
}

// DefaultModulesDir is the default directory name for vendored modules.
const DefaultModulesDir = ".slyds-modules"

// ResolveModulesDir returns the modules directory path, using the default if not set.
func (m *Manifest) ResolveModulesDir(root string) string {
	dir := m.ModulesDir
	if dir == "" {
		dir = DefaultModulesDir
	}
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(root, dir)
}

// HasSources returns true if the manifest declares any external sources.
func (m *Manifest) HasSources() bool {
	return len(m.Sources) > 0
}

// ManifestPath returns the path to .slyds.yaml in the given directory.
func ManifestPath(dir string) string {
	return filepath.Join(dir, ".slyds.yaml")
}

// LockPath returns the path to .slyds.lock in the given directory.
func LockPath(dir string) string {
	return filepath.Join(dir, ".slyds.lock")
}

// ReadManifest reads and parses .slyds.yaml from dir.
// Returns ErrManifestNotFound if the file does not exist.
func ReadManifest(dir string) (*Manifest, error) {
	data, err := os.ReadFile(ManifestPath(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrManifestNotFound
		}
		return nil, fmt.Errorf("failed to read .slyds.yaml: %w", err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse .slyds.yaml: %w", err)
	}
	return &m, nil
}

// WriteManifest writes a .slyds.yaml file to the given directory.
func WriteManifest(dir string, m Manifest) error {
	data, err := yaml.Marshal(&m)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	return os.WriteFile(ManifestPath(dir), data, 0644)
}
