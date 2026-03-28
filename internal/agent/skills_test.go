package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSkill(t *testing.T) {
	content := `---
name: test-skill
description: A test skill
actions:
  - read_file
  - write_file
keywords:
  - test
emoji: "\U0001F4DD"
---

# Test Skill

This is the body of the test skill.`

	skill, err := parseSkill(content)
	require.NoError(t, err)
	assert.Equal(t, "test-skill", skill.Name)
	assert.Equal(t, "A test skill", skill.Description)
	assert.Equal(t, []string{"read_file", "write_file"}, skill.Actions)
	assert.Contains(t, skill.Body, "This is the body")
}

func TestParseSkill_NoFrontmatter(t *testing.T) {
	_, err := parseSkill("No frontmatter here")
	assert.Error(t, err)
}

func TestSkillMatchesTool(t *testing.T) {
	skill := Skill{Actions: []string{"read_file", "write_file"}}
	assert.True(t, skill.MatchesTool("read_file"))
	assert.False(t, skill.MatchesTool("execute_command"))
}

func TestSkillManager_LoadFromDir(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	content := `---
name: custom
description: Custom skill
actions:
  - execute_command
---
# Custom Skill Body`
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "custom.md"), []byte(content), 0o644))

	sm := NewSkillManager(dir)
	skills := sm.Skills()
	found := false
	for _, s := range skills {
		if s.Name == "custom" {
			found = true
			assert.Equal(t, "Custom skill", s.Description)
		}
	}
	assert.True(t, found, "custom skill should be loaded")
}

func TestSkillManager_LightSummary(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "test.md"), []byte(`---
name: test
description: Test skill
actions: [read_file]
---
Body`), 0o644))

	sm := NewSkillManager(dir)
	summary := sm.LightSummary()
	assert.Contains(t, summary, "test")
	assert.Contains(t, summary, "Test skill")
}

func TestSkillManager_MatchSkills(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "git.md"), []byte(`---
name: git
description: Git ops
actions: [git_status, git_diff]
---
Git body`), 0o644))

	sm := NewSkillManager(dir)

	matched := sm.MatchSkills([]string{"git_status"})
	require.Len(t, matched, 1)
	assert.Equal(t, "git", matched[0].Name)

	// Second match for same skill — should not re-activate.
	matched2 := sm.MatchSkills([]string{"git_diff"})
	assert.Empty(t, matched2, "already active skill should not be re-matched")
}

func TestSkillManager_ActiveBodies(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "test.md"), []byte(`---
name: test
description: Test
actions: [read_file]
---
Detailed instructions here`), 0o644))

	sm := NewSkillManager(dir)
	assert.Empty(t, sm.ActiveSkillBodies())

	sm.MatchSkills([]string{"read_file"})
	bodies := sm.ActiveSkillBodies()
	assert.Contains(t, bodies, "Detailed instructions here")
}

func TestSkillManager_ResetSession(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "test.md"), []byte(`---
name: test
description: Test
actions: [read_file]
---
Body`), 0o644))

	sm := NewSkillManager(dir)
	sm.MatchSkills([]string{"read_file"})
	sm.ResetSession()

	// After reset, the same skill should match again.
	matched := sm.MatchSkills([]string{"read_file"})
	assert.Len(t, matched, 1)
}
