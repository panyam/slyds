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

// Manifest represents the .slyds.yaml file stored in a presentation directory.
type Manifest struct {
	Theme string `yaml:"theme"`
	Title string `yaml:"title"`
}

// ManifestPath returns the path to .slyds.yaml in the given directory.
func ManifestPath(dir string) string {
	return filepath.Join(dir, ".slyds.yaml")
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
