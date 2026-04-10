package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteReadRemovePID(t *testing.T) {
	workspace := t.TempDir()
	dotDir := filepath.Join(workspace, ".openparallax")
	require.NoError(t, os.MkdirAll(dotDir, 0o755))

	// Write.
	require.NoError(t, WritePID(workspace, 12345))

	// Read.
	pid, err := ReadPID(workspace)
	require.NoError(t, err)
	assert.Equal(t, 12345, pid)

	// Remove.
	require.NoError(t, RemovePID(workspace))
	_, err = ReadPID(workspace)
	assert.Error(t, err)
}

func TestIsRunningCurrentProcess(t *testing.T) {
	workspace := t.TempDir()
	dotDir := filepath.Join(workspace, ".openparallax")
	require.NoError(t, os.MkdirAll(dotDir, 0o755))

	// Write our own PID — should be alive.
	require.NoError(t, WritePID(workspace, os.Getpid()))
	assert.True(t, IsRunning(workspace))
}

func TestIsRunningStalePID(t *testing.T) {
	workspace := t.TempDir()
	dotDir := filepath.Join(workspace, ".openparallax")
	require.NoError(t, os.MkdirAll(dotDir, 0o755))

	// Write a bogus PID that is almost certainly not running.
	require.NoError(t, WritePID(workspace, 2147483647))
	assert.False(t, IsRunning(workspace))

	// Stale PID file should be cleaned up.
	_, err := ReadPID(workspace)
	assert.Error(t, err)
}

func TestIsRunningNoPIDFile(t *testing.T) {
	workspace := t.TempDir()
	assert.False(t, IsRunning(workspace))
}

func TestWriteReadRemoveGRPCPort(t *testing.T) {
	workspace := t.TempDir()

	// Missing → (0, false).
	port, ok := ReadGRPCPort(workspace)
	assert.False(t, ok)
	assert.Equal(t, 0, port)

	// Write.
	require.NoError(t, WriteGRPCPort(workspace, 47384))

	// Read.
	port, ok = ReadGRPCPort(workspace)
	require.True(t, ok)
	assert.Equal(t, 47384, port)

	// Remove.
	require.NoError(t, RemoveGRPCPort(workspace))
	_, ok = ReadGRPCPort(workspace)
	assert.False(t, ok)
}

func TestReadGRPCPortInvalidContents(t *testing.T) {
	workspace := t.TempDir()
	dotDir := filepath.Join(workspace, ".openparallax")
	require.NoError(t, os.MkdirAll(dotDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dotDir, "engine.port"), []byte("not-a-number"), 0o644))

	port, ok := ReadGRPCPort(workspace)
	assert.False(t, ok)
	assert.Equal(t, 0, port)
}
