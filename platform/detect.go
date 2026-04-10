// Package platform provides OS detection, platform-specific path handling,
// shell configuration, and security-relevant path lists.
package platform

import "runtime"

// Platform identifies the operating system.
type Platform string

const (
	// PlatformLinux is the Linux operating system.
	PlatformLinux Platform = "linux"
	// PlatformMacOS is the macOS operating system.
	PlatformMacOS Platform = "darwin"
	// PlatformWindows is the Windows operating system.
	PlatformWindows Platform = "windows"
)

// Current returns the platform for the running OS.
func Current() Platform {
	switch runtime.GOOS {
	case "windows":
		return PlatformWindows
	case "darwin":
		return PlatformMacOS
	default:
		return PlatformLinux
	}
}

// IsWindows returns true if the current platform is Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}
