package platform

import "path/filepath"

// SafeCommands returns the curated allowlist of shell commands whose
// first token marks them as safe to fast-path through Shield. The list
// is platform-specific (different shell builtins per OS) and ships in
// the binary. Per-platform implementations live in
// safe_commands_{unix,windows}.go.
func SafeCommands() map[string]bool { return safeCommands() }

// NormalizeCommandName strips directory prefix and any platform-specific
// executable suffix from a command name, then case-folds when the
// platform's command lookup is case-insensitive (Windows). Used to
// match a parsed command token against the SafeCommands table.
func NormalizeCommandName(name string) string {
	name = filepath.Base(name)
	return normalizeCommandName(name)
}

// normalizeCommandName is the per-platform tail of NormalizeCommandName.
// Implementations live in safe_commands_{unix,windows}.go.
