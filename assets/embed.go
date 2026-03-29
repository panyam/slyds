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

// ThemeFiles returns all theme CSS files as a map of filename → content.
// The map includes _base.css and all named theme override files.
func ThemeFiles() map[string]string {
	files := make(map[string]string)
	entries, err := fs.ReadDir(themesFS, "themes")
	if err != nil {
		return files
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".css") {
			if data, err := fs.ReadFile(themesFS, "themes/"+e.Name()); err == nil {
				files[e.Name()] = string(data)
			}
		}
	}
	return files
}

// ThemeFileNames returns theme CSS file names in load order:
// _base.css first, then all named themes alphabetically.
func ThemeFileNames() []string {
	entries, err := fs.ReadDir(themesFS, "themes")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".css") && e.Name() != "_base.css" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	// _base.css must load first
	return append([]string{"_base.css"}, names...)
}
