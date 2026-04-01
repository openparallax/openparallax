package sandbox

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanaryPathPerPlatform(t *testing.T) {
	path := canaryPath()
	switch runtime.GOOS {
	case "linux", "darwin":
		assert.Equal(t, "/etc/passwd", path)
	case "windows":
		assert.Contains(t, path, "SAM")
	}
}

func TestVerifyCanaryUnsandboxed(t *testing.T) {
	// Without a sandbox applied, the canary should succeed (open works),
	// meaning the result is "unsandboxed".
	result := VerifyCanary()
	// On a normal test environment, /etc/passwd is readable.
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		assert.Equal(t, "unsandboxed", result.Status)
		assert.False(t, result.Verified)
		assert.False(t, result.CanaryBlocked)
		assert.Equal(t, "/etc/passwd", result.CanaryPath)
		assert.Equal(t, runtime.GOOS, result.Platform)
		assert.NotEmpty(t, result.Mechanism)
	}
}

func TestVerifyCanaryFields(t *testing.T) {
	result := VerifyCanary()
	assert.NotEmpty(t, result.Status)
	assert.NotEmpty(t, result.CanaryPath)
	assert.NotEmpty(t, result.Platform)
	assert.NotEmpty(t, result.Mechanism)
	assert.False(t, result.Timestamp.IsZero())
}

func TestWriteAndReadCanaryResult(t *testing.T) {
	workspace := t.TempDir()

	result := CanaryResult{
		Verified:      true,
		Status:        "sandboxed",
		CanaryPath:    "/etc/passwd",
		CanaryBlocked: true,
		Platform:      "linux",
		Mechanism:     "landlock",
	}

	require.NoError(t, WriteCanaryResult(workspace, result))

	// Verify file exists.
	path := filepath.Join(workspace, ".openparallax", "sandbox.status")
	_, err := os.Stat(path)
	require.NoError(t, err)

	// Read back.
	read := ReadCanaryResult(workspace)
	assert.Equal(t, "sandboxed", read.Status)
	assert.True(t, read.Verified)
	assert.True(t, read.CanaryBlocked)
	assert.Equal(t, "/etc/passwd", read.CanaryPath)
	assert.Equal(t, "linux", read.Platform)
	assert.Equal(t, "landlock", read.Mechanism)
}

func TestReadCanaryResultMissing(t *testing.T) {
	workspace := t.TempDir()
	result := ReadCanaryResult(workspace)
	assert.Equal(t, "unknown", result.Status)
}

func TestReadCanaryResultCorrupt(t *testing.T) {
	workspace := t.TempDir()
	dir := filepath.Join(workspace, ".openparallax")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sandbox.status"), []byte("not json"), 0o644))

	result := ReadCanaryResult(workspace)
	assert.Equal(t, "unknown", result.Status)
}

func TestCanaryResultSandboxedVsUnsandboxed(t *testing.T) {
	// Verify result structure via write+read round-trip.
	workspace := t.TempDir()

	sandboxed := CanaryResult{
		Verified:      true,
		Status:        "sandboxed",
		CanaryBlocked: true,
	}
	require.NoError(t, WriteCanaryResult(workspace, sandboxed))
	read := ReadCanaryResult(workspace)
	assert.True(t, read.Verified)
	assert.True(t, read.CanaryBlocked)
	assert.Equal(t, "sandboxed", read.Status)

	unsandboxed := CanaryResult{
		Verified:      false,
		Status:        "unsandboxed",
		CanaryBlocked: false,
	}
	require.NoError(t, WriteCanaryResult(workspace, unsandboxed))
	read = ReadCanaryResult(workspace)
	assert.False(t, read.Verified)
	assert.False(t, read.CanaryBlocked)
	assert.Equal(t, "unsandboxed", read.Status)
}

func TestCanaryResultInconclusive(t *testing.T) {
	workspace := t.TempDir()
	inconclusive := CanaryResult{
		Verified: false,
		Status:   "inconclusive",
		Error:    "file not found",
	}
	require.NoError(t, WriteCanaryResult(workspace, inconclusive))
	read := ReadCanaryResult(workspace)
	assert.False(t, read.Verified)
	assert.Equal(t, "inconclusive", read.Status)
	assert.NotEmpty(t, read.Error)
}
