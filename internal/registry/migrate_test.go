package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateNoWorkspaces(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "agents.json")
	require.NoError(t, Migrate(regPath))

	reg, err := Load(regPath)
	require.NoError(t, err)
	assert.Empty(t, reg.Agents)
}

func TestMigrateSingleWorkspace(t *testing.T) {
	dir := t.TempDir()

	// Create a workspace with config.yaml.
	wsDir := filepath.Join(dir, "nova")
	require.NoError(t, os.MkdirAll(wsDir, 0o755))
	cfg := "identity:\n  name: Nova\nweb:\n  port: 3100\n"
	require.NoError(t, os.WriteFile(filepath.Join(wsDir, "config.yaml"), []byte(cfg), 0o644))

	regPath := filepath.Join(dir, "agents.json")
	require.NoError(t, Migrate(regPath))

	reg, err := Load(regPath)
	require.NoError(t, err)
	require.Len(t, reg.Agents, 1)
	assert.Equal(t, "Nova", reg.Agents[0].Name)
	assert.Equal(t, "nova", reg.Agents[0].Slug)
	assert.Equal(t, 3100, reg.Agents[0].WebPort)
	assert.Equal(t, 4100, reg.Agents[0].GRPCPort)
}

func TestMigrateMultipleWorkspaces(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"nova", "jarvis"} {
		wsDir := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(wsDir, 0o755))
		cfg := "identity:\n  name: " + name + "\n"
		require.NoError(t, os.WriteFile(filepath.Join(wsDir, "config.yaml"), []byte(cfg), 0o644))
	}

	regPath := filepath.Join(dir, "agents.json")
	require.NoError(t, Migrate(regPath))

	reg, err := Load(regPath)
	require.NoError(t, err)
	assert.Len(t, reg.Agents, 2)
}

func TestMigrateSentinelPreventsRerun(t *testing.T) {
	dir := t.TempDir()

	wsDir := filepath.Join(dir, "nova")
	require.NoError(t, os.MkdirAll(wsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wsDir, "config.yaml"), []byte("identity:\n  name: Nova\n"), 0o644))

	regPath := filepath.Join(dir, "agents.json")
	require.NoError(t, Migrate(regPath))

	reg, err := Load(regPath)
	require.NoError(t, err)
	require.Len(t, reg.Agents, 1)

	// Add another workspace.
	ws2 := filepath.Join(dir, "jarvis")
	require.NoError(t, os.MkdirAll(ws2, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ws2, "config.yaml"), []byte("identity:\n  name: Jarvis\n"), 0o644))

	// Migrate again — sentinel should prevent re-run.
	require.NoError(t, Migrate(regPath))
	reg2, err := Load(regPath)
	require.NoError(t, err)
	assert.Len(t, reg2.Agents, 1) // Still 1, not 2.
}

func TestMigrateSkipsWorkspaceDir(t *testing.T) {
	dir := t.TempDir()

	// "workspace" is a legacy default directory — should be skipped.
	wsDir := filepath.Join(dir, "workspace")
	require.NoError(t, os.MkdirAll(wsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wsDir, "config.yaml"), []byte("identity:\n  name: Atlas\n"), 0o644))

	regPath := filepath.Join(dir, "agents.json")
	require.NoError(t, Migrate(regPath))

	reg, err := Load(regPath)
	require.NoError(t, err)
	assert.Empty(t, reg.Agents)
}
