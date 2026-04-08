//go:build darwin

package platform

import (
	"os"
	"path/filepath"
)

func restrictedPrefixes() []string {
	prefixes := []string{}
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
			filepath.Join(home, "Library", "Keychains"),
			filepath.Join(home, "Library", "Cookies"),
			filepath.Join(home, "Library", "Application Support", "Google", "Chrome"),
			filepath.Join(home, "Library", "Application Support", "Firefox"),
			filepath.Join(home, "Library", "Safari"),
		)
	}
	return prefixes
}

func restrictedFiles() []string {
	files := []string{}
	if home, err := os.UserHomeDir(); err == nil {
		files = append(files,
			filepath.Join(home, ".netrc"),
		)
	}
	return files
}

func protectedPrefixes() []string {
	return []string{}
}

func protectedFiles() []string {
	files := []string{
		"/etc/hosts",
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
