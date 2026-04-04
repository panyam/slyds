// Package core bridges slyds's .slyds.yaml manifest with templar's
// module system via WritableFS. All I/O goes through the FS.
package core

import (
	"fmt"

	"github.com/panyam/templar"
)

// SlydsToolInfo provides slyds-specific branding for templar-generated content.
var SlydsToolInfo = templar.ToolInfo{
	Name:        "slyds",
	ConfigNames: []string{".slyds.yaml"},
	VendorDir:   "./.slyds-modules",
	LockFile:    ".slyds.lock",
	FetchCmd:    "slyds update",
	ProjectURL:  "https://github.com/panyam/slyds",
}

// ToVendorConfig converts a slyds Manifest's sources into a templar VendorConfig.
// The FS is used for all template resolution and vendored module access.
func ToVendorConfig(fsys templar.WritableFS, manifest *Manifest) *templar.VendorConfig {
	modulesDir := manifest.ResolvedModulesDir()

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
		FS:        fsys,
		SearchPaths: []string{
			".",
			modulesDir,
		},
	}
}

// FetchAll fetches all sources declared in the manifest via WritableFS.
// Downloads dependencies into .slyds-modules/ and writes .slyds.lock.
func FetchAll(fsys templar.WritableFS, manifest *Manifest) error {
	if !manifest.HasSources() {
		return nil
	}

	config := ToVendorConfig(fsys, manifest)

	// Ensure modules directory exists
	fsys.MkdirAll(config.VendorDir, 0755)

	// Fetch all sources via FS
	results, err := templar.FetchAllSourcesFS(fsys, config)
	if err != nil {
		return fmt.Errorf("failed to fetch sources: %w", err)
	}

	// Build lock file
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

	// Write lock file via FS
	if err := templar.WriteLockFileFS(fsys, ".slyds.lock", lock, SlydsToolInfo); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	// Write vendor readme via FS
	templar.WriteVendorReadmeFS(fsys, config.VendorDir, SlydsToolInfo)

	return nil
}

// NewLoaderForDeck creates a templar LoaderList configured for a deck's FS.
func NewLoaderForDeck(fsys templar.WritableFS) templar.TemplateLoader {
	manifest, err := ReadManifestFS(fsys)
	if err != nil || !manifest.HasSources() {
		return (&templar.LoaderList{}).AddLoader(
			templar.NewFileSystemLoader(templar.FSFolder{FS: fsys, Path: "."}))
	}

	config := ToVendorConfig(fsys, manifest)
	sourceLoader := templar.NewSourceLoader(config)

	return (&templar.LoaderList{}).
		AddLoader(templar.NewFileSystemLoader(templar.FSFolder{FS: fsys, Path: "."})).
		AddLoader(sourceLoader)
}

// ModulesExist checks if the modules directory is populated via FS.
func ModulesExist(fsys templar.WritableFS) bool {
	manifest, err := ReadManifestFS(fsys)
	if err != nil {
		return false
	}
	entries, err := fsys.ReadDir(manifest.ResolvedModulesDir())
	if err != nil {
		return false
	}
	return len(entries) > 0
}
