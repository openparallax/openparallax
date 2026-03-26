package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrentPlatform(t *testing.T) {
	p := Current()
	switch runtime.GOOS {
	case "linux":
		assert.Equal(t, PlatformLinux, p)
	case "darwin":
		assert.Equal(t, PlatformMacOS, p)
	case "windows":
		assert.Equal(t, PlatformWindows, p)
	}
}

func TestNormalizePathTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	result := NormalizePath("~/Documents")
	assert.Equal(t, filepath.ToSlash(filepath.Join(home, "Documents")), result)
}

func TestNormalizePathForwardSlash(t *testing.T) {
	// On Windows, filepath.ToSlash converts \ to /. On Unix, \ is a valid
	// filename character so it stays as-is. Test with a path that uses
	// filepath.Join to get platform-appropriate separators.
	result := NormalizePath(filepath.Join("some", "path", "here"))
	assert.Equal(t, "some/path/here", result)
}

func TestNormalizePathClean(t *testing.T) {
	result := NormalizePath("/tmp/./foo/../bar")
	assert.Equal(t, "/tmp/bar", result)
}

func TestIsWithinDirectoryTrue(t *testing.T) {
	assert.True(t, IsWithinDirectory("/home/user/workspace/file.txt", "/home/user/workspace"))
}

func TestIsWithinDirectoryEqual(t *testing.T) {
	assert.True(t, IsWithinDirectory("/home/user/workspace", "/home/user/workspace"))
}

func TestIsWithinDirectoryTraversal(t *testing.T) {
	assert.False(t, IsWithinDirectory("/home/user/workspace/../../../etc/passwd", "/home/user/workspace"))
}

func TestIsWithinDirectorySiblingPrefix(t *testing.T) {
	// /home/username should NOT match /home/user.
	assert.False(t, IsWithinDirectory("/home/username", "/home/user"))
}

func TestIsWithinDirectoryRelative(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "subdir", "file.txt")
	assert.True(t, IsWithinDirectory(child, dir))
}

func TestSensitivePathsNonEmpty(t *testing.T) {
	paths := SensitivePaths()
	assert.NotEmpty(t, paths, "sensitive paths should not be empty")
	assert.GreaterOrEqual(t, len(paths), 6, "should have at least 6 cross-platform sensitive paths")
}

func TestShellConfig(t *testing.T) {
	cmd, flag := ShellConfig()
	if runtime.GOOS == "windows" {
		assert.Equal(t, "cmd.exe", cmd)
		assert.Equal(t, "/c", flag)
	} else {
		assert.Equal(t, "/bin/sh", cmd)
		assert.Equal(t, "-c", flag)
	}
}

func TestShellInjectionRulesNonEmpty(t *testing.T) {
	rules := ShellInjectionRules()
	assert.NotEmpty(t, rules)

	// Should always include cross-platform rules.
	xpCount := 0
	for _, r := range rules {
		if len(r.ID) >= 2 && r.ID[:2] == "XP" {
			xpCount++
		}
	}
	assert.Equal(t, 18, xpCount, "should have exactly 18 cross-platform rules")
}

func TestShellInjectionRulesPlatformSpecific(t *testing.T) {
	rules := ShellInjectionRules()

	hasUX := false
	hasWIN := false
	for _, r := range rules {
		if len(r.ID) >= 2 && r.ID[:2] == "UX" {
			hasUX = true
		}
		if len(r.ID) >= 3 && r.ID[:3] == "WIN" {
			hasWIN = true
		}
	}

	if runtime.GOOS == "windows" {
		assert.True(t, hasWIN, "Windows should have WIN rules")
		assert.False(t, hasUX, "Windows should not have UX rules")
	} else {
		assert.True(t, hasUX, "Unix should have UX rules")
		assert.False(t, hasWIN, "Unix should not have WIN rules")
	}
}
