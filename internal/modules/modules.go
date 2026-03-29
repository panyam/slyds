// Package modules bridges slyds's .slyds.yaml manifest with templar's
// module system (VendorConfig, SourceLoader, FetchSource, lock files).
//
// slyds constructs templar.VendorConfig programmatically from .slyds.yaml
// rather than using templar.yaml. This keeps slyds as the config surface
// owner while leveraging templar's fetch/vendor/lock machinery.
package modules

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/panyam/slyds/internal/scaffold"
	"github.com/panyam/templar"
)

// ToVendorConfig converts a slyds Manifest's sources into a templar VendorConfig.
// The returned config uses slyds-specific directory names (.slyds-modules/).
func ToVendorConfig(manifest *scaffold.Manifest, root string) *templar.VendorConfig {
	modulesDir := manifest.ResolveModulesDir(root)

	sources := make(map[string]templar.SourceConfig, len(manifest.Sources))
	for name, src := range manifest.Sources {
		sources[name] = templar.SourceConfig{
			URL:     src.URL,
			Path:    src.Path,
			Version: src.Version,
			Ref:     src.Ref,
			Include: src.Include,
			Exclude: src.Exclude,
		}
	}

	return &templar.VendorConfig{
		Sources:   sources,
		VendorDir: modulesDir,
		SearchPaths: []string{
			root,
			modulesDir,
		},
	}
}

// FetchAll fetches all sources declared in the manifest.
// It downloads dependencies into .slyds-modules/ and writes .slyds.lock.
func FetchAll(manifest *scaffold.Manifest, root string) error {
	if !manifest.HasSources() {
		return nil
	}

	config := ToVendorConfig(manifest, root)

	// Ensure modules directory exists
	if err := os.MkdirAll(config.VendorDir, 0755); err != nil {
		return fmt.Errorf("failed to create modules directory: %w", err)
	}

	// Fetch all sources
	results, err := templar.FetchAllSources(config)
	if err != nil {
		return fmt.Errorf("failed to fetch sources: %w", err)
	}

	// Build lock file from fetch results
	lock := &templar.VendorLock{
		Version: 1,
		Sources: make(map[string]templar.LockedSource),
	}
	for name, result := range results {
		lock.Sources[name] = templar.LockedSource{
			URL:            result.URL,
			Version:        result.Version,
			Ref:            result.Ref,
			ResolvedCommit: result.ResolvedCommit,
			FetchedAt:      result.FetchedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	// Write lock file
	lockPath := scaffold.LockPath(root)
	if err := templar.WriteLockFile(lockPath, lock); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	// Write vendor readme
	if err := templar.WriteVendorReadme(config.VendorDir); err != nil {
		// Non-fatal
	}

	return nil
}

// NewLoaderForDeck creates a templar LoaderList configured for a deck directory.
// If the deck has modules (.slyds-modules/), it uses a SourceLoader that can
// resolve @sourcename/path references. Otherwise, it falls back to a plain
// FileSystemLoader.
func NewLoaderForDeck(root string) templar.TemplateLoader {
	manifest, err := scaffold.ReadManifest(root)
	if err != nil || !manifest.HasSources() {
		// No manifest or no sources — use plain filesystem loader
		return (&templar.LoaderList{}).AddLoader(templar.NewFileSystemLoader(root))
	}

	modulesDir := manifest.ResolveModulesDir(root)
	if _, err := os.Stat(modulesDir); os.IsNotExist(err) {
		// Sources declared but not fetched — fall back to filesystem
		return (&templar.LoaderList{}).AddLoader(templar.NewFileSystemLoader(root))
	}

	// Build loader chain: SourceLoader (handles @source/ paths) + FileSystemLoader (local files)
	config := ToVendorConfig(manifest, root)
	sourceLoader := templar.NewSourceLoader(config)

	return (&templar.LoaderList{}).
		AddLoader(templar.NewFileSystemLoader(root)).
		AddLoader(sourceLoader)
}

// ModulesExist checks if the modules directory is populated.
func ModulesExist(root string) bool {
	manifest, err := scaffold.ReadManifest(root)
	if err != nil {
		return false
	}
	modulesDir := manifest.ResolveModulesDir(root)
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// SourcePath resolves a source-relative path to an absolute filesystem path.
// For example, SourcePath(root, "core", "themes/dark.css") returns the absolute
// path to dark.css within the vendored "core" source.
func SourcePath(root, sourceName, relativePath string) (string, error) {
	manifest, err := scaffold.ReadManifest(root)
	if err != nil {
		return "", fmt.Errorf("failed to read manifest: %w", err)
	}
	modulesDir := manifest.ResolveModulesDir(root)
	path := filepath.Join(modulesDir, sourceName, relativePath)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("source path not found: %s/%s: %w", sourceName, relativePath, err)
	}
	return path, nil
}
