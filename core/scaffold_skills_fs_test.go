package core

import (
	"testing"

	"github.com/panyam/templar"
)

// TestWriteSkillsFS verifies that writeSkillsFS copies all embedded skill
// definitions into .claude/skills/ on a WritableFS. Each skill should have
// a SKILL.md file with non-empty content. This ensures `slyds init`
// scaffolds Claude Code skills that agents discover automatically.
func TestWriteSkillsFS(t *testing.T) {
	fsys := templar.NewMemFS()
	if err := writeSkillsFS(fsys); err != nil {
		t.Fatalf("writeSkillsFS: %v", err)
	}

	expected := []string{"slyds-preview", "slyds-add-slide", "slyds-check", "slyds-build", "slyds-slides"}
	for _, name := range expected {
		path := ".claude/skills/" + name + "/SKILL.md"
		data, err := fsys.ReadFile(path)
		if err != nil {
			t.Errorf("missing skill %s: %v", name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("skill %s SKILL.md is empty", name)
		}
	}
}

// TestScaffoldDeckIncludesSkills verifies that ScaffoldDeck writes skills
// alongside AGENT.md, slides, and other scaffold artifacts. This is the
// end-to-end test for skill scaffolding via `slyds init`.
func TestScaffoldDeckIncludesSkills(t *testing.T) {
	fsys := templar.NewMemFS()
	_, err := ScaffoldDeck(fsys, ScaffoldOpts{
		Title:      "Skills Test",
		SlideCount: 3,
		ThemeName:  "default",
	})
	if err != nil {
		t.Fatalf("ScaffoldDeck: %v", err)
	}

	// AGENT.md should exist
	if _, err := fsys.ReadFile("AGENT.md"); err != nil {
		t.Error("missing AGENT.md")
	}

	// Skills should exist
	for _, name := range []string{"slyds-preview", "slyds-check", "slyds-slides"} {
		path := ".claude/skills/" + name + "/SKILL.md"
		if _, err := fsys.ReadFile(path); err != nil {
			t.Errorf("ScaffoldDeck missing skill %s: %v", name, err)
		}
	}
}
