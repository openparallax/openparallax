package shield

import (
	"strings"

	"github.com/openparallax/openparallax/platform"
)

// safeCommandTable is the platform-appropriate set of known-safe shell
// command first-tokens, snapshotted at package init via the platform
// accessor. Engine code never makes runtime platform decisions.
var safeCommandTable = platform.SafeCommands()

// IsSafeCommand reports whether the given shell command qualifies for
// the Tier 0 fast-path allowlist. Curated, conservative, ships in the
// binary, not user-extensible.
//
// A command qualifies when:
//
//  1. After stripping an optional `cd <abs-path> && ` prefix (parsed
//     by platform.StripLeadingCD), the command is a single statement —
//     no shell metacharacters that compose multiple commands or
//     capture output: ; & | > < ` $(.
//
//  2. The first whitespace-separated token, normalized via
//     platform.NormalizeCommandName, matches an entry in the
//     platform-appropriate safe command table.
//
// The allowlist deliberately excludes commands that read or write
// arbitrary paths (cat, head, tail, grep, find, ls on Unix; type, dir,
// findstr on Windows). Those go through Shield's normal pipeline so
// the heuristic and Tier 2 layers can evaluate the actual targets.
//
// IsSafeCommand returns false for any command containing shell
// metacharacters that could compose other commands. A user who needs
// command chaining sends each command as a separate proposal.
func IsSafeCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}

	if target, rest, ok := platform.StripLeadingCD(cmd); ok {
		// The cd target must be absolute. A relative cd target
		// disqualifies the command from the fast path; Shield then
		// evaluates the original command normally.
		if !platform.IsAbsolutePathSpec(target) {
			return false
		}
		cmd = strings.TrimSpace(rest)
		if cmd == "" {
			return false
		}
	}

	if containsShellMeta(cmd) {
		return false
	}

	first := platform.FirstToken(cmd)
	if first == "" {
		return false
	}
	return safeCommandTable[platform.NormalizeCommandName(first)]
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
