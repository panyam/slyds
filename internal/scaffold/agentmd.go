package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/slyds/internal/layout"
)

// WriteAgentMD generates an AGENT.md file in the deck directory containing
// documentation for LLM agents (Claude Code, Cursor, Copilot, etc.) about
// available commands, layouts, themes, and conventions.
// This file is regenerated on init and update.
func WriteAgentMD(dir string, manifest Manifest) error {
	themes, _ := ListThemes()
	layouts, _ := layout.ListLayouts()
	reg, _ := layout.LoadRegistry()

	var buf strings.Builder

	fmt.Fprintf(&buf, "# Presentation: %s\n\n", manifest.Title)
	buf.WriteString("This is a slyds presentation. Edit slides in the `slides/` directory.\n\n")

	buf.WriteString("## Quick Reference\n\n")
	buf.WriteString("```\n")
	buf.WriteString("slyds serve              # preview at localhost:3000\n")
	buf.WriteString("slyds build              # build dist/index.html (self-contained)\n")
	buf.WriteString("slyds ls                 # list slides with layouts\n")
	buf.WriteString("slyds describe           # structured deck summary (YAML)\n")
	buf.WriteString("slyds check              # validate deck\n")
	buf.WriteString("```\n\n")

	buf.WriteString("## Editing Slides\n\n")
	buf.WriteString("```\n")
	buf.WriteString("slyds add \"Name\" --layout content     # add a slide (append)\n")
	buf.WriteString("slyds add \"Name\" --after 3            # add after slide 3\n")
	buf.WriteString("slyds insert 2 \"Name\" --layout title  # insert at position\n")
	buf.WriteString("slyds rm 4                            # remove slide 4\n")
	buf.WriteString("slyds mv 2 5                          # move slide 2 to position 5\n")
	buf.WriteString("```\n\n")

	buf.WriteString("## Reading/Writing Slide Content\n\n")
	buf.WriteString("```\n")
	buf.WriteString("slyds query 3 h1 --text                                # read heading\n")
	buf.WriteString("slyds query 3 h1 --set \"New Title\"                     # set heading\n")
	buf.WriteString("slyds query 3 '[data-slot=\"left\"]' --set \"<p>…</p>\"    # set slot content\n")
	buf.WriteString("slyds query 3 '[data-slot=\"body\"]' --append \"<li>…</li>\"\n")
	buf.WriteString("```\n\n")

	buf.WriteString("## Available Layouts\n\n")
	buf.WriteString("| Layout | Use for | Slots |\n")
	buf.WriteString("|--------|---------|-------|\n")
	for _, name := range layouts {
		if reg != nil {
			if entry, ok := reg.Layouts[name]; ok {
				fmt.Fprintf(&buf, "| `%s` | %s | %s |\n", name, entry.Description, strings.Join(entry.Slots, ", "))
				continue
			}
		}
		fmt.Fprintf(&buf, "| `%s` | | |\n", name)
	}
	buf.WriteString("\n")

	buf.WriteString("## Available Themes\n\n")
	fmt.Fprintf(&buf, "%s (current: %s)\n\n", strings.Join(themes, ", "), manifest.Theme)
	buf.WriteString("Switch at runtime via the theme button in the toolbar, or set in `.slyds.yaml`.\n\n")

	buf.WriteString("## Conventions\n\n")
	buf.WriteString("- One HTML file per slide in `slides/`\n")
	buf.WriteString("- Slides are plain HTML — no template syntax\n")
	buf.WriteString("- `index.html` controls slide order (don't edit manually — use slyds commands)\n")
	buf.WriteString("- Use `slyds query` for content edits, not regex/string manipulation\n")
	buf.WriteString("- Speaker notes go in `<div class=\"speaker-notes\">` inside each slide\n")
	buf.WriteString("- Each slide has a `data-layout` attribute identifying its structural layout\n")
	buf.WriteString("- Use `[data-slot=\"name\"]` CSS selectors to target named content regions\n")

	agentPath := filepath.Join(dir, "AGENT.md")
	if err := os.WriteFile(agentPath, []byte(buf.String()), 0644); err != nil {
		return err
	}

	// Create CLAUDE.md symlink so Claude Code loads it as project instructions
	claudeLink := filepath.Join(dir, "CLAUDE.md")
	os.Remove(claudeLink) // remove stale symlink/file if exists
	return os.Symlink("AGENT.md", claudeLink)
}
