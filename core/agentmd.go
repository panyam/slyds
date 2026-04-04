package core

import (
	"strings"
	"text/template"
)

// agentLayoutEntry holds layout data for the AGENT.md template.
type agentLayoutEntry struct {
	Name        string
	Description string
	Slots       string
}

// agentMDData holds all data passed to the agent.md.tmpl template.
type agentMDData struct {
	Title      string
	Theme      string
	ThemeList  string
	Layouts    []agentLayoutEntry
	IncludeMCP bool
}

// renderAgentMD generates the AGENT.md content string from a manifest.
// Pure function — no I/O. Used by both WriteAgentMD and writeAgentMDFS.
func renderAgentMD(manifest Manifest) (string, error) {
	themes, _ := ListThemes()
	layoutNames, _ := ListLayouts()
	reg, _ := LoadRegistry()

	var layouts []agentLayoutEntry
	for _, name := range layoutNames {
		entry := agentLayoutEntry{Name: name}
		if reg != nil {
			if regEntry, ok := reg.Layouts[name]; ok {
				entry.Description = regEntry.Description
				entry.Slots = strings.Join(regEntry.Slots, ", ")
			}
		}
		layouts = append(layouts, entry)
	}

	data := agentMDData{
		Title:      manifest.Title,
		Theme:      manifest.Theme,
		ThemeList:  strings.Join(themes, ", "),
		Layouts:    layouts,
		IncludeMCP: manifest.IncludeMCPInAgentDocs(),
	}

	content, err := TemplatesFS.ReadFile("templates/agent.md.tmpl")
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("agent.md").Parse(string(content))
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
