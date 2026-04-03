package platform

import (
	"os"
	"path/filepath"
)

// SensitivePaths returns platform-specific paths that should trigger elevated
// Shield evaluation. These are files and directories containing credentials,
// system configuration, or other security-critical data.
func SensitivePaths() []string {
	home, _ := os.UserHomeDir()

	// Cross-platform sensitive paths
	paths := []string{
		filepath.Join(home, ".ssh"),
		filepath.Join(home, ".aws"),
		filepath.Join(home, ".kube"),
		filepath.Join(home, ".docker"),
		filepath.Join(home, ".gnupg"),
		filepath.Join(home, ".env"),
	}

	switch Current() {
	case PlatformWindows:
		// Windows stores credentials in APPDATA and system config in System32.
		appdata := os.Getenv("APPDATA")
		localappdata := os.Getenv("LOCALAPPDATA")
		paths = append(paths,
			filepath.Join(appdata, "Microsoft", "Credentials"),
			filepath.Join(appdata, "Microsoft", "Protect"),
			filepath.Join(localappdata, "Microsoft", "Credentials"),
			filepath.Join(home, ".rdp"),
			filepath.Join(home, ".ppk"),
			`C:\Windows\System32\config\SAM`,
			`C:\Windows\System32\config\SYSTEM`,
			`C:\Windows\System32\config\SECURITY`,
		)
	case PlatformMacOS:
		// macOS stores credentials in Keychain and browser data in Library.
		paths = append(paths,
			filepath.Join(home, "Library", "Keychains"),
			filepath.Join(home, "Library", "Cookies"),
			filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "Default", "Login Data"),
		)
	case PlatformLinux:
		// Linux stores system credentials in /etc and root home.
		paths = append(paths,
			"/etc/shadow",
			"/etc/passwd",
			"/etc/sudoers",
			"/etc/ssh",
			"/root",
		)
	}

	return paths
}
