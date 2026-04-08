package shield

import (
	"path/filepath"
	"runtime"
	"strings"
)

// IsSafeCommand reports whether the given shell command qualifies for
// the Tier 0 fast-path allowlist. Curated, conservative, ships in the
// binary. Not user-extensible.
//
// A command qualifies when:
//
//  1. After stripping an optional `cd <abs-path> && ` prefix (and
//     verifying the cd target is absolute), the command is a single
//     statement — no shell metacharacters that compose multiple
//     commands or capture output: ; & | > < ` $(.
//
//  2. The first whitespace-separated token, lowercased and stripped of
//     directory and .exe suffix, matches an entry in the
//     platform-appropriate safe command table.
//
// The allowlist deliberately excludes commands that read or write
// arbitrary paths (cat, head, tail, grep, find, ls on Unix; type, dir,
// findstr on Windows). Those go through Shield's normal pipeline so
// the heuristic and Tier 2 layers can evaluate the actual targets.
//
// The allowlist is for commands whose safety is determined by the
// command itself: dev workflow commands that operate on the current
// working directory (git, npm, make, go) and state-query commands that
// take no arbitrary path arguments (pwd, whoami, date, hostname).
//
// IsSafeCommand returns false for any command containing shell
// metacharacters that could compose other commands. A user who needs
// command chaining sends each command as a separate proposal.
func IsSafeCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}

	// Strip an optional `cd <abs> && ` prefix. The cd target must be
	// absolute. The rest of the command is then evaluated normally.
	if rest, ok := stripLeadingAbsoluteCD(cmd); ok {
		cmd = strings.TrimSpace(rest)
		if cmd == "" {
			return false
		}
	}

	// Reject anything containing shell metacharacters that could
	// compose other commands or run additional code paths. These
	// disqualify the command from the fast path; Shield then
	// evaluates the original command normally.
	if containsShellMeta(cmd) {
		return false
	}

	first := firstToken(cmd)
	if first == "" {
		return false
	}

	return safeCommands()[normalizeCommand(first)]
}

// containsShellMeta returns true when the command contains any shell
// metacharacter that composes multiple commands or captures output.
// The fast path requires a single, simple statement.
func containsShellMeta(cmd string) bool {
	for _, ch := range cmd {
		switch ch {
		case ';', '|', '&', '>', '<', '`':
			return true
		}
	}
	return strings.Contains(cmd, "$(")
}

// firstToken returns the first whitespace-separated token of cmd.
func firstToken(cmd string) string {
	for i, ch := range cmd {
		if ch == ' ' || ch == '\t' {
			return cmd[:i]
		}
	}
	return cmd
}

// normalizeCommand strips directory prefix and (on Windows) .exe
// suffix from a command name, then lowercases on Windows.
func normalizeCommand(name string) string {
	name = filepath.Base(name)
	if runtime.GOOS == "windows" {
		name = strings.ToLower(name)
		name = strings.TrimSuffix(name, ".exe")
		name = strings.TrimSuffix(name, ".cmd")
		name = strings.TrimSuffix(name, ".bat")
	}
	return name
}

// stripLeadingAbsoluteCD detects `cd <absolute-path> && <command>` at
// the start of cmd and returns the command body. Returns ok=false when
// the command does not start with cd, when the cd target is not
// absolute, or when the cd is followed by anything other than `&&`.
func stripLeadingAbsoluteCD(cmd string) (rest string, ok bool) {
	trimmed := strings.TrimLeft(cmd, " \t")
	if !strings.HasPrefix(trimmed, "cd ") && !strings.HasPrefix(trimmed, "cd\t") {
		return "", false
	}
	trimmed = strings.TrimSpace(trimmed[2:])

	// Read the cd target. Support unquoted, single-quoted, double-quoted.
	var target, remainder string
	switch trimmed[0] {
	case '"':
		end := strings.Index(trimmed[1:], `"`)
		if end < 0 {
			return "", false
		}
		target = trimmed[1 : 1+end]
		remainder = trimmed[2+end:]
	case '\'':
		end := strings.Index(trimmed[1:], `'`)
		if end < 0 {
			return "", false
		}
		target = trimmed[1 : 1+end]
		remainder = trimmed[2+end:]
	default:
		end := strings.IndexAny(trimmed, " \t")
		if end < 0 {
			return "", false
		}
		target = trimmed[:end]
		remainder = trimmed[end:]
	}

	// Target must be absolute.
	if !filepath.IsAbs(target) && !strings.HasPrefix(target, "~") {
		return "", false
	}

	remainder = strings.TrimLeft(remainder, " \t")
	if !strings.HasPrefix(remainder, "&&") {
		return "", false
	}
	return strings.TrimLeft(remainder[2:], " \t"), true
}

// safeCommands returns the platform-appropriate allowlist.
func safeCommands() map[string]bool {
	if runtime.GOOS == "windows" {
		return windowsSafeCommands
	}
	return unixSafeCommands
}

// unixSafeCommands is the Unix (Linux/macOS, sh/bash) allowlist.
// Curated, conservative. Commands here are safe regardless of their
// arguments because they either take no arbitrary path arguments or
// they operate predictably on the current working directory.
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
	"type":     true, // bash builtin
	"command":  true, // bash builtin

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

	// Version managers (no arbitrary writes during normal use).
	"nvm":   true,
	"pyenv": true,
	"rbenv": true,
	"jenv":  true,
	"asdf":  true,
}

// windowsSafeCommands is the cmd.exe allowlist. Same dev tools as Unix
// (they install as external programs on Windows too) plus the safe
// cmd.exe builtins that don't take arbitrary path arguments.
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
