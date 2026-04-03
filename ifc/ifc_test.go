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

func TestIsFlowAllowed_ConfidentialToShell_Blocked(t *testing.T) {
	c := &DataClassification{Sensitivity: SensitivityConfidential}
	assert.False(t, IsFlowAllowed(c, ActionExecCommand))
}
