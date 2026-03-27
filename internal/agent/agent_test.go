package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextAssemblerLoadsMemoryFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("Be helpful."), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "IDENTITY.md"), []byte("Name: Atlas"), 0o644))

	assembler := NewContextAssembler(dir)
	prompt, err := assembler.Assemble()
	require.NoError(t, err)

	assert.Contains(t, prompt, "Be helpful.")
	assert.Contains(t, prompt, "Name: Atlas")
	assert.Contains(t, prompt, "Core Values")
	assert.Contains(t, prompt, "Identity")
}

func TestContextAssemblerSkipsMissingFiles(t *testing.T) {
	dir := t.TempDir()
	// No files created — all should be silently skipped.

	assembler := NewContextAssembler(dir)
	prompt, err := assembler.Assemble()
	require.NoError(t, err)

	// Even with no memory files, behavioral rules are present.
	assert.Contains(t, prompt, "Behavioral Rules")
	assert.NotContains(t, prompt, "Core Values")
}

func TestContextAssemblerSkipsEmptyFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("   \n\n  "), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "IDENTITY.md"), []byte("Real content"), 0o644))

	assembler := NewContextAssembler(dir)
	prompt, err := assembler.Assemble()
	require.NoError(t, err)

	assert.NotContains(t, prompt, "Core Values")
	assert.Contains(t, prompt, "Real content")
}

func TestContextAssemblerContainsSOULContent(t *testing.T) {
	dir := t.TempDir()
	soulContent := "Safety first. Never take an irreversible action without approval."
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte(soulContent), 0o644))

	assembler := NewContextAssembler(dir)
	prompt, err := assembler.Assemble()
	require.NoError(t, err)

	assert.Contains(t, prompt, soulContent)
}
