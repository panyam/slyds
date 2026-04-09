package core

import (
	"io/fs"

	"github.com/panyam/templar"
)

// writeSkillsFS copies embedded skill definitions into .claude/skills/ for
// Claude Code discovery. Skills are static SKILL.md files read from the
// embedded assets/skills/ directory — no template rendering needed.
//
// Each subdirectory in skills/ becomes a skill:
//   assets/skills/preview/SKILL.md → .claude/skills/preview/SKILL.md
func writeSkillsFS(fsys templar.WritableFS) error {
	entries, err := fs.ReadDir(SkillsFS, "skills")
	if err != nil {
		return nil // skills dir missing from embed — skip silently
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillName := entry.Name()
		content, err := fs.ReadFile(SkillsFS, "skills/"+skillName+"/SKILL.md")
		if err != nil {
			continue
		}
		outDir := ".claude/skills/" + skillName
		fsys.MkdirAll(outDir, 0755)
		fsys.WriteFile(outDir+"/SKILL.md", content, 0644)
	}
	return nil
}
