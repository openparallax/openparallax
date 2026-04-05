package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSkill(t *testing.T) {
	content := `---
name: deploy-guide
description: How we deploy to production using Terraform and AWS
---

# Deployment Guide

Always run terraform plan before apply.`

	skill, err := parseSkill(content)
	require.NoError(t, err)
	assert.Equal(t, "deploy-guide", skill.Name)
	assert.Equal(t, "How we deploy to production using Terraform and AWS", skill.Description)
	assert.Contains(t, skill.Body, "Always run terraform plan")
}

func TestParseSkillNoFrontmatter(t *testing.T) {
	_, err := parseSkill("No frontmatter here")
	assert.Error(t, err)
}

func TestParseSkillNoName(t *testing.T) {
	_, err := parseSkill("---\ndescription: missing name\n---\nbody")
	assert.Error(t, err)
}

func TestSkillManagerLoadFromDir(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "my-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	content := `---
name: my-skill
description: A custom skill for testing
---
# My Skill Body`
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644))

	sm := NewSkillManager(dir, nil)
	skills := sm.Skills()
	require.Len(t, skills, 1)
	assert.Equal(t, "my-skill", skills[0].Name)
	assert.Equal(t, "A custom skill for testing", skills[0].Description)
	assert.Contains(t, skills[0].Body, "My Skill Body")
	assert.Equal(t, skillDir, skills[0].Dir)
}

func TestSkillManagerIgnoresFlatFiles(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	// Flat .md file should be ignored — must be in subdirectory.
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "old-style.md"), []byte(`---
name: old
description: Old flat style
---
body`), 0o644))

	sm := NewSkillManager(dir, nil)
	assert.Empty(t, sm.Skills())
}

func TestSkillManagerDiscoverySummary(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "deploy")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: deploy
description: Production deployment procedures
---
Body`), 0o644))

	sm := NewSkillManager(dir, nil)
	summary := sm.DiscoverySummary()
	assert.Contains(t, summary, "deploy")
	assert.Contains(t, summary, "Production deployment procedures")
	assert.Contains(t, summary, "load_skills")
}

func TestSkillManagerDiscoverySummaryEmpty(t *testing.T) {
	dir := t.TempDir()
	sm := NewSkillManager(dir, nil)
	assert.Empty(t, sm.DiscoverySummary())
}

func TestSkillManagerLoadSkill(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "test")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: test
description: Test skill
---
Detailed instructions here`), 0o644))

	sm := NewSkillManager(dir, nil)

	// Before loading.
	assert.Empty(t, sm.LoadedSkillBodies())

	// Load by name.
	body, ok := sm.LoadSkill("test")
	assert.True(t, ok)
	assert.Contains(t, body, "Detailed instructions here")

	// Loaded bodies should now include it.
	assert.Contains(t, sm.LoadedSkillBodies(), "Detailed instructions here")

	// Loading unknown skill.
	_, ok = sm.LoadSkill("nonexistent")
	assert.False(t, ok)
}

func TestSkillManagerResetSession(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "test")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: test
description: Test
---
Body`), 0o644))

	sm := NewSkillManager(dir, nil)
	sm.LoadSkill("test")
	assert.NotEmpty(t, sm.LoadedSkillBodies())

	sm.ResetSession()
	assert.Empty(t, sm.LoadedSkillBodies())
}

func TestSkillManagerHasSkills(t *testing.T) {
	dir := t.TempDir()
	sm := NewSkillManager(dir, nil)
	assert.False(t, sm.HasSkills())

	skillDir := filepath.Join(dir, "skills", "one")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: one
description: One
---
body`), 0o644))

	sm2 := NewSkillManager(dir, nil)
	assert.True(t, sm2.HasSkills())
}

func TestSkillManagerDisabled(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		skillDir := filepath.Join(dir, "skills", name)
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(fmt.Sprintf(`---
name: %s
description: Skill %s
---
Body for %s`, name, name, name)), 0o644))
	}

	// Without disabled — both present.
	sm := NewSkillManager(dir, nil)
	assert.Len(t, sm.Skills(), 2)

	// With alpha disabled.
	sm2 := NewSkillManager(dir, []string{"alpha"})
	assert.Len(t, sm2.Skills(), 1)
	assert.Equal(t, "beta", sm2.Skills()[0].Name)

	// With both disabled.
	sm3 := NewSkillManager(dir, []string{"alpha", "beta"})
	assert.Empty(t, sm3.Skills())
}
