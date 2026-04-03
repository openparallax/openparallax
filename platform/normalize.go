package platform

import (
	"os"
	"path/filepath"
	"strings"
)

// NormalizePath converts a path to a canonical form for consistent matching.
// It expands ~ to the home directory, converts backslashes to forward slashes,
// resolves . and .. components, and lowercases the drive letter on Windows.
func NormalizePath(p string) string {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			p = filepath.Join(home, p[1:])
		}
	}

	p = filepath.Clean(p)
	p = filepath.ToSlash(p)

	// Lowercase the drive letter on Windows for consistent matching.
	if len(p) >= 2 && p[1] == ':' {
		p = strings.ToLower(p[:1]) + p[1:]
	}

	return p
}

// IsWithinDirectory checks whether child is inside (or equal to) parent.
// Both paths are resolved to absolute form before comparison. Returns false
// if either path cannot be resolved, preventing path traversal attacks.
func IsWithinDirectory(child, parent string) bool {
	childAbs, err := filepath.Abs(child)
	if err != nil {
		return false
	}
	parentAbs, err := filepath.Abs(parent)
	if err != nil {
		return false
	}

	childAbs = filepath.Clean(childAbs)
	parentAbs = filepath.Clean(parentAbs)

	if childAbs == parentAbs {
		return true
	}

	// Ensure the parent path has a trailing separator so prefix matching
	// doesn't conflate /home/user with /home/username.
	parentPrefix := parentAbs + string(filepath.Separator)
	return strings.HasPrefix(childAbs+string(filepath.Separator), parentPrefix)
}

// KillProcessTree kills a process and all its children.
// On Unix, it sends SIGKILL to the process group.
// On Windows, it uses taskkill /T /F.
func KillProcessTree(pid int) error {
	return killProcessTreePlatform(pid)
}
