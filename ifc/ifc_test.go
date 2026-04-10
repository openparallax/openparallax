package ifc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifySource_CredentialFile(t *testing.T) {
	c := ClassifySource("~/.env")
	assert.Equal(t, SensitivityCritical, c.Sensitivity)
	assert.Equal(t, "credential", c.ContentType)
}

func TestClassifySource_SSHDir(t *testing.T) {
	c := ClassifySource("~/.ssh/id_rsa")
	assert.Equal(t, SensitivityRestricted, c.Sensitivity)
}

func TestClassifySource_AgentConfig(t *testing.T) {
	c := ClassifySource("config.yaml")
	assert.Equal(t, SensitivityConfidential, c.Sensitivity)
}

func TestClassifySource_NormalFile(t *testing.T) {
	c := ClassifySource("~/projects/readme.md")
	assert.Equal(t, SensitivityPublic, c.Sensitivity)
}

func TestIsFlowAllowed_PublicToHTTP(t *testing.T) {
	c := &DataClassification{Sensitivity: SensitivityPublic}
	assert.True(t, IsFlowAllowed(c, ActionHTTPRequest))
}

func TestIsFlowAllowed_InternalToHTTP_Blocked(t *testing.T) {
	c := &DataClassification{Sensitivity: SensitivityInternal}
	assert.False(t, IsFlowAllowed(c, ActionHTTPRequest))
}

func TestIsFlowAllowed_InternalToWriteFile_Allowed(t *testing.T) {
	c := &DataClassification{Sensitivity: SensitivityInternal}
	assert.True(t, IsFlowAllowed(c, ActionWriteFile))
}

func TestIsFlowAllowed_ConfidentialToHTTP_Blocked(t *testing.T) {
	c := &DataClassification{Sensitivity: SensitivityConfidential}
	assert.False(t, IsFlowAllowed(c, ActionHTTPRequest))
}

func TestIsFlowAllowed_ConfidentialToWriteFile_Allowed(t *testing.T) {
	c := &DataClassification{Sensitivity: SensitivityConfidential}
	assert.True(t, IsFlowAllowed(c, ActionWriteFile))
}

func TestIsFlowAllowed_RestrictedToAnything_Blocked(t *testing.T) {
	c := &DataClassification{Sensitivity: SensitivityRestricted}
	assert.False(t, IsFlowAllowed(c, ActionWriteFile))
	assert.False(t, IsFlowAllowed(c, ActionHTTPRequest))
	assert.False(t, IsFlowAllowed(c, ActionSendEmail))
}

func TestIsFlowAllowed_NilClassification_Allowed(t *testing.T) {
	assert.True(t, IsFlowAllowed(nil, ActionHTTPRequest))
}

// Confidential data is allowed to flow to non-external sinks (shell, MCP
// tools, future built-ins). Only HTTP/email/channel sinks are blocked.
// This matches the Internal rule and avoids fail-closed-by-typo: an unknown
// action type should not be blocked just because it isn't on a hand-curated
// allowlist.
func TestIsFlowAllowed_ConfidentialToShell_Allowed(t *testing.T) {
	c := &DataClassification{Sensitivity: SensitivityConfidential}
	assert.True(t, IsFlowAllowed(c, ActionExecCommand))
}

// Confidential data must still be blocked from external sinks (HTTP, mail,
// channel messages). This is the only sink class that gets blocked.
func TestIsFlowAllowed_ConfidentialToEmail_Blocked(t *testing.T) {
	c := &DataClassification{Sensitivity: SensitivityConfidential}
	assert.False(t, IsFlowAllowed(c, ActionSendEmail))
	assert.False(t, IsFlowAllowed(c, ActionSendMessage))
}

// MCP/sub-agent/unknown action types must be allowed for Confidential data
// — the previous allowlist behavior blocked them silently. This is a
// regression guard for that specific class of false positive.
func TestIsFlowAllowed_ConfidentialToUnknownAction_Allowed(t *testing.T) {
	c := &DataClassification{Sensitivity: SensitivityConfidential}
	assert.True(t, IsFlowAllowed(c, ActionType("mcp_unknown_tool")))
}
