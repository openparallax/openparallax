package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextAssemblerLoadsMemoryFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("Be helpful."), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "IDENTITY.md"), []byte("Name: Atlas"), 0o644))

	assembler := NewContextAssembler(dir)
	prompt, err := assembler.Assemble(types.SessionNormal)
	require.NoError(t, err)

	assert.Contains(t, prompt, "Be helpful.")
	assert.Contains(t, prompt, "Name: Atlas")
	assert.Contains(t, prompt, "Core Guardrails")
	assert.Contains(t, prompt, "Your Identity")
}

func TestContextAssemblerSkipsMissingFiles(t *testing.T) {
	dir := t.TempDir()

	assembler := NewContextAssembler(dir)
	prompt, err := assembler.Assemble(types.SessionNormal)
	require.NoError(t, err)

	// Behavioral rules are always present even with no memory files.
	assert.Contains(t, prompt, "Behavioral Rules")
	assert.NotContains(t, prompt, "Your Identity")
}

func TestContextAssemblerSkipsEmptyFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("   \n\n  "), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "IDENTITY.md"), []byte("Real content"), 0o644))

	assembler := NewContextAssembler(dir)
	prompt, err := assembler.Assemble(types.SessionNormal)
	require.NoError(t, err)

	assert.NotContains(t, prompt, "Core Guardrails")
	assert.Contains(t, prompt, "Real content")
}

func TestContextAssemblerContainsSOULContent(t *testing.T) {
	dir := t.TempDir()
	soulContent := "Safety first. Never take an irreversible action without approval."
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte(soulContent), 0o644))

	assembler := NewContextAssembler(dir)
	prompt, err := assembler.Assemble(types.SessionNormal)
	require.NoError(t, err)

	assert.Contains(t, prompt, soulContent)
}

func TestContextAssemblerOTRNotice(t *testing.T) {
	dir := t.TempDir()
	assembler := NewContextAssembler(dir)

	normalPrompt, _ := assembler.Assemble(types.SessionNormal)
	otrPrompt, _ := assembler.Assemble(types.SessionOTR)

	assert.NotContains(t, normalPrompt, "Off the Record")
	assert.Contains(t, otrPrompt, "Off the Record")
	assert.Contains(t, otrPrompt, "READ-ONLY")
}

func TestContextAssemblerSecretRules(t *testing.T) {
	dir := t.TempDir()
	assembler := NewContextAssembler(dir)

	prompt, _ := assembler.Assemble(types.SessionNormal)
	assert.Contains(t, prompt, "Sensitive Data")
}
