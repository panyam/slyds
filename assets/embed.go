package assets

import (
	"embed"
	"io/fs"
	"sort"
	"strings"
)

//go:embed slyds.css
var SlydsCSS string

//go:embed slyds.js
var SlydsJS string

//go:embed slyds-export.js
var SlydsExportJS string

//go:embed all:templates
var TemplatesFS embed.FS

//go:embed all:themes
var themesFS embed.FS

// ThemesCSS returns the concatenated CSS for all theme variable definitions.
// It loads _base.css first (the contract defaults), then all named theme
// override files in alphabetical order.
func ThemesCSS() string {
	var buf strings.Builder

	// _base.css first — defines the full variable contract
	if base, err := fs.ReadFile(themesFS, "themes/_base.css"); err == nil {
		buf.Write(base)
		buf.WriteByte('\n')
	}

	// Read all other .css files in sorted order
	entries, err := fs.ReadDir(themesFS, "themes")
	if err != nil {
		return buf.String()
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".css") && e.Name() != "_base.css" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		if data, err := fs.ReadFile(themesFS, "themes/"+name); err == nil {
			buf.WriteByte('\n')
			buf.Write(data)
			buf.WriteByte('\n')
		}
	}

	return buf.String()
}
