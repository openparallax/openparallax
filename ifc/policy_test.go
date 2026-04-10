package ifc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Preset loading ---

func presetPath(t *testing.T, name string) string {
	t.Helper()
	candidates := []string{
		filepath.Join("../internal/templates/files/security/ifc", name),
		filepath.Join("internal/templates/files/security/ifc", name),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	t.Fatalf("%s not found", name)
	return ""
}

func TestLoadDefaultPreset(t *testing.T) {
	p, err := LoadPolicy(presetPath(t, "default.yaml"))
	require.NoError(t, err)
	assert.Equal(t, ModeEnforce, p.Mode)
	assert.True(t, len(p.Sources) > 5, "default should have many source rules")
	assert.Equal(t, 4, len(p.Sinks), "should have 4 sink categories")
}

func TestLoadPermissivePreset(t *testing.T) {
	p, err := LoadPolicy(presetPath(t, "permissive.yaml"))
	require.NoError(t, err)
	assert.Equal(t, ModeEnforce, p.Mode)
}

func TestLoadStrictPreset(t *testing.T) {
	p, err := LoadPolicy(presetPath(t, "strict.yaml"))
	require.NoError(t, err)
	assert.Equal(t, ModeEnforce, p.Mode)
}

// --- Classification ---

func loadDefault(t *testing.T) *Policy {
	t.Helper()
	p, err := LoadPolicy(presetPath(t, "default.yaml"))
	require.NoError(t, err)
	return p
}

func TestClassify_SSHKeyIsCritical(t *testing.T) {
	p := loadDefault(t)
	cls := p.Classify("/home/user/.ssh/id_rsa")
	require.NotNil(t, cls)
	assert.Equal(t, SensitivityCritical, cls.Sensitivity)
}

func TestClassify_EnvFileIsCritical(t *testing.T) {
	p := loadDefault(t)
	cls := p.Classify("/home/user/project/.env")
	require.NotNil(t, cls)
	assert.Equal(t, SensitivityCritical, cls.Sensitivity)
}

func TestClassify_EnvExampleIsPublic(t *testing.T) {
	p := loadDefault(t)
	cls := p.Classify("/home/user/project/.env.example")
	assert.Nil(t, cls, ".env.example should be public")
}

func TestClassify_PemFileIsCritical(t *testing.T) {
	p := loadDefault(t)
	cls := p.Classify("/home/user/certs/server.pem")
	require.NotNil(t, cls)
	assert.Equal(t, SensitivityCritical, cls.Sensitivity)
}

func TestClassify_InvoiceIsRestricted(t *testing.T) {
	p := loadDefault(t)
	cls := p.Classify("/home/user/docs/invoice-2024.pdf")
	require.NotNil(t, cls)
	assert.Equal(t, SensitivityRestricted, cls.Sensitivity)
}

func TestClassify_ConfigYamlUnderOpenparallax(t *testing.T) {
	p := loadDefault(t)
	cls := p.Classify("/home/user/.openparallax/indigo/config.yaml")
	require.NotNil(t, cls)
	// config.yaml's basename matches the workspace_config rule (confidential).
	// The openparallax_internals rule requires BOTH path_contains AND basename_in
	// to match — config.yaml is not in [canary.token, audit.jsonl, openparallax.db].
	// This is fine: the protection layer hard-blocks .openparallax/ writes anyway.
	assert.Equal(t, SensitivityConfidential, cls.Sensitivity)
}

func TestClassify_WorkspaceConfigYaml(t *testing.T) {
	p := loadDefault(t)
	cls := p.Classify("/home/user/myproject/config.yaml")
	require.NotNil(t, cls)
	assert.Equal(t, SensitivityConfidential, cls.Sensitivity)
}

func TestClassify_RegularSourceCode(t *testing.T) {
	p := loadDefault(t)
	cls := p.Classify("/home/user/project/main.go")
	assert.Nil(t, cls, "regular source code should be public")
}

func TestClassify_EmptyPathIsPublic(t *testing.T) {
	p := loadDefault(t)
	cls := p.Classify("")
	assert.Nil(t, cls)
}

func TestClassify_TokenizerPyIsPublic(t *testing.T) {
	p := loadDefault(t)
	cls := p.Classify("/home/user/project/tokenizer.py")
	assert.Nil(t, cls, "tokenizer.py should not false-positive on 'token'")
}

func TestClassify_PasswordTutorialIsPublic(t *testing.T) {
	p := loadDefault(t)
	cls := p.Classify("/home/user/tutorials/password-tutorial.md")
	assert.Nil(t, cls, "tutorial about passwords should not be classified")
}

// --- Decision matrix ---

func TestDecide_PublicToExternal_Allowed(t *testing.T) {
	p := loadDefault(t)
	assert.Equal(t, DecisionAllow, p.Decide(nil, ActionHTTPRequest))
}

func TestDecide_CriticalToExternal_Blocked(t *testing.T) {
	p := loadDefault(t)
	cls := &DataClassification{Sensitivity: SensitivityCritical}
	assert.Equal(t, DecisionBlock, p.Decide(cls, ActionHTTPRequest))
}

func TestDecide_CriticalToRead_Blocked(t *testing.T) {
	p := loadDefault(t)
	cls := &DataClassification{Sensitivity: SensitivityCritical}
	assert.Equal(t, DecisionBlock, p.Decide(cls, ActionReadFile))
}

func TestDecide_ConfidentialToExternal_Blocked(t *testing.T) {
	p := loadDefault(t)
	cls := &DataClassification{Sensitivity: SensitivityConfidential}
	assert.Equal(t, DecisionBlock, p.Decide(cls, ActionSendEmail))
}

func TestDecide_ConfidentialToWrite_Allowed(t *testing.T) {
	p := loadDefault(t)
	cls := &DataClassification{Sensitivity: SensitivityConfidential}
	assert.Equal(t, DecisionAllow, p.Decide(cls, ActionWriteFile))
}

func TestDecide_RestrictedToExec_Escalated(t *testing.T) {
	p := loadDefault(t)
	cls := &DataClassification{Sensitivity: SensitivityRestricted}
	assert.Equal(t, DecisionEscalate, p.Decide(cls, ActionExecCommand))
}

func TestDecide_UnknownActionType_DefaultAllow(t *testing.T) {
	p := loadDefault(t)
	cls := &DataClassification{Sensitivity: SensitivityConfidential}
	assert.Equal(t, DecisionAllow, p.Decide(cls, ActionType("mcp_custom_tool")))
}

// --- Permissive preset behavior ---

func TestPermissive_RestrictedToExternal_Allowed(t *testing.T) {
	p, err := LoadPolicy(presetPath(t, "permissive.yaml"))
	require.NoError(t, err)
	cls := &DataClassification{Sensitivity: SensitivityRestricted}
	assert.Equal(t, DecisionAllow, p.Decide(cls, ActionHTTPRequest))
}

func TestPermissive_CriticalToExternal_Blocked(t *testing.T) {
	p, err := LoadPolicy(presetPath(t, "permissive.yaml"))
	require.NoError(t, err)
	cls := &DataClassification{Sensitivity: SensitivityCritical}
	assert.Equal(t, DecisionBlock, p.Decide(cls, ActionHTTPRequest))
}

// --- Strict preset behavior ---

func TestStrict_ConfidentialToWrite_Escalated(t *testing.T) {
	p, err := LoadPolicy(presetPath(t, "strict.yaml"))
	require.NoError(t, err)
	cls := &DataClassification{Sensitivity: SensitivityConfidential}
	assert.Equal(t, DecisionEscalate, p.Decide(cls, ActionWriteFile))
}

func TestStrict_RestrictedToWrite_Blocked(t *testing.T) {
	p, err := LoadPolicy(presetPath(t, "strict.yaml"))
	require.NoError(t, err)
	cls := &DataClassification{Sensitivity: SensitivityRestricted}
	assert.Equal(t, DecisionBlock, p.Decide(cls, ActionWriteFile))
}

// --- Validation ---

func TestParsePolicy_InvalidMode(t *testing.T) {
	_, err := ParsePolicy([]byte("mode: garbage\nsources: []\nsinks: {}\nrules: {}"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid mode")
}

func TestParsePolicy_InvalidSensitivity(t *testing.T) {
	_, err := ParsePolicy([]byte(`
mode: enforce
sources:
  - name: bad
    sensitivity: nonexistent
    match: {}
sinks:
  cat: [read_file]
rules:
  public:
    cat: allow
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown sensitivity")
}

func TestParsePolicy_DuplicateActionInSinks(t *testing.T) {
	_, err := ParsePolicy([]byte(`
mode: enforce
sources: []
sinks:
  a: [read_file]
  b: [read_file]
rules:
  public:
    a: allow
    b: allow
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "appears in sink categories")
}

func TestParsePolicy_MissingDecision(t *testing.T) {
	_, err := ParsePolicy([]byte(`
mode: enforce
sources: []
sinks:
  a: [read_file]
  b: [write_file]
rules:
  public:
    a: allow
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing decision")
}
