package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupProtectionWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create protected files.
	for _, f := range []string{"SOUL.md", "IDENTITY.md",
		"AGENTS.md", "HEARTBEAT.md", "USER.md", "MEMORY.md"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), []byte("content"), 0o644))
	}

	// Create hard-blocked files.
	for _, f := range []string{"config.yaml", "canary.token", "audit.jsonl", "evaluator-v1.md"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), []byte("secret"), 0o644))
	}

	// Create .openparallax directory.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".openparallax"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".openparallax", "openparallax.db"), []byte("db"), 0o644))

	// Create policies directory.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "policies"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "policies", "default.yaml"), []byte("policy"), 0o644))

	// Create skills directory.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "skills"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skills", "git.md"), []byte("skill"), 0o644))

	return dir
}

func makeAction(actionType types.ActionType, payload map[string]any) *types.ActionRequest {
	return &types.ActionRequest{Type: actionType, Payload: payload}
}

// --- Hardcoded protection: ReadOnly files ---

func TestProtection_WriteSOUL_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionWriteFile, map[string]any{"path": filepath.Join(ws, "SOUL.md"), "content": "evil"})
	allowed, prot, reason := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Equal(t, ReadOnly, prot)
	assert.Contains(t, reason, "protected")
}

func TestProtection_DeleteSOUL_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionDeleteFile, map[string]any{"path": filepath.Join(ws, "SOUL.md")})
	allowed, _, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
}

func TestProtection_CopyToSOUL_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionCopyFile, map[string]any{"source": filepath.Join(ws, "junk.txt"), "destination": filepath.Join(ws, "SOUL.md")})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Equal(t, ReadOnly, prot)
}

func TestProtection_MoveToSOUL_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionMoveFile, map[string]any{"source": filepath.Join(ws, "junk.txt"), "destination": filepath.Join(ws, "SOUL.md")})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Equal(t, ReadOnly, prot)
}

func TestProtection_ReadSOUL_Allowed(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionReadFile, map[string]any{"path": filepath.Join(ws, "SOUL.md")})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.True(t, allowed)
	assert.Equal(t, Unprotected, prot)
}

func TestProtection_WriteIDENTITY_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionWriteFile, map[string]any{"path": filepath.Join(ws, "IDENTITY.md"), "content": "evil"})
	allowed, _, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
}

func TestProtection_ReadIDENTITY_Allowed(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionReadFile, map[string]any{"path": filepath.Join(ws, "IDENTITY.md")})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.True(t, allowed)
	assert.Equal(t, Unprotected, prot)
}

// --- Shell command write target detection ---

func TestProtection_ShellRedirectToSOUL_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionExecCommand, map[string]any{"command": "echo 'x' > " + filepath.Join(ws, "SOUL.md")})
	allowed, _, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
}

func TestProtection_ShellCpToSOUL_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionExecCommand, map[string]any{"command": "cp " + filepath.Join(ws, "junk.txt") + " " + filepath.Join(ws, "SOUL.md")})
	allowed, _, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
}

func TestProtection_ShellTeeSOUL_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionExecCommand, map[string]any{"command": "echo x | tee " + filepath.Join(ws, "SOUL.md")})
	allowed, _, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
}

func TestProtection_ShellCatSOUL_Allowed(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionExecCommand, map[string]any{"command": "cat " + filepath.Join(ws, "SOUL.md")})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.True(t, allowed)
	assert.Equal(t, Unprotected, prot)
}

func TestProtection_ShellGrepSOUL_Allowed(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionExecCommand, map[string]any{"command": "grep pattern " + filepath.Join(ws, "SOUL.md")})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.True(t, allowed)
	assert.Equal(t, Unprotected, prot)
}

func TestProtection_ShellLs_Allowed(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionExecCommand, map[string]any{"command": "ls -la"})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.True(t, allowed)
	assert.Equal(t, Unprotected, prot)
}

func TestProtection_ShellRmSOUL_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionExecCommand, map[string]any{"command": "rm " + filepath.Join(ws, "SOUL.md")})
	allowed, _, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
}

// --- Hard-blocked files (FullBlock) ---

func TestProtection_ReadConfigYaml_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionReadFile, map[string]any{"path": filepath.Join(ws, "config.yaml")})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Equal(t, FullBlock, prot)
}

func TestProtection_ReadAuditJsonl_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionReadFile, map[string]any{"path": filepath.Join(ws, "audit.jsonl")})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Equal(t, FullBlock, prot)
}

func TestProtection_ReadOpenparallaxDir_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionReadFile, map[string]any{"path": filepath.Join(ws, ".openparallax", "openparallax.db")})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Equal(t, FullBlock, prot)
}

func TestProtection_ReadPoliciesDir_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionReadFile, map[string]any{"path": filepath.Join(ws, "policies", "default.yaml")})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Equal(t, FullBlock, prot)
}

// --- Skills directory (read-only) ---

func TestProtection_WriteSkills_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionWriteFile, map[string]any{"path": filepath.Join(ws, "skills", "custom.md"), "content": "new skill"})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Equal(t, ReadOnly, prot)
}

func TestProtection_ReadSkills_Allowed(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionReadFile, map[string]any{"path": filepath.Join(ws, "skills", "git.md")})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.True(t, allowed)
	assert.Equal(t, Unprotected, prot)
}

// --- Outside workspace: not protected ---

func TestProtection_WriteSOULOutsideWorkspace_Allowed(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	otherDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(otherDir, "SOUL.md"), []byte("x"), 0o644))
	action := makeAction(types.ActionWriteFile, map[string]any{"path": filepath.Join(otherDir, "SOUL.md"), "content": "ok"})
	allowed, _, _ := CheckProtection(action, ws)
	assert.True(t, allowed)
}

// --- Symlink bypass ---

func TestProtection_SymlinkToSOUL_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	symlinkPath := filepath.Join(ws, "innocent.txt")
	require.NoError(t, os.Symlink(filepath.Join(ws, "SOUL.md"), symlinkPath))
	action := makeAction(types.ActionWriteFile, map[string]any{"path": symlinkPath, "content": "evil"})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Equal(t, ReadOnly, prot)
}

func TestProtection_SymlinkToConfig_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	symlinkPath := filepath.Join(ws, "safe.txt")
	require.NoError(t, os.Symlink(filepath.Join(ws, "config.yaml"), symlinkPath))
	action := makeAction(types.ActionReadFile, map[string]any{"path": symlinkPath})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Equal(t, FullBlock, prot)
}

// --- Directory overwrite ---

func TestProtection_CopyDirOverwritesSOUL_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	srcDir := filepath.Join(ws, "evil-dir")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "soul.md"), []byte("evil"), 0o644))

	action := makeAction(types.ActionCopyDir, map[string]any{"source": srcDir, "destination": ws})
	allowed, _, reason := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Contains(t, reason, "overwrite protected file")
}

func TestProtection_MoveDirOverwritesIDENTITY_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	srcDir := filepath.Join(ws, "bad-dir")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "identity.md"), []byte("evil"), 0o644))

	action := makeAction(types.ActionMoveDir, map[string]any{"source": srcDir, "destination": ws})
	allowed, _, reason := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Contains(t, reason, "overwrite protected file")
}

// --- Tier escalation (MinTier) ---

func TestProtection_WriteHEARTBEAT_EscalateTier2(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionWriteFile, map[string]any{"path": filepath.Join(ws, "HEARTBEAT.md"), "content": "schedule"})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.True(t, allowed)
	assert.Equal(t, EscalateTier2, prot)
}

func TestProtection_WriteAGENTS_EscalateTier2(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionWriteFile, map[string]any{"path": filepath.Join(ws, "AGENTS.md"), "content": "agent def"})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.True(t, allowed)
	assert.Equal(t, EscalateTier2, prot)
}

func TestProtection_DeleteAGENTS_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionDeleteFile, map[string]any{"path": filepath.Join(ws, "AGENTS.md")})
	allowed, _, reason := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Contains(t, reason, "cannot be deleted")
}

func TestProtection_DeleteHEARTBEAT_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionDeleteFile, map[string]any{"path": filepath.Join(ws, "HEARTBEAT.md")})
	allowed, _, reason := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Contains(t, reason, "cannot be deleted")
}

func TestProtection_ShellRmAGENTS_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionExecCommand, map[string]any{"command": "rm " + filepath.Join(ws, "AGENTS.md")})
	allowed, _, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
}

func TestProtection_WriteUSER_WriteTier1Min(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionWriteFile, map[string]any{"path": filepath.Join(ws, "USER.md"), "content": "timezone: UTC"})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.True(t, allowed)
	assert.Equal(t, WriteTier1Min, prot)
}

func TestProtection_WriteMEMORY_WriteTier1Min(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionWriteFile, map[string]any{"path": filepath.Join(ws, "MEMORY.md"), "content": "fact"})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.True(t, allowed)
	assert.Equal(t, WriteTier1Min, prot)
}

// --- Case-insensitive matching ---

func TestProtection_WriteLowercaseSOUL_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionWriteFile, map[string]any{"path": filepath.Join(ws, "soul.md"), "content": "evil"})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Equal(t, ReadOnly, prot)
}

func TestProtection_WriteMixedCaseSOUL_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionWriteFile, map[string]any{"path": filepath.Join(ws, "Soul.MD"), "content": "evil"})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Equal(t, ReadOnly, prot)
}

// --- Windows shell patterns ---

func TestProtection_WinCopyToSOUL_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionExecCommand, map[string]any{"command": "copy " + filepath.Join(ws, "junk.txt") + " " + filepath.Join(ws, "SOUL.md")})
	allowed, _, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
}

func TestProtection_WinSetContentSOUL_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionExecCommand, map[string]any{"command": "Set-Content " + filepath.Join(ws, "SOUL.md") + " -Value 'x'"})
	allowed, _, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
}

func TestProtection_WinDelSOUL_Blocked(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionExecCommand, map[string]any{"command": "del " + filepath.Join(ws, "SOUL.md")})
	allowed, _, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
}

// --- Shell write to escalation files ---

func TestProtection_ShellRedirectToUSER_EscalateTier1(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionExecCommand, map[string]any{"command": "echo 'tz: UTC' >> " + filepath.Join(ws, "USER.md")})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.True(t, allowed)
	assert.Equal(t, WriteTier1Min, prot)
}

func TestProtection_ShellRedirectToHEARTBEAT_EscalateTier2(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionExecCommand, map[string]any{"command": "echo 'schedule' > " + filepath.Join(ws, "HEARTBEAT.md")})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.True(t, allowed)
	assert.Equal(t, EscalateTier2, prot)
}

// --- Write target extraction ---

func TestExtractWriteTargets_Redirect(t *testing.T) {
	targets := extractWriteTargetsFromCommand("echo hello > output.txt")
	assert.Contains(t, targets, "output.txt")
}

func TestExtractWriteTargets_AppendRedirect(t *testing.T) {
	targets := extractWriteTargetsFromCommand("echo hello >> output.txt")
	assert.Contains(t, targets, "output.txt")
}

func TestExtractWriteTargets_CpMv(t *testing.T) {
	targets := extractWriteTargetsFromCommand("cp source.txt dest.txt")
	assert.Contains(t, targets, "dest.txt")
	assert.Contains(t, targets, "source.txt")
}

func TestExtractWriteTargets_Rm(t *testing.T) {
	targets := extractWriteTargetsFromCommand("rm -f file.txt")
	assert.Contains(t, targets, "file.txt")
}

func TestExtractWriteTargets_Tee(t *testing.T) {
	targets := extractWriteTargetsFromCommand("echo x | tee output.log")
	assert.Contains(t, targets, "output.log")
}

func TestExtractWriteTargets_WinCopy(t *testing.T) {
	targets := extractWriteTargetsFromCommand("copy source.txt dest.txt")
	assert.Contains(t, targets, "dest.txt")
}

func TestExtractWriteTargets_WinDel(t *testing.T) {
	targets := extractWriteTargetsFromCommand("del file.txt")
	assert.Contains(t, targets, "file.txt")
}

func TestExtractWriteTargets_PowerShellSetContent(t *testing.T) {
	targets := extractWriteTargetsFromCommand("Set-Content output.txt -Value 'hello'")
	assert.Contains(t, targets, "output.txt")
}

func TestExtractWriteTargets_NoTargets(t *testing.T) {
	targets := extractWriteTargetsFromCommand("ls -la")
	assert.Empty(t, targets)
}

func TestExtractWriteTargets_CatNotWriteTarget(t *testing.T) {
	targets := extractWriteTargetsFromCommand("cat file.txt")
	assert.Empty(t, targets)
}

// --- Policy path extraction ---

func TestExtractPolicyPaths_IncludesDestination(t *testing.T) {
	action := makeAction(types.ActionCopyFile, map[string]any{"source": "a.txt", "destination": "SOUL.md"})
	paths := extractAllPaths(action)
	assert.Contains(t, paths, "a.txt")
	assert.Contains(t, paths, "SOUL.md")
}

// --- Unprotected file operations ---

func TestProtection_WriteNormalFile_Allowed(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionWriteFile, map[string]any{"path": filepath.Join(ws, "notes.txt"), "content": "hello"})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.True(t, allowed)
	assert.Equal(t, Unprotected, prot)
}

func TestProtection_DeleteNormalFile_Allowed(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionDeleteFile, map[string]any{"path": filepath.Join(ws, "notes.txt")})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.True(t, allowed)
	assert.Equal(t, Unprotected, prot)
}

// --- Absolute path enforcement ---

func TestProtection_RelativePath_Rejected(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionReadFile, map[string]any{"path": "notes.txt"})
	allowed, prot, reason := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Equal(t, FullBlock, prot)
	assert.Contains(t, reason, "relative")
}

func TestProtection_TildePath_Allowed(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionReadFile, map[string]any{"path": "~/Desktop/notes.txt"})
	allowed, _, _ := CheckProtection(action, ws)
	assert.True(t, allowed)
}

func TestProtection_ShellRelativeWriteTarget_Rejected(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	action := makeAction(types.ActionExecCommand, map[string]any{"command": "echo x > notes.txt"})
	allowed, _, reason := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Contains(t, reason, "relative")
}

func TestProtection_ShellCDPrefix_Allowed(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	cmd := "cd " + ws + " && echo x > notes.txt"
	action := makeAction(types.ActionExecCommand, map[string]any{"command": cmd})
	allowed, _, _ := CheckProtection(action, ws)
	assert.True(t, allowed)
}

func TestProtection_ShellCDPrefix_BlocksProtectedTarget(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	// cd into ws, then write to SOUL.md (relative — resolved against the cd target)
	cmd := "cd " + ws + " && echo x > SOUL.md"
	action := makeAction(types.ActionExecCommand, map[string]any{"command": cmd})
	allowed, prot, _ := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Equal(t, ReadOnly, prot)
}

func TestProtection_ShellCDPrefixRelative_Rejected(t *testing.T) {
	ws := setupProtectionWorkspace(t)
	// Relative cd target — must be rejected.
	action := makeAction(types.ActionExecCommand, map[string]any{"command": "cd backend && rm main.go"})
	allowed, _, reason := CheckProtection(action, ws)
	assert.False(t, allowed)
	assert.Contains(t, reason, "relative")
}
