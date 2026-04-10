//go:build linux

package platform

import (
	"os"
	"path/filepath"
)

func restrictedPrefixes() []string {
	prefixes := []string{
		"/root",
		"/etc/sudoers.d",
		"/etc/ssh",
		"/var/lib/sss",
		"/var/lib/ldap",
	}
	if home, err := os.UserHomeDir(); err == nil {
		prefixes = append(prefixes,
			filepath.Join(home, ".ssh"),
			filepath.Join(home, ".aws"),
			filepath.Join(home, ".gnupg"),
			filepath.Join(home, ".docker"),
			filepath.Join(home, ".kube"),
			filepath.Join(home, ".password-store"),
			filepath.Join(home, ".azure"),
			filepath.Join(home, ".config", "gcloud"),
			filepath.Join(home, ".config", "op"),
			// Cross-agent isolation: block all OpenParallax workspaces.
			// The agent's own workspace is exempted in CheckProtection
			// so it can still read/write its own files normally.
			filepath.Join(home, ".openparallax"),
		)
	}
	return prefixes
}

func restrictedFiles() []string {
	files := []string{
		"/etc/shadow",
		"/etc/gshadow",
		"/etc/sudoers",
	}
	if home, err := os.UserHomeDir(); err == nil {
		files = append(files,
			filepath.Join(home, ".netrc"),
		)
	}
	return files
}

func protectedPrefixes() []string {
	return []string{
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
	}
}

func protectedFiles() []string {
	files := []string{
		"/etc/hosts",
		"/etc/passwd",
		"/etc/group",
		"/etc/fstab",
		"/etc/resolv.conf",
		"/etc/crontab",
		"/etc/environment",
	}
	if home, err := os.UserHomeDir(); err == nil {
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
	return files
}
