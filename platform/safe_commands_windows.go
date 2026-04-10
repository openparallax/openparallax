//go:build windows

package platform

import "strings"

// windowsSafeCommands is the cmd.exe allowlist of known-safe shell
// command first-tokens. Same external dev tools as Unix (they install
// as external programs on Windows too) plus the safe cmd.exe builtins
// that don't take arbitrary path arguments.
var windowsSafeCommands = map[string]bool{
	// cmd.exe state queries.
	"echo":       true,
	"date":       true,
	"time":       true,
	"ver":        true,
	"vol":        true,
	"hostname":   true,
	"whoami":     true,
	"tasklist":   true,
	"ipconfig":   true,
	"systeminfo": true,
	"where":      true,

	// VCS.
	"git": true,
	"hg":  true,

	// JavaScript.
	"node": true,
	"npm":  true,
	"npx":  true,
	"pnpm": true,
	"yarn": true,
	"bun":  true,
	"deno": true,

	// Python.
	"python":  true,
	"python3": true,
	"pip":     true,
	"pip3":    true,
	"poetry":  true,

	// Rust.
	"cargo":  true,
	"rustc":  true,
	"rustup": true,

	// Go.
	"go":    true,
	"gofmt": true,

	// JVM.
	"java":   true,
	"javac":  true,
	"mvn":    true,
	"gradle": true,

	// Build systems.
	"make":  true,
	"cmake": true,
	"ninja": true,

	// Containers.
	"docker":         true,
	"docker-compose": true,
	"kubectl":        true,
	"helm":           true,
}

func safeCommands() map[string]bool { return windowsSafeCommands }

// windowsExeSuffixes are the executable file extensions cmd.exe will
// resolve when launching a command. Stripped during normalization so
// "git.exe", "git.EXE", and bare "git" all match the same allowlist
// entry.
var windowsExeSuffixes = []string{".exe", ".cmd", ".bat", ".com"}

// normalizeCommandName on Windows lowercases and strips the executable
// suffix so cmd.exe and CMD.EXE both match the bare "cmd" entry in the
// allowlist.
func normalizeCommandName(name string) string {
	name = strings.ToLower(name)
	for _, suffix := range windowsExeSuffixes {
		if strings.HasSuffix(name, suffix) {
			return name[:len(name)-len(suffix)]
		}
	}
	return name
}
