package core

import (
	"fmt"
	"strings"
)

// NamingScheme controls how slide files are named on the underlying
// storage. The numbered scheme (default) produces NN-slug.html and
// renames files on every position shift; the slug-only scheme produces
// slug.html and skips renames entirely, relying on index.html and
// .slyds.yaml for ordering.
//
// The scheme is configured per-deck via the filename_style field in
// .slyds.yaml. A future WritableFS-level hint (templar upstream) can
// set the default for storage backends that prefer slug-only (e.g., S3
// where rename = copy+delete).
type NamingScheme interface {
	// Format returns the slide filename for a given 1-based position
	// and slug. The result includes the .html suffix.
	Format(position int, slug string) string

	// ShouldRenumber reports whether files need renaming when slide
	// positions shift (insert, remove, move). True for numbered,
	// false for slug-only.
	ShouldRenumber() bool

	// ExtractSlug parses the slug from a filename produced by Format.
	ExtractSlug(filename string) string

	// Name returns the scheme name as stored in .slyds.yaml
	// ("numbered" or "slug-only").
	Name() string
}

// FilenameStyleNumbered is the manifest value for the numbered scheme.
const FilenameStyleNumbered = "numbered"

// FilenameStyleSlugOnly is the manifest value for the slug-only scheme.
const FilenameStyleSlugOnly = "slug-only"

// SchemeForStyle returns the NamingScheme for a manifest filename_style
// value. Empty string defaults to numbered for backward compatibility.
func SchemeForStyle(style string) NamingScheme {
	switch style {
	case FilenameStyleSlugOnly:
		return SlugOnlyScheme{}
	default:
		return NumberedScheme{}
	}
}

// --- Numbered scheme (default, current behavior) ---

// NumberedScheme produces filenames like 03-intro.html and renames files
// on every position shift. This is the traditional slyds behavior that
// gives human-friendly ls output at the cost of N renames per mutation.
type NumberedScheme struct{}

func (NumberedScheme) Format(pos int, slug string) string {
	return fmt.Sprintf("%02d-%s.html", pos, slug)
}

func (NumberedScheme) ShouldRenumber() bool { return true }

func (NumberedScheme) ExtractSlug(f string) string {
	return slideSlugFromFile(f)
}

func (NumberedScheme) Name() string { return FilenameStyleNumbered }

// --- Slug-only scheme (for hosted/S3 backends) ---

// SlugOnlyScheme produces filenames like intro.html and never renames
// files when positions shift. Ordering lives entirely in index.html
// and .slyds.yaml. This eliminates the N-rename cost on S3/Azure blob
// storage where rename = copy+delete.
type SlugOnlyScheme struct{}

func (SlugOnlyScheme) Format(_ int, slug string) string {
	return slug + ".html"
}

func (SlugOnlyScheme) ShouldRenumber() bool { return false }

func (SlugOnlyScheme) ExtractSlug(f string) string {
	return strings.TrimSuffix(f, ".html")
}

func (SlugOnlyScheme) Name() string { return FilenameStyleSlugOnly }
