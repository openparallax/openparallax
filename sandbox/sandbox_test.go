package sandbox

import (
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	sb := New()
	require.NotNil(t, sb)
}

func TestMode(t *testing.T) {
	sb := New()
	mode := sb.Mode()

	switch runtime.GOOS {
	case "linux":
		assert.Equal(t, "landlock", mode)
	case "darwin":
		assert.Equal(t, "sandbox-exec", mode)
	case "windows":
		assert.Equal(t, "job-object", mode)
	default:
		assert.Equal(t, "none", mode)
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := Config{
		AllowedReadPaths:  []string{"/usr/lib"},
		AllowedWritePaths: nil,
		AllowedTCPConnect: []string{"localhost:50051"},
		AllowProcessSpawn: false,
	}

	assert.Len(t, cfg.AllowedReadPaths, 1)
	assert.Empty(t, cfg.AllowedWritePaths)
	assert.Len(t, cfg.AllowedTCPConnect, 1)
	assert.False(t, cfg.AllowProcessSpawn)
}

func TestProbe(t *testing.T) {
	status := Probe()

	// Status must always have a mode set.
	assert.NotEmpty(t, status.Mode)

	switch runtime.GOOS {
	case "linux":
		// Landlock may or may not be available depending on kernel.
		assert.Equal(t, "landlock", status.Mode)
		if status.Active {
			assert.True(t, status.Filesystem)
			assert.Greater(t, status.Version, 0)
		}
	case "darwin":
		assert.Equal(t, "sandbox-exec", status.Mode)
	case "windows":
		assert.Equal(t, "job-object", status.Mode)
		assert.True(t, status.Active)
	default:
		assert.False(t, status.Active)
		assert.Equal(t, "none", status.Mode)
	}
}

func TestStatusFields(t *testing.T) {
	s := Status{
		Active:     true,
		Mode:       "landlock",
		Version:    6,
		Filesystem: true,
		Network:    true,
	}

	assert.True(t, s.Active)
	assert.Equal(t, "landlock", s.Mode)
	assert.Equal(t, 6, s.Version)
	assert.True(t, s.Filesystem)
	assert.True(t, s.Network)
	assert.Empty(t, s.Reason)
}

func TestStatusInactive(t *testing.T) {
	s := Status{
		Active: false,
		Mode:   "none",
		Reason: "unsupported platform",
	}

	assert.False(t, s.Active)
	assert.Equal(t, "none", s.Mode)
	assert.Equal(t, "unsupported platform", s.Reason)
}

func TestWrapCommandNoOP(t *testing.T) {
	sb := New()
	cmd := exec.Command("echo", "test")

	// On Linux, WrapCommand is a no-op (agent self-sandboxes).
	// On other platforms, it may modify the command.
	err := sb.WrapCommand(cmd, Config{})
	assert.NoError(t, err)
}

func TestAvailableLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	sb := New()
	// On modern kernels (5.13+), this should be true.
	// We don't assert true because CI may run older kernels.
	_ = sb.Available()
}

func TestAvailableDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-only test")
	}

	sb := New()
	// sandbox-exec exists on all current macOS.
	_, err := os.Stat("/usr/bin/sandbox-exec")
	if err == nil {
		assert.True(t, sb.Available())
	}
}

func TestAvailableWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	sb := New()
	// Job Objects are always available on Windows.
	assert.True(t, sb.Available())
}
