package parser

import (
	"context"
	"os"
	"testing"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestProvider(t *testing.T) llm.Provider {
	t.Helper()
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	model := os.Getenv("OPENAI_MODEL")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	p, err := llm.NewOpenAIProvider(apiKey, model, baseURL)
	require.NoError(t, err)
	return p
}

func TestParseFileRequest(t *testing.T) {
	p := New(getTestProvider(t))
	intent, err := p.Parse(context.Background(), "read my SOUL.md")
	require.NoError(t, err)
	assert.Equal(t, types.GoalFileManagement, intent.Goal)
	assert.Contains(t, string(intent.PrimaryAction), "read")
}

func TestParseShellRequest(t *testing.T) {
	p := New(getTestProvider(t))
	intent, err := p.Parse(context.Background(), "execute the shell command: ls -la")
	require.NoError(t, err)
	assert.Contains(t, string(intent.PrimaryAction), "execute",
		"action should involve execution")
}

func TestParseConversation(t *testing.T) {
	p := New(getTestProvider(t))
	intent, err := p.Parse(context.Background(), "hello how are you")
	require.NoError(t, err)
	assert.Equal(t, types.GoalConversation, intent.Goal)
}

func TestParseDestructiveRequest(t *testing.T) {
	p := New(getTestProvider(t))
	intent, err := p.Parse(context.Background(), "delete all files in /tmp")
	require.NoError(t, err)
	assert.True(t, intent.Destructive)
}

// Validator unit tests — no LLM needed.

func TestValidatorNormalizesGoal(t *testing.T) {
	v := NewValidator()
	raw := &RawIntent{Goal: "file_management", Action: "read_file", Confidence: 0.9}
	intent, err := v.Validate(raw, "read a file")
	require.NoError(t, err)
	assert.Equal(t, types.GoalFileManagement, intent.Goal)
}

func TestValidatorNormalizesUnknownGoal(t *testing.T) {
	v := NewValidator()
	raw := &RawIntent{Goal: "something_unknown", Action: "read_file", Confidence: 0.9}
	intent, err := v.Validate(raw, "do something")
	require.NoError(t, err)
	// Should fall back to a known goal or conversation.
	assert.NotEmpty(t, intent.Goal)
}

func TestValidatorClampsConfidence(t *testing.T) {
	v := NewValidator()

	raw := &RawIntent{Goal: "conversation", Confidence: -0.5}
	intent, err := v.Validate(raw, "test")
	require.NoError(t, err)
	assert.Equal(t, 0.0, intent.Confidence)

	raw = &RawIntent{Goal: "conversation", Confidence: 1.5}
	intent, err = v.Validate(raw, "test")
	require.NoError(t, err)
	assert.Equal(t, 1.0, intent.Confidence)
}

func TestValidatorCrossChecksDestructive(t *testing.T) {
	v := NewValidator()
	raw := &RawIntent{Goal: "code_execution", Action: "execute_command", Destructive: false}
	intent, err := v.Validate(raw, "rm -rf /tmp/test")
	require.NoError(t, err)
	assert.True(t, intent.Destructive, "keyword detector should override LLM's false destructive flag")
}

func TestKeywordDetectorMatchesDestructive(t *testing.T) {
	kd := NewKeywordDetector()

	assert.True(t, kd.IsDestructive("rm -rf /"))
	assert.True(t, kd.IsDestructive("drop table users"))
	assert.True(t, kd.IsDestructive("git push --force"))
	assert.True(t, kd.IsDestructive("sudo rm something"))
}

func TestKeywordDetectorNoBenignFalsePositive(t *testing.T) {
	kd := NewKeywordDetector()

	assert.False(t, kd.IsDestructive("remove the item from the list"))
	assert.False(t, kd.IsDestructive("read the file"))
	assert.False(t, kd.IsDestructive("hello world"))
}

func TestSensitivitySSHKey(t *testing.T) {
	sl := NewSensitivityLookup()
	level := sl.Evaluate(map[string]string{"path": "~/.ssh/id_rsa"})
	assert.Equal(t, types.SensitivityCritical, level)
}

func TestSensitivityNormalFile(t *testing.T) {
	sl := NewSensitivityLookup()
	level := sl.Evaluate(map[string]string{"path": "~/documents/notes.txt"})
	assert.Equal(t, types.SensitivityPublic, level)
}

func TestSensitivityCredentialPattern(t *testing.T) {
	sl := NewSensitivityLookup()
	level := sl.Evaluate(map[string]string{"path": "/tmp/.env"})
	assert.Equal(t, types.SensitivityCritical, level)
}

func TestSensitivityFinancialPattern(t *testing.T) {
	sl := NewSensitivityLookup()
	level := sl.Evaluate(map[string]string{"path": "~/documents/tax_return.pdf"})
	assert.Equal(t, types.SensitivityRestricted, level)
}

func TestTaxonomyNormalizeDirect(t *testing.T) {
	tax := NewGoalTaxonomy()
	assert.Equal(t, types.GoalFileManagement, tax.Normalize("file_management"))
	assert.Equal(t, types.GoalGitOperations, tax.Normalize("git_operations"))
}

func TestTaxonomyNormalizeKeyword(t *testing.T) {
	tax := NewGoalTaxonomy()
	assert.Equal(t, types.GoalFileManagement, tax.Normalize("file_ops"))
	assert.Equal(t, types.GoalCodeExecution, tax.Normalize("execute_cmd"))
}

func TestTaxonomyNormalizeFallback(t *testing.T) {
	tax := NewGoalTaxonomy()
	assert.Equal(t, types.GoalConversation, tax.Normalize("xyznonexistent"))
}

func TestStripCodeFences(t *testing.T) {
	input := "```json\n{\"goal\": \"file\"}\n```"
	result := stripCodeFences(input)
	assert.Equal(t, "{\"goal\": \"file\"}", result)
}

func TestStripCodeFencesNoFences(t *testing.T) {
	input := `{"goal": "file"}`
	result := stripCodeFences(input)
	assert.Equal(t, input, result)
}
