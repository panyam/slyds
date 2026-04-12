package core

import (
	"errors"
	"fmt"

	"github.com/panyam/templar"
	"gopkg.in/yaml.v3"
)

// ErrManifestNotFound is returned when .slyds.yaml does not exist.
var ErrManifestNotFound = errors.New("manifest not found")

// SourceConfig represents an external template/theme dependency.
type SourceConfig struct {
	URL     string   `yaml:"url"`
	Path    string   `yaml:"path,omitempty"`
	Version string   `yaml:"version,omitempty"`
	Ref     string   `yaml:"ref,omitempty"`
	Include []string `yaml:"include,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`
}

// SlideRecord maps a stable slide_id to the slide's current filename.
// The id is assigned by slyds at slide creation time and survives every
// subsequent mutation: inserts, removes, moves, renumbers, even
// slugify-style renames. Only the File field changes as the underlying
// file is renamed on disk. Agents can reference a slide by id for
// rename-safe access across tool calls.
//
// SlideRecord is the per-slide part of the Manifest schema introduced
// in issue #83.
type SlideRecord struct {
	ID   string `yaml:"id"`
	File string `yaml:"file"`
}

// Manifest represents the .slyds.yaml file stored in a presentation directory.
type Manifest struct {
	Theme           string                  `yaml:"theme"`
	Title           string                  `yaml:"title"`
	Sources         map[string]SourceConfig `yaml:"sources,omitempty"`
	ModulesDir      string                  `yaml:"modules_dir,omitempty"`
	AgentIncludeMCP *bool                   `yaml:"agent_include_mcp,omitempty"`

	// Slides is the list of slide_id → filename records. Empty for
	// legacy decks that predate #83 — they get auto-migrated on the
	// next mutation. Read-only operations tolerate the empty state.
	Slides []SlideRecord `yaml:"slides,omitempty"`
}

// DefaultModulesDir is the default directory name for vendored modules.
const DefaultModulesDir = ".slyds-modules"

// DefaultCoreURL is the default GitHub URL for the slyds-core engine package.
const DefaultCoreURL = "github.com/panyam/slyds"

// DefaultCorePath is the subdirectory within the core URL that contains engine assets.
const DefaultCorePath = "assets"

// ResolvedModulesDir returns the modules directory name (relative path).
func (m *Manifest) ResolvedModulesDir() string {
	if m.ModulesDir != "" {
		return m.ModulesDir
	}
	return DefaultModulesDir
}

// HasSources returns true if the manifest declares any external sources.
func (m *Manifest) HasSources() bool {
	return len(m.Sources) > 0
}

// IncludeMCPInAgentDocs returns whether AGENT.md should include the MCP section.
func (m *Manifest) IncludeMCPInAgentDocs() bool {
	if m.AgentIncludeMCP == nil {
		return true
	}
	return *m.AgentIncludeMCP
}

// ReadManifestFS reads and parses .slyds.yaml from a WritableFS.
func ReadManifestFS(fsys templar.WritableFS) (*Manifest, error) {
	data, err := fsys.ReadFile(".slyds.yaml")
	if err != nil {
		return nil, ErrManifestNotFound
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse .slyds.yaml: %w", err)
	}
	return &m, nil
}

// WriteManifestFS writes .slyds.yaml to a WritableFS.
func WriteManifestFS(fsys templar.WritableFS, m Manifest) error {
	data, err := yaml.Marshal(&m)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	return fsys.WriteFile(".slyds.yaml", data, 0644)
}
