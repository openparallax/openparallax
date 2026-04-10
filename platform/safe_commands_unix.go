//go:build !windows

package platform

// unixSafeCommands is the Unix (Linux/macOS, sh/bash) allowlist of
// known-safe shell command first-tokens. Curated, conservative.
// Commands here are safe regardless of arguments because they either
// take no arbitrary path arguments or operate predictably on the
// current working directory.
//
// Deliberately excludes commands that read or write arbitrary paths
// (cat, ls, head, tail, grep, find, rm, cp, mv): those go through
// Shield's normal pipeline so the heuristic and Tier 2 layers can
// evaluate the actual targets.
var unixSafeCommands = map[string]bool{
	// State queries — no arbitrary path arguments.
	"pwd":      true,
	"echo":     true,
	"printf":   true,
	"date":     true,
	"hostname": true,
	"whoami":   true,
	"id":       true,
	"uname":    true,
	"true":     true,
	"false":    true,
	"which":    true,
	"whereis":  true,
	"type":     true,
	"command":  true,

	// System state inspection.
	"df":       true,
	"du":       true,
	"free":     true,
	"ps":       true,
	"top":      true,
	"htop":     true,
	"uptime":   true,
	"lsof":     true,
	"netstat":  true,
	"ss":       true,
	"ip":       true,
	"ifconfig": true,
	"arp":      true,
	"route":    true,

	// VCS — operates on cwd.
	"git": true,
	"hg":  true,
	"svn": true,

	// JavaScript ecosystem.
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
	"pipx":    true,
	"poetry":  true,
	"uv":      true,

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
	"ant":    true,
	"kotlin": true,
	"scala":  true,

	// Other languages.
	"ruby":     true,
	"gem":      true,
	"bundle":   true,
	"php":      true,
	"composer": true,

	// Build systems.
	"make":  true,
	"cmake": true,
	"ninja": true,
	"bazel": true,
	"buck":  true,

	// Containers.
	"docker":         true,
	"docker-compose": true,
	"podman":         true,
	"buildah":        true,
	"kubectl":        true,
	"helm":           true,
	"k9s":            true,

	// Help and docs.
	"man":  true,
	"info": true,
	"help": true,

	// Version managers.
	"nvm":   true,
	"pyenv": true,
	"rbenv": true,
	"jenv":  true,
	"asdf":  true,
}

func safeCommands() map[string]bool { return unixSafeCommands }

// normalizeCommandName on Unix is identity. Command names are
// case-sensitive and do not have an executable suffix.
func normalizeCommandName(name string) string { return name }
