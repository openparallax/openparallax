package tier0

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func policyPath(t *testing.T, name string) string {
	t.Helper()
	// Find policies relative to repo root.
	candidates := []string{
		filepath.Join("../../../policies", name),
		filepath.Join("policies", name),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	t.Fatalf("policy file %s not found", name)
	return ""
}

func TestLoadDefaultPolicy(t *testing.T) {
	pe, err := NewPolicyEngine(policyPath(t, "default.yaml"))
	require.NoError(t, err)
	assert.Greater(t, pe.DenyCount(), 0)
	assert.Greater(t, pe.AllowCount(), 0)
}

func TestLoadStrictPolicy(t *testing.T) {
	pe, err := NewPolicyEngine(policyPath(t, "strict.yaml"))
	require.NoError(t, err)

	defaultPE, _ := NewPolicyEngine(policyPath(t, "default.yaml"))
	assert.GreaterOrEqual(t, pe.DenyCount(), defaultPE.DenyCount(),
		"strict should have at least as many deny rules as default")
}

func TestLoadPermissivePolicy(t *testing.T) {
	pe, err := NewPolicyEngine(policyPath(t, "permissive.yaml"))
	require.NoError(t, err)

	defaultPE, _ := NewPolicyEngine(policyPath(t, "default.yaml"))
	assert.GreaterOrEqual(t, pe.AllowCount(), defaultPE.AllowCount(),
		"permissive should have at least as many allow rules as default")
}

func TestDenyBlocksSSHKey(t *testing.T) {
	pe, err := NewPolicyEngine(policyPath(t, "default.yaml"))
	require.NoError(t, err)

	home, _ := os.UserHomeDir()
	result := pe.Evaluate(&types.ActionRequest{
		Type:    types.ActionReadFile,
		Payload: map[string]any{"path": filepath.Join(home, ".ssh", "id_rsa")},
	})
	assert.Equal(t, Deny, result.Decision)
}

func TestAllowWorkspaceRead(t *testing.T) {
	pe, err := NewPolicyEngine(policyPath(t, "default.yaml"))
	require.NoError(t, err)

	result := pe.Evaluate(&types.ActionRequest{
		Type:    types.ActionReadFile,
		Payload: map[string]any{"path": "~/workspace/SOUL.md"},
	})
	assert.Equal(t, Allow, result.Decision)
}

func TestVerifyEscalatesSOULWrite(t *testing.T) {
	pe, err := NewPolicyEngine(policyPath(t, "default.yaml"))
	require.NoError(t, err)

	result := pe.Evaluate(&types.ActionRequest{
		Type:    types.ActionWriteFile,
		Payload: map[string]any{"path": "/home/user/workspace/SOUL.md"},
	})
	assert.Equal(t, Escalate, result.Decision)
	assert.Equal(t, 2, result.EscalateTo)
}

func TestVerifyEscalatesShellCommand(t *testing.T) {
	pe, err := NewPolicyEngine(policyPath(t, "default.yaml"))
	require.NoError(t, err)

	result := pe.Evaluate(&types.ActionRequest{
		Type:    types.ActionExecCommand,
		Payload: map[string]any{"command": "ls -la"},
	})
	assert.Equal(t, Escalate, result.Decision)
}

func TestNoMatchFallsThrough(t *testing.T) {
	pe, err := NewPolicyEngine(policyPath(t, "default.yaml"))
	require.NoError(t, err)

	result := pe.Evaluate(&types.ActionRequest{
		Type:    types.ActionWriteFile,
		Payload: map[string]any{"path": "/tmp/random.txt"},
	})
	assert.Equal(t, NoMatch, result.Decision)
}

func TestGlobMatchSSHWildcard(t *testing.T) {
	pe, err := NewPolicyEngine(policyPath(t, "default.yaml"))
	require.NoError(t, err)

	home, _ := os.UserHomeDir()
	result := pe.Evaluate(&types.ActionRequest{
		Type:    types.ActionReadFile,
		Payload: map[string]any{"path": filepath.Join(home, ".ssh", "config")},
	})
	assert.Equal(t, Deny, result.Decision, "~/.ssh/** should match ~/.ssh/config")
}

func TestAllowMemorySearch(t *testing.T) {
	pe, err := NewPolicyEngine(policyPath(t, "default.yaml"))
	require.NoError(t, err)

	result := pe.Evaluate(&types.ActionRequest{
		Type:    types.ActionMemorySearch,
		Payload: map[string]any{},
	})
	assert.Equal(t, Allow, result.Decision)
}

func TestLoadMissingPolicyFile(t *testing.T) {
	_, err := NewPolicyEngine("/nonexistent/policy.yaml")
	assert.Error(t, err)
}
