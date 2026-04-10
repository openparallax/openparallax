package engine

import (
	"path/filepath"
	"strings"

	"github.com/openparallax/openparallax/platform"
)

// The default denylist is a curated, ship-with-the-binary set of paths
// the agent must never touch (Restricted) or must never modify
// (Protected). The data tables live in the platform package, behind
// build-tagged accessors per OS, so engine code stays free of any
// runtime platform decisions.
//
// Two levels:
//
//   - FullBlock:  no read, no write. Credential dirs and files where
//     the content itself is the secret.
//   - ReadOnly:   read OK, write/delete blocked. Shell rc files,
//     system reference files, package manager configs.
//
// Both apply to any path the agent touches, anywhere on disk. The
// check runs after symlink resolution and before workspace-relative
// rules in CheckProtection.
//
// The lists are computed once at package init via the platform
// accessors and stored in package-level vars; lookups during action
// evaluation do not allocate or call back into the platform package.

var (
	denylistRestrictedPrefixes = platform.RestrictedPrefixes()
	denylistRestrictedFiles    = cleanedSet(platform.RestrictedFiles())
	denylistProtectedPrefixes  = platform.ProtectedPrefixes()
	denylistProtectedFiles     = cleanedSet(platform.ProtectedFiles())

	denylistRestrictedBasenameSuffixes = platform.RestrictedBasenameSuffixes()
	denylistRestrictedBasenameExact    = stringSet(platform.RestrictedBasenameExact())
)

// cleanedSet returns a set of cleaned paths for O(1) exact-match lookup.
func cleanedSet(paths []string) map[string]bool {
	out := make(map[string]bool, len(paths))
	for _, p := range paths {
		out[filepath.Clean(p)] = true
	}
	return out
}

// stringSet returns a set keyed by lowercased string for O(1) lookup.
func stringSet(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, s := range items {
		out[strings.ToLower(s)] = true
	}
	return out
}

// defaultProtection returns the protection level for a resolved
// absolute path against the cross-platform default denylist. Returns
// Unprotected when no rule matches; the caller's existing
// workspace-relative checks then take over.
//
// Match order: restricted prefixes → restricted files → restricted
// basename patterns → protected prefixes → protected files. First
// match wins. Restricted always beats Protected because the
// restricted checks run first.
func defaultProtection(resolved string) ProtectionLevel {
	if resolved == "" {
		return Unprotected
	}
	normalized := filepath.Clean(resolved)

	for _, prefix := range denylistRestrictedPrefixes {
		if platform.PathHasPrefix(normalized, prefix) {
			return FullBlock
		}
	}
	if denylistRestrictedFiles[normalized] {
		return FullBlock
	}

	base := strings.ToLower(filepath.Base(normalized))
	if denylistRestrictedBasenameExact[base] {
		return FullBlock
	}
	for _, suffix := range denylistRestrictedBasenameSuffixes {
		if strings.HasSuffix(base, suffix) {
			return FullBlock
		}
	}

	for _, prefix := range denylistProtectedPrefixes {
		if platform.PathHasPrefix(normalized, prefix) {
			return ReadOnly
		}
	}
	if denylistProtectedFiles[normalized] {
		return ReadOnly
	}

	return Unprotected
}
