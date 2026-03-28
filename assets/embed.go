package assets

import "embed"

//go:embed slyds.css
var SlydsCSS string

//go:embed slyds.js
var SlydsJS string

//go:embed slyds-export.js
var SlydsExportJS string

//go:embed all:templates
var TemplatesFS embed.FS
