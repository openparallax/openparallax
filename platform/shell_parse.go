package platform

import (
	"path/filepath"
	"strings"
)

// FirstToken returns the first whitespace-separated token of cmd.
// Whitespace is space or tab. Returns the entire string when there
// is no whitespace. Returns the empty string for an empty input.
func FirstToken(cmd string) string {
	for i, ch := range cmd {
		if ch == ' ' || ch == '\t' {
			return cmd[:i]
		}
	}
	return cmd
}

// StripLeadingCD detects an optional `cd <target> && <command>` prefix
// at the start of cmd and returns the cd target and the rest of the
// command. Returns ok=false when:
//
//   - the command does not start with `cd ` or `cd\t`,
//   - the cd target is empty,
//   - the cd target is not followed by `&&`,
//   - quoted targets are not properly terminated.
//
// The target is returned exactly as written (no expansion, no
// resolution). Quoted targets (single or double) have their quotes
// stripped. Callers are responsible for any further validation such
// as requiring the target to be absolute.
//
// Only the simplest single-cd form is recognized. Chained cds, env-var
// targets, command substitution, and globs all return ok=false and the
// caller treats the command as having no cd prefix.
func StripLeadingCD(cmd string) (target, rest string, ok bool) {
	trimmed := strings.TrimLeft(cmd, " \t")
	if !strings.HasPrefix(trimmed, "cd ") && !strings.HasPrefix(trimmed, "cd\t") {
		return "", "", false
	}
	trimmed = strings.TrimSpace(trimmed[2:])
	if trimmed == "" {
		return "", "", false
	}

	var remainder string
	switch trimmed[0] {
	case '"':
		end := strings.Index(trimmed[1:], `"`)
		if end < 0 {
			return "", "", false
		}
		target = trimmed[1 : 1+end]
		remainder = trimmed[2+end:]
	case '\'':
		end := strings.Index(trimmed[1:], `'`)
		if end < 0 {
			return "", "", false
		}
		target = trimmed[1 : 1+end]
		remainder = trimmed[2+end:]
	default:
		end := strings.IndexAny(trimmed, " \t")
		if end < 0 {
			return "", "", false
		}
		target = trimmed[:end]
		remainder = trimmed[end:]
	}

	if target == "" {
		return "", "", false
	}

	remainder = strings.TrimLeft(remainder, " \t")
	if !strings.HasPrefix(remainder, "&&") {
		return "", "", false
	}
	rest = strings.TrimLeft(remainder[2:], " \t")
	return target, rest, true
}

// IsAbsolutePathSpec returns true when the raw path string from a tool
// payload is an absolute path that the engine can evaluate without an
// implicit working directory. A path starting with "~" counts as
// absolute because NormalizePath expands it to the user's home
// directory. Empty input is rejected.
func IsAbsolutePathSpec(raw string) bool {
	if raw == "" {
		return false
	}
	if strings.HasPrefix(raw, "~") {
		return true
	}
	return filepath.IsAbs(raw)
}

// PathHasPrefix reports whether path equals prefix or lives inside
// prefix as a subdirectory. Both arguments are cleaned with
// filepath.Clean before comparison so trailing separators do not
// affect the result. Uses the platform separator.
func PathHasPrefix(path, prefix string) bool {
	path = filepath.Clean(path)
	prefix = filepath.Clean(prefix)
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+string(filepath.Separator))
}
