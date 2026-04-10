package engine

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/platform"
)

// ProtectionLevel defines how a file is protected from agent modification.
type ProtectionLevel int

const (
	// Unprotected means no hardcoded protection applies.
	Unprotected ProtectionLevel = iota
	// ReadOnly means the agent can read but all modifications are hard-blocked.
	ReadOnly
	// EscalateTier2 means writes are allowed but require Tier 2 LLM evaluation.
	EscalateTier2
	// WriteTier1Min means writes are allowed but require Tier 1 minimum.
	WriteTier1Min
	// FullBlock means the agent cannot read or write the file.
	FullBlock
)

// protectedFiles maps workspace-relative filenames to their protection level.
// This is HARDCODED. Not configurable. Not overridable by policy.
var protectedFiles = map[string]ProtectionLevel{
	// Core identity — read-only, never modifiable by the agent.
	"soul.md":     ReadOnly,
	"identity.md": ReadOnly,

	// Significant changes — LLM evaluation required.
	"agents.md":    EscalateTier2,
	"heartbeat.md": EscalateTier2,

	// Normal agent operations — heuristic check catches injection.
	"user.md":   WriteTier1Min,
	"memory.md": WriteTier1Min,
}

// hardBlockedFiles are files the agent cannot read OR write.
var hardBlockedFiles = map[string]bool{
	"config.yaml":     true,
	"canary.token":    true,
	"audit.jsonl":     true,
	"openparallax.db": true,
}

// hardBlockedDirs are directories the agent cannot read or write into.
var hardBlockedDirs = []string{
	".openparallax",
	"security",
}

// readOnlyDirs are directories the agent can read but not write into.
var readOnlyDirs = []string{
	"skills",
}

// CheckProtection is the FIRST check in processToolCall. It runs before OTR,
// before Shield, before audit, before everything. Returns whether the action
// is allowed, the protection level, and a reason if blocked.
func CheckProtection(action *types.ActionRequest, workspacePath string) (bool, ProtectionLevel, string) {
	// For shell commands, extract write targets and check them separately.
	if action.Type == types.ActionExecCommand {
		return checkShellProtection(action, workspacePath)
	}

	// For directory copy/move, check if any protected files would be overwritten.
	if action.Type == types.ActionCopyDir || action.Type == types.ActionMoveDir {
		if allowed, prot, reason := checkDirectoryOverwrite(action, workspacePath); !allowed {
			return false, prot, reason
		}
	}

	// Check all path fields in the action payload.
	allPaths := extractAllPaths(action)

	// Enforce absolute paths. Shield evaluates the literal path and cannot
	// resolve relative paths against an implicit working directory. The
	// agent must send absolute paths (with ~ expansion) for every payload
	// field. This makes Shield's path-based rules deterministic.
	for _, rawPath := range allPaths {
		if !platform.IsAbsolutePathSpec(rawPath) {
			return false, FullBlock, "path " + rawPath + " is relative — Shield requires absolute paths so it can evaluate the literal target. Resend with the full absolute path."
		}
	}

	isWrite := isWriteAction(action.Type)

	highestProtection := Unprotected

	for _, rawPath := range allPaths {
		resolved := resolveProtectionPath(rawPath, workspacePath)

		// Resolve symlinks to detect symlink bypass attacks.
		if realPath, err := filepath.EvalSymlinks(resolved); err == nil {
			resolved = realPath
		}

		// Default cross-platform denylist (applies anywhere on disk).
		// Restricted: no read, no write. Protected: read OK, write blocked.
		// Exemption: the agent's own workspace is allowed even though
		// ~/.openparallax is on the restricted list (cross-agent
		// isolation). Other agents' workspaces remain blocked.
		if !isInWorkspace(resolved, workspacePath) {
			switch defaultProtection(resolved) {
			case FullBlock:
				return false, FullBlock, "access to " + filepath.Base(resolved) + " is blocked — this path is on the default denylist (credentials, system secrets, or sensitive user data)"
			case ReadOnly:
				if isWrite {
					return false, ReadOnly, filepath.Base(resolved) + " is read-only on the default denylist — the agent can read it but not modify it. Edit it manually if you need to change it."
				}
			}
		}

		// Check hard-blocked files and directories.
		if isHardBlocked(resolved, workspacePath) {
			return false, FullBlock, "access to " + filepath.Base(resolved) + " is blocked — this is a system-critical file"
		}

		// Check read-only directories (write blocked, read allowed).
		if isWrite && isInReadOnlyDir(resolved, workspacePath) {
			return false, ReadOnly, "skills directory is read-only — create or edit skills manually"
		}

		// Check protected workspace files.
		basename := strings.ToLower(filepath.Base(resolved))
		if protection, ok := protectedFiles[basename]; ok {
			if isInWorkspace(resolved, workspacePath) && isWrite {
				switch protection {
				case ReadOnly:
					return false, ReadOnly, filepath.Base(resolved) + " is protected — it defines the agent's core identity and guardrails. Edit it manually if you need to change it."
				case EscalateTier2:
					if isDeleteAction(action.Type) {
						return false, ReadOnly, filepath.Base(resolved) + " cannot be deleted — it is an agent configuration file."
					}
					if protection > highestProtection {
						highestProtection = protection
					}
				case WriteTier1Min:
					if protection > highestProtection {
						highestProtection = protection
					}
				}
			}
		}
	}

	return true, highestProtection, ""
}

// isDeleteAction returns true for actions that remove files or directories.
func isDeleteAction(t types.ActionType) bool {
	switch t {
	case types.ActionDeleteFile, types.ActionDeleteDir:
		return true
	default:
		return false
	}
}

// checkShellProtection handles execute_command by extracting only write targets
// from the command string. Read operations (cat, grep, head) are allowed.
//
// Shell commands must use absolute paths so Shield can evaluate the literal
// targets. The one allowed exception is `cd <absolute-path> && <command>` —
// the cd prefix establishes an implicit working directory, and write targets
// in the rest of the command are resolved against it. Anything else with
// relative paths is rejected with a clear error so the agent can re-roll.
func checkShellProtection(action *types.ActionRequest, workspacePath string) (bool, ProtectionLevel, string) {
	cmd, _ := action.Payload["command"].(string)
	if cmd == "" {
		return true, Unprotected, ""
	}

	// Parse an optional `cd <abs-path> && ` prefix. If present, use the cd
	// target as the resolution base for write targets in the rest of the
	// command. Reject relative cd targets outright.
	cmdBody := cmd
	cdBase := ""
	if base, rest, ok := platform.StripLeadingCD(cmd); ok {
		if !platform.IsAbsolutePathSpec(base) {
			return false, FullBlock, "cd target " + base + " is relative — Shield requires `cd <absolute-path> && <command>` so it can resolve write targets. Resend with the full absolute path."
		}
		cdBase = platform.NormalizePath(base)
		cmdBody = rest
	}

	writeTargets := extractWriteTargetsFromCommand(cmdBody)
	highestProtection := Unprotected

	for _, target := range writeTargets {
		// Reject relative write targets when there's no cd prefix to anchor
		// them. With a cd prefix, the resolution base is the cd target.
		if !platform.IsAbsolutePathSpec(target) && cdBase == "" {
			return false, FullBlock, "shell command contains relative path " + target + " — Shield requires absolute paths or a leading `cd <absolute-path> && ` prefix. Resend with the full absolute path."
		}
		base := workspacePath
		if cdBase != "" {
			base = cdBase
		}
		resolved := resolveProtectionPath(target, base)
		if realPath, err := filepath.EvalSymlinks(resolved); err == nil {
			resolved = realPath
		}

		// Default cross-platform denylist applies to shell write
		// targets the same as it does to non-shell actions. Own
		// workspace is exempted (cross-agent isolation).
		if !isInWorkspace(resolved, workspacePath) {
			switch defaultProtection(resolved) {
			case FullBlock:
				return false, FullBlock, filepath.Base(resolved) + " is on the default denylist and cannot be modified via shell command"
			case ReadOnly:
				return false, ReadOnly, filepath.Base(resolved) + " is read-only on the default denylist and cannot be modified via shell command"
			}
		}

		if isHardBlocked(resolved, workspacePath) {
			return false, FullBlock, filepath.Base(resolved) + " is a system-critical file and cannot be modified via shell command"
		}

		if isInReadOnlyDir(resolved, workspacePath) {
			return false, ReadOnly, "skills directory is read-only — cannot modify via shell command"
		}

		basename := strings.ToLower(filepath.Base(resolved))
		if protection, ok := protectedFiles[basename]; ok {
			if isInWorkspace(resolved, workspacePath) {
				switch protection {
				case ReadOnly:
					return false, ReadOnly, filepath.Base(resolved) + " is protected and cannot be modified via shell command"
				case EscalateTier2:
					if shellCommandIsDelete(cmd) {
						return false, ReadOnly, filepath.Base(resolved) + " cannot be deleted via shell command"
					}
					if protection > highestProtection {
						highestProtection = protection
					}
				case WriteTier1Min:
					if protection > highestProtection {
						highestProtection = protection
					}
				}
			}
		}
	}

	return true, highestProtection, ""
}

// shellCommandIsDelete checks if a shell command is attempting to delete a specific target.
func shellCommandIsDelete(cmd string) bool {
	lower := strings.ToLower(cmd)
	return strings.Contains(lower, "rm ") || strings.Contains(lower, "del ") ||
		strings.Contains(lower, "erase ") || strings.Contains(lower, "remove-item ")
}

// checkDirectoryOverwrite checks if a directory copy/move would overwrite protected files.
func checkDirectoryOverwrite(action *types.ActionRequest, workspacePath string) (bool, ProtectionLevel, string) {
	dst, _ := action.Payload["destination"].(string)
	if dst == "" {
		return true, Unprotected, ""
	}
	dstResolved := resolveProtectionPath(dst, workspacePath)
	if !isInWorkspace(dstResolved, workspacePath) {
		return true, Unprotected, ""
	}

	src, _ := action.Payload["source"].(string)
	srcResolved := resolveProtectionPath(src, workspacePath)

	// Check if protected files exist at the destination that would be overwritten,
	// or if the source contains files that would overwrite protected targets.
	// We scan the source directory for any file whose lowercase name matches
	// a protected filename, because os.Stat is case-sensitive on Linux but
	// protectedFiles keys are lowercase.
	for filename, protection := range protectedFiles {
		if protection != ReadOnly && protection != FullBlock {
			continue
		}
		// Check destination for existing protected files (case-insensitive).
		if dirContainsProtectedFile(dstResolved, filename) {
			return false, protection, "directory operation would overwrite protected file " + filename
		}
		// Check source for files that would overwrite protected targets.
		if src != "" && dirContainsProtectedFile(srcResolved, filename) {
			if isInWorkspace(dstResolved, workspacePath) {
				return false, protection, "directory operation would overwrite protected file " + filename
			}
		}
	}

	return true, Unprotected, ""
}

// dirContainsProtectedFile checks if a directory contains a file whose name
// matches the given protected filename (case-insensitive).
func dirContainsProtectedFile(dir, protectedName string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(e.Name(), protectedName) {
			return true
		}
	}
	return false
}

// extractAllPaths returns every filesystem path from structured payload fields.
func extractAllPaths(action *types.ActionRequest) []string {
	var paths []string
	for _, key := range []string{"path", "source", "destination", "dir", "file", "target"} {
		if v, ok := action.Payload[key].(string); ok && v != "" {
			paths = append(paths, v)
		}
	}
	return paths
}

// isWriteAction returns true for any action that modifies the filesystem.
// execute_command is NOT here — shell commands are handled by extracting write targets.
func isWriteAction(t types.ActionType) bool {
	switch t {
	case types.ActionWriteFile, types.ActionDeleteFile,
		types.ActionMoveFile, types.ActionCopyFile,
		types.ActionCreateDir, types.ActionMemoryWrite,
		types.ActionCanvasCreate, types.ActionCanvasUpdate,
		types.ActionCanvasProject,
		types.ActionCopyDir, types.ActionMoveDir, types.ActionDeleteDir:
		return true
	default:
		return false
	}
}

// isHardBlocked checks if a resolved path is a hard-blocked file or in a hard-blocked directory.
func isHardBlocked(resolved, workspacePath string) bool {
	basename := strings.ToLower(filepath.Base(resolved))
	if hardBlockedFiles[basename] {
		if isInWorkspace(resolved, workspacePath) {
			return true
		}
	}
	for _, dir := range hardBlockedDirs {
		blocked := filepath.Join(workspacePath, dir)
		if strings.HasPrefix(resolved, blocked+string(filepath.Separator)) || resolved == blocked {
			return true
		}
	}
	return false
}

// isInReadOnlyDir checks if a resolved path is inside a read-only directory.
func isInReadOnlyDir(resolved, workspacePath string) bool {
	for _, dir := range readOnlyDirs {
		roDir := filepath.Join(workspacePath, dir)
		if strings.HasPrefix(resolved, roDir+string(filepath.Separator)) || resolved == roDir {
			return true
		}
	}
	return false
}

// isInWorkspace checks if a resolved path is inside the workspace.
func isInWorkspace(resolved, workspacePath string) bool {
	return strings.HasPrefix(resolved, workspacePath+string(filepath.Separator)) || resolved == workspacePath
}

// resolveProtectionPath resolves a raw path to an absolute path using workspace as base.
func resolveProtectionPath(raw, workspacePath string) string {
	expanded := platform.NormalizePath(raw)
	if !filepath.IsAbs(expanded) {
		expanded = filepath.Join(workspacePath, expanded)
	}
	return filepath.Clean(expanded)
}

// --- Shell command write target extraction ---

var (
	// Unix patterns
	redirectRe = regexp.MustCompile(`>{1,2}\s*([^\s;|&]+)`)
	teeRe      = regexp.MustCompile(`\btee\s+(?:-a\s+)?([^\s;|&]+)`)
	cpMvRe     = regexp.MustCompile(`\b(?:cp|mv)\s+(?:-[a-zA-Z]*\s+)*([^\s]+)\s+([^\s;|&]+)`)
	rmRe       = regexp.MustCompile(`\brm\s+(?:-[a-zA-Z]*\s+)*(.+?)(?:[;|&]|$)`)

	// Windows patterns
	winCopyRe = regexp.MustCompile(`(?i)\b(?:copy|xcopy)\s+(?:/[a-zA-Z]\s+)*([^\s]+)\s+([^\s;|&]+)`)
	winMoveRe = regexp.MustCompile(`(?i)\bmove\s+(?:/[a-zA-Z]\s+)*([^\s]+)\s+([^\s;|&]+)`)
	winDelRe  = regexp.MustCompile(`(?i)\b(?:del|erase)\s+(?:/[a-zA-Z]\s+)*([^\s;|&]+)`)
	psItemRe  = regexp.MustCompile(`(?i)(?:Copy-Item|Move-Item|Remove-Item)\s+(?:-[a-zA-Z]+\s+)*['"]?([^'"\s]+)['"]?`)
	psWriteRe = regexp.MustCompile(`(?i)(?:Set-Content|Out-File|Add-Content)\s+['"]?([^'"\s]+)['"]?`)
)

// extractWriteTargetsFromCommand parses a shell command and returns only paths
// that are WRITE targets. Read-only references (cat, grep, head) are ignored.
func extractWriteTargetsFromCommand(cmd string) []string {
	var paths []string
	paths = append(paths, extractWriteTargetsUnix(cmd)...)
	paths = append(paths, extractWriteTargetsWindows(cmd)...)
	return paths
}

func extractWriteTargetsUnix(cmd string) []string {
	var paths []string

	for _, match := range redirectRe.FindAllStringSubmatch(cmd, -1) {
		if len(match) > 1 {
			paths = append(paths, match[1])
		}
	}
	for _, match := range teeRe.FindAllStringSubmatch(cmd, -1) {
		if len(match) > 1 {
			paths = append(paths, match[1])
		}
	}
	for _, match := range cpMvRe.FindAllStringSubmatch(cmd, -1) {
		if len(match) > 2 {
			paths = append(paths, match[1], match[2])
		}
	}
	for _, match := range rmRe.FindAllStringSubmatch(cmd, -1) {
		if len(match) > 1 {
			for _, p := range strings.Fields(match[1]) {
				if !strings.HasPrefix(p, "-") {
					paths = append(paths, p)
				}
			}
		}
	}

	return paths
}

func extractWriteTargetsWindows(cmd string) []string {
	var paths []string

	for _, match := range winCopyRe.FindAllStringSubmatch(cmd, -1) {
		if len(match) > 2 {
			paths = append(paths, match[1], match[2])
		}
	}
	for _, match := range winMoveRe.FindAllStringSubmatch(cmd, -1) {
		if len(match) > 2 {
			paths = append(paths, match[1], match[2])
		}
	}
	for _, match := range winDelRe.FindAllStringSubmatch(cmd, -1) {
		if len(match) > 1 {
			paths = append(paths, match[1])
		}
	}
	for _, match := range psItemRe.FindAllStringSubmatch(cmd, -1) {
		if len(match) > 1 {
			paths = append(paths, match[1])
		}
	}
	for _, match := range psWriteRe.FindAllStringSubmatch(cmd, -1) {
		if len(match) > 1 {
			paths = append(paths, match[1])
		}
	}

	return paths
}
