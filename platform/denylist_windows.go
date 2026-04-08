//go:build windows

package platform

import (
	"os"
	"path/filepath"
)

func restrictedPrefixes() []string {
	prefixes := []string{
		`C:\Windows\System32\config`,
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
		)
	}
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		prefixes = append(prefixes,
			filepath.Join(appdata, "Microsoft", "Credentials"),
			filepath.Join(appdata, "Microsoft", "Protect"),
		)
	}
	if localappdata := os.Getenv("LOCALAPPDATA"); localappdata != "" {
		prefixes = append(prefixes,
			filepath.Join(localappdata, "Microsoft", "Credentials"),
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
	return []string{
		`C:\Windows\System32\drivers\etc\hosts`,
	}
}
