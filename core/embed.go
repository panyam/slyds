package core

import (
	"github.com/panyam/slyds/assets"
)

// Re-export from assets package for backward compatibility within core/.
var (
	SlydsCSS      = assets.SlydsCSS
	SlydsJS       = assets.SlydsJS
	SlydsExportJS = assets.SlydsExportJS
	TemplatesFS   = assets.TemplatesFS
	LayoutsFS     = assets.LayoutsFS
	themesFS      = assets.ThemesFS
)

// ThemeFiles returns all theme CSS files as a map of filename → content.
func ThemeFiles() map[string]string {
	return assets.ThemeFiles()
}

// ThemeFileNames returns theme CSS file names in load order.
func ThemeFileNames() []string {
	return assets.ThemeFileNames()
}
