package core

import (
	"os"
	"path/filepath"
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

// WriteAgentMD generates AGENT.md + CLAUDE.md symlink on the local filesystem.
// This is the path-based convenience function for CLI usage.
func WriteAgentMD(dir string, manifest Manifest) error {
	content, err := renderAgentMD(manifest)
	if err != nil {
		return err
	}

	agentPath := filepath.Join(dir, "AGENT.md")
	if err := os.WriteFile(agentPath, []byte(content), 0644); err != nil {
		return err
	}

	// Create CLAUDE.md symlink
	claudeLink := filepath.Join(dir, "CLAUDE.md")
	os.Remove(claudeLink)
	return os.Symlink("AGENT.md", claudeLink)
}
