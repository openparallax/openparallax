package platform

// The default denylist is curated and ships in the binary. The user
// cannot extend or override it. If a user wants the agent to access
// something on this list, they relocate the data to a path that is
// not on the list — moving the file is the explicit consent action.
//
// Two levels:
//
//   - Restricted: no read, no write. The path's content is sensitive
//     and reading it is the attack (private keys, credentials).
//   - Protected:  read OK, write/delete blocked. Reading is useful and
//     safe, modifying is a persistence vector or destabilises the host
//     (shell rc files, /etc/hosts, ~/.gitconfig).
//
// Both levels apply to any path the agent touches, anywhere on disk.
// The platform package exposes accessors that return the resolved
// absolute path lists for the current OS. Engine code consumes them
// without making runtime platform decisions of its own.

// RestrictedPrefixes returns absolute path prefixes whose entire
// subtree is fully blocked from agent reads and writes. Per-platform
// implementations are in denylist_{linux,darwin,windows}.go.
func RestrictedPrefixes() []string { return restrictedPrefixes() }

// RestrictedFiles returns absolute file paths fully blocked from agent
// reads and writes. Per-platform implementations are in
// denylist_{linux,darwin,windows}.go.
func RestrictedFiles() []string { return restrictedFiles() }

// ProtectedPrefixes returns absolute path prefixes whose entire
// subtree is readable but not writable by the agent. Per-platform
// implementations are in denylist_{linux,darwin,windows}.go.
func ProtectedPrefixes() []string { return protectedPrefixes() }

// ProtectedFiles returns exact absolute paths the agent can read but
// not write. Per-platform implementations are in
// denylist_{linux,darwin,windows}.go.
func ProtectedFiles() []string { return protectedFiles() }

// RestrictedBasenameSuffixes returns filename suffixes that mark a
// file as Restricted regardless of where it lives on disk. Matched
// case-insensitively against the basename. Cross-platform.
func RestrictedBasenameSuffixes() []string {
	return []string{
		".pem",
		".key",
		".p12",
		".pfx",
		".keystore",
		".jks",
		".asc", // PGP armored keys
	}
}

// RestrictedBasenameExact returns exact basenames (case-insensitive)
// that mark a file as Restricted regardless of location. Cross-platform.
func RestrictedBasenameExact() []string {
	return []string{
		"id_rsa",
		"id_dsa",
		"id_ecdsa",
		"id_ed25519",
		".env",
		".env.local",
		".env.production",
		"credentials",
		"credentials.json",
		"secrets.yaml",
		"secrets.yml",
		"secrets.json",
		"token.json",
		"service-account.json",
		".pgpass",
		".my.cnf",
	}
}
