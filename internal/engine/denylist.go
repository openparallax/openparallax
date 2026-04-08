package engine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// The default denylist is curated and ships in the binary. The user
// cannot extend or override it. If a user wants the agent to access
// something on this list, they relocate the data to a path that is
// not on the list — moving the file is the explicit consent action.
//
// Two levels:
//
//   - Restricted: no read, no write. The path's content is sensitive
//     and reading it IS the attack (private keys, credentials).
//   - Protected:  read OK, write/delete blocked. Reading is useful and
//     safe, modifying is a persistence vector or destabilises the host
//     (shell rc files, /etc/hosts, ~/.gitconfig).
//
// Both levels apply to ANY path the agent touches, anywhere on disk —
// not just paths inside the workspace. The check runs before any
// workspace-relative protection rules in CheckProtection.
//
// The denylist is read symlink-resolved paths. EvalSymlinks runs
// before defaultProtection so a symlink trick like ~/safe.txt →
// ~/.ssh/id_rsa cannot bypass it.

// restrictedPrefixes are absolute path prefixes whose entire subtree
// is fully blocked from agent reads and writes.
func restrictedPrefixes() []string {
	home := userHome()
	prefixes := []string{}

	// Cross-platform credential dirs (under user home).
	if home != "" {
		prefixes = append(prefixes,
			filepath.Join(home, ".ssh"),
			filepath.Join(home, ".aws"),
			filepath.Join(home, ".gnupg"),
			filepath.Join(home, ".docker"),
			filepath.Join(home, ".kube"),
			filepath.Join(home, ".password-store"),
			filepath.Join(home, ".azure"),
			filepath.Join(home, ".config", "gcloud"),
			filepath.Join(home, ".config", "op"), // 1Password CLI
		)
	}

	switch runtime.GOOS {
	case "linux":
		prefixes = append(prefixes,
			"/root",
			"/etc/sudoers.d",
			"/etc/ssh",
			"/var/lib/sss",
			"/var/lib/ldap",
		)
	case "darwin":
		if home != "" {
			prefixes = append(prefixes,
				filepath.Join(home, "Library", "Keychains"),
				filepath.Join(home, "Library", "Cookies"),
				filepath.Join(home, "Library", "Application Support", "Google", "Chrome"),
				filepath.Join(home, "Library", "Application Support", "Firefox"),
				filepath.Join(home, "Library", "Safari"),
			)
		}
	case "windows":
		appdata := os.Getenv("APPDATA")
		localappdata := os.Getenv("LOCALAPPDATA")
		if appdata != "" {
			prefixes = append(prefixes,
				filepath.Join(appdata, "Microsoft", "Credentials"),
				filepath.Join(appdata, "Microsoft", "Protect"),
			)
		}
		if localappdata != "" {
			prefixes = append(prefixes,
				filepath.Join(localappdata, "Microsoft", "Credentials"),
			)
		}
		prefixes = append(prefixes,
			`C:\Windows\System32\config`,
		)
	}

	return prefixes
}

// restrictedFiles are absolute file paths fully blocked from agent
// reads and writes.
func restrictedFiles() []string {
	home := userHome()
	files := []string{}

	if home != "" {
		files = append(files,
			filepath.Join(home, ".netrc"),
		)
	}

	if runtime.GOOS == "linux" {
		files = append(files,
			"/etc/shadow",
			"/etc/gshadow",
			"/etc/sudoers",
		)
	}

	return files
}

// restrictedBasenameSuffixes are filename patterns that mark a file as
// Restricted regardless of where it lives. Matched as case-insensitive
// suffix on the basename. Used to catch private keys and credential
// files the user may have placed anywhere.
var restrictedBasenameSuffixes = []string{
	".pem",
	".key",
	".p12",
	".pfx",
	".keystore",
	".jks",
	".asc", // PGP armored keys
}

// restrictedBasenameExact are exact basenames (case-insensitive) that
// mark a file as Restricted regardless of location.
var restrictedBasenameExact = map[string]bool{
	"id_rsa":               true,
	"id_dsa":               true,
	"id_ecdsa":             true,
	"id_ed25519":           true,
	"id_rsa.pub":           false, // public key, not sensitive
	".env":                 true,
	".env.local":           true,
	".env.production":      true,
	".npmrc":               false, // protected, not restricted (handled below)
	"credentials":          true,
	"credentials.json":     true,
	"secrets.yaml":         true,
	"secrets.yml":          true,
	"secrets.json":         true,
	"token.json":           true,
	"service-account.json": true,
	".pgpass":              true,
	".my.cnf":              true, // MySQL credentials
}

// protectedPrefixes are absolute path prefixes whose entire subtree is
// readable but not writable by the agent.
func protectedPrefixes() []string {
	prefixes := []string{}

	if runtime.GOOS == "linux" {
		prefixes = append(prefixes,
			"/etc/cron.d",
			"/etc/cron.daily",
			"/etc/cron.weekly",
			"/etc/cron.monthly",
			"/etc/cron.hourly",
			"/etc/systemd",
			"/etc/init.d",
			"/etc/apt",
			"/etc/yum.repos.d",
			"/etc/dnf",
			"/etc/pacman.d",
			"/etc/network",
			"/etc/NetworkManager",
		)
	}

	return prefixes
}

// protectedFilesAbsolute are exact absolute paths the agent can read
// but not write. Different from the workspace-relative protectedFiles
// map at the top of protection.go — those apply to files inside the
// agent's workspace, these apply anywhere on disk.
func protectedFilesAbsolute() []string {
	home := userHome()
	files := []string{}

	if home != "" {
		files = append(files,
			filepath.Join(home, ".bashrc"),
			filepath.Join(home, ".bash_profile"),
			filepath.Join(home, ".bash_logout"),
			filepath.Join(home, ".zshrc"),
			filepath.Join(home, ".zprofile"),
			filepath.Join(home, ".zlogin"),
			filepath.Join(home, ".profile"),
			filepath.Join(home, ".config", "fish", "config.fish"),
			filepath.Join(home, ".gitconfig"),
			filepath.Join(home, ".gitignore_global"),
			filepath.Join(home, ".npmrc"),
			filepath.Join(home, ".yarnrc"),
			filepath.Join(home, ".config", "pip", "pip.conf"),
			filepath.Join(home, ".config", "cargo", "config.toml"),
			filepath.Join(home, ".vimrc"),
			filepath.Join(home, ".config", "nvim", "init.lua"),
			filepath.Join(home, ".config", "nvim", "init.vim"),
			filepath.Join(home, ".tmux.conf"),
			filepath.Join(home, ".inputrc"),
		)
	}

	switch runtime.GOOS {
	case "linux":
		files = append(files,
			"/etc/hosts",
			"/etc/passwd",
			"/etc/group",
			"/etc/fstab",
			"/etc/resolv.conf",
			"/etc/crontab",
			"/etc/environment",
		)
	case "darwin":
		files = append(files,
			"/etc/hosts",
		)
	case "windows":
		files = append(files,
			`C:\Windows\System32\drivers\etc\hosts`,
		)
	}

	return files
}

// defaultProtection returns the protection level for a resolved
// absolute path against the cross-platform default denylist. Returns
// Unprotected when no rule matches; the caller's existing
// workspace-relative checks then take over.
//
// The check order is: Restricted prefixes → Restricted files →
// Restricted basename patterns → Protected prefixes → Protected files.
// First match wins. Restricted always beats Protected.
func defaultProtection(resolved string) ProtectionLevel {
	if resolved == "" {
		return Unprotected
	}
	normalized := filepath.Clean(resolved)

	for _, prefix := range restrictedPrefixes() {
		if pathHasPrefix(normalized, prefix) {
			return FullBlock
		}
	}
	for _, file := range restrictedFiles() {
		if normalized == filepath.Clean(file) {
			return FullBlock
		}
	}

	base := strings.ToLower(filepath.Base(normalized))
	if restrictedBasenameExact[base] {
		return FullBlock
	}
	for _, suffix := range restrictedBasenameSuffixes {
		if strings.HasSuffix(base, suffix) {
			return FullBlock
		}
	}

	for _, prefix := range protectedPrefixes() {
		if pathHasPrefix(normalized, prefix) {
			return ReadOnly
		}
	}
	for _, file := range protectedFilesAbsolute() {
		if normalized == filepath.Clean(file) {
			return ReadOnly
		}
	}

	return Unprotected
}

// pathHasPrefix returns true when path is equal to prefix or lives
// inside prefix as a subdirectory. Uses the platform separator.
func pathHasPrefix(path, prefix string) bool {
	prefix = filepath.Clean(prefix)
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+string(filepath.Separator))
}

// userHome returns the user's home directory or empty string when
// unavailable. Cached on first call would be possible but the call is
// cheap.
func userHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}
