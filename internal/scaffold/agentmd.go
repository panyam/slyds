package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/panyam/slyds/assets"
	"github.com/panyam/slyds/internal/layout"
)

// agentLayoutEntry holds layout data for the AGENT.md template.
type agentLayoutEntry struct {
	Name        string
	Description string
	Slots       string
}

// agentMDData holds all data passed to the agent.md.tmpl template.
type agentMDData struct {
	Title     string
	Theme     string
	ThemeList string
	Layouts   []agentLayoutEntry
}

// WriteAgentMD generates an AGENT.md file in the deck directory containing
// documentation for LLM agents (Claude Code, Cursor, Copilot, etc.) about
// available commands, layouts, themes, and conventions.
// This file is regenerated on init and update.
func WriteAgentMD(dir string, manifest Manifest) error {
	themes, _ := ListThemes()
	layoutNames, _ := layout.ListLayouts()
	reg, _ := layout.LoadRegistry()

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
		Title:     manifest.Title,
		Theme:     manifest.Theme,
		ThemeList: strings.Join(themes, ", "),
		Layouts:   layouts,
	}

	// Load and render the template
	content, err := assets.TemplatesFS.ReadFile("templates/agent.md.tmpl")
	if err != nil {
		return err
	}

	tmpl, err := template.New("agent.md").Parse(string(content))
	if err != nil {
		return err
	}

	agentPath := filepath.Join(dir, "AGENT.md")
	f, err := os.Create(agentPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	// Create CLAUDE.md symlink so Claude Code loads it as project instructions
	claudeLink := filepath.Join(dir, "CLAUDE.md")
	os.Remove(claudeLink) // remove stale symlink/file if exists
	return os.Symlink("AGENT.md", claudeLink)
}
