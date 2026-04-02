package sandbox

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyCanaryUnsandboxed(t *testing.T) {
	// Without a sandbox applied, probes should detect no enforcement.
	result := VerifyCanary()
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		// At least file_read and file_write probes run and should fail (not blocked).
		assert.False(t, result.Verified)
		assert.True(t, result.Failed() > 0, "expected at least one failed probe")
	}
	assert.Equal(t, runtime.GOOS, result.Platform)
	assert.NotEmpty(t, result.Mechanism)
	assert.NotEmpty(t, result.Summary)
	assert.False(t, result.Timestamp.IsZero())
}

func TestVerifyCanaryProbeResults(t *testing.T) {
	result := VerifyCanary()
	// Every probe should have a name and status.
	for _, p := range result.Probes {
		assert.NotEmpty(t, p.Name, "probe must have a name")
		assert.Contains(t, []string{"blocked", "failed", "skipped"}, p.Status,
			"probe %s has invalid status %q", p.Name, p.Status)
	}
}

func TestVerifyCanarySummary(t *testing.T) {
	result := VerifyCanary()
	assert.Contains(t, result.Summary, "probes blocked")
}

func TestCanaryResultHelpers(t *testing.T) {
	r := CanaryResult{
		Probes: []ProbeResult{
			{Name: "file_read", Status: "blocked"},
			{Name: "file_write", Status: "failed"},
			{Name: "network", Status: "skipped"},
		},
	}
	assert.Equal(t, 1, r.Blocked())
	assert.Equal(t, 1, r.Failed())
	assert.Equal(t, 1, r.Skipped())
}

func TestWriteAndReadCanaryResult(t *testing.T) {
	workspace := t.TempDir()

	result := CanaryResult{
		Verified:  true,
		Status:    "sandboxed",
		Platform:  "linux",
		Mechanism: "landlock",
		Probes: []ProbeResult{
			{Name: "file_read", Status: "blocked", Target: "/etc/shadow"},
			{Name: "file_write", Status: "blocked", Target: "/tmp"},
		},
		Summary: "Sandbox verified: 2/2 probes blocked (file_read, file_write).",
	}

	require.NoError(t, WriteCanaryResult(workspace, result))

	path := filepath.Join(workspace, ".openparallax", "sandbox.status")
	_, err := os.Stat(path)
	require.NoError(t, err)

	read := ReadCanaryResult(workspace)
	assert.Equal(t, "sandboxed", read.Status)
	assert.True(t, read.Verified)
	assert.Equal(t, "linux", read.Platform)
	assert.Equal(t, "landlock", read.Mechanism)
	assert.Len(t, read.Probes, 2)
	assert.Equal(t, 2, read.Blocked())
	assert.Equal(t, 0, read.Failed())
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

func TestCanaryStatusPartial(t *testing.T) {
	workspace := t.TempDir()
	partial := CanaryResult{
		Verified: false,
		Status:   "partial",
		Probes: []ProbeResult{
			{Name: "file_read", Status: "blocked"},
			{Name: "file_write", Status: "failed"},
		},
	}
	require.NoError(t, WriteCanaryResult(workspace, partial))
	read := ReadCanaryResult(workspace)
	assert.False(t, read.Verified)
	assert.Equal(t, "partial", read.Status)
	assert.Equal(t, 1, read.Blocked())
	assert.Equal(t, 1, read.Failed())
}
