package shield

import (
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestClassifySource_CredentialFile(t *testing.T) {
	c := ClassifySource("~/.env")
	assert.Equal(t, types.SensitivityCritical, c.Sensitivity)
	assert.Equal(t, "credential", c.ContentType)
}

func TestClassifySource_SSHDir(t *testing.T) {
	c := ClassifySource("~/.ssh/id_rsa")
	assert.Equal(t, types.SensitivityRestricted, c.Sensitivity)
}

func TestClassifySource_AgentConfig(t *testing.T) {
	c := ClassifySource("config.yaml")
	assert.Equal(t, types.SensitivityConfidential, c.Sensitivity)
}

func TestClassifySource_NormalFile(t *testing.T) {
	c := ClassifySource("~/projects/readme.md")
	assert.Equal(t, types.SensitivityPublic, c.Sensitivity)
}

func TestIsFlowAllowed_PublicToHTTP(t *testing.T) {
	c := &types.DataClassification{Sensitivity: types.SensitivityPublic}
	assert.True(t, IsFlowAllowed(c, types.ActionHTTPRequest))
}

func TestIsFlowAllowed_InternalToHTTP_Blocked(t *testing.T) {
	c := &types.DataClassification{Sensitivity: types.SensitivityInternal}
	assert.False(t, IsFlowAllowed(c, types.ActionHTTPRequest))
}

func TestIsFlowAllowed_InternalToWriteFile_Allowed(t *testing.T) {
	c := &types.DataClassification{Sensitivity: types.SensitivityInternal}
	assert.True(t, IsFlowAllowed(c, types.ActionWriteFile))
}

func TestIsFlowAllowed_ConfidentialToHTTP_Blocked(t *testing.T) {
	c := &types.DataClassification{Sensitivity: types.SensitivityConfidential}
	assert.False(t, IsFlowAllowed(c, types.ActionHTTPRequest))
}

func TestIsFlowAllowed_ConfidentialToWriteFile_Allowed(t *testing.T) {
	c := &types.DataClassification{Sensitivity: types.SensitivityConfidential}
	assert.True(t, IsFlowAllowed(c, types.ActionWriteFile))
}

func TestIsFlowAllowed_RestrictedToAnything_Blocked(t *testing.T) {
	c := &types.DataClassification{Sensitivity: types.SensitivityRestricted}
	assert.False(t, IsFlowAllowed(c, types.ActionWriteFile))
	assert.False(t, IsFlowAllowed(c, types.ActionHTTPRequest))
	assert.False(t, IsFlowAllowed(c, types.ActionSendEmail))
}

func TestIsFlowAllowed_NilClassification_Allowed(t *testing.T) {
	assert.True(t, IsFlowAllowed(nil, types.ActionHTTPRequest))
}

func TestIsFlowAllowed_ConfidentialToShell_Blocked(t *testing.T) {
	c := &types.DataClassification{Sensitivity: types.SensitivityConfidential}
	assert.False(t, IsFlowAllowed(c, types.ActionExecCommand))
}
