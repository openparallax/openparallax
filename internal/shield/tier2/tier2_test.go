package tier2

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPromptInjectsCanary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompt.md")
	require.NoError(t, os.WriteFile(path, []byte("Canary: {{CANARY_TOKEN}}"), 0o644))

	prompt, _, err := LoadPrompt(path, "abc123")
	require.NoError(t, err)
	assert.Contains(t, prompt, "abc123")
	assert.NotContains(t, prompt, "{{CANARY_TOKEN}}")
}

func TestLoadPromptHashIntegrity(t *testing.T) {
	dir := t.TempDir()
	content := "Test evaluator prompt {{CANARY_TOKEN}}"
	path := filepath.Join(dir, "prompt.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	_, hash, err := LoadPrompt(path, "token")
	require.NoError(t, err)

	expected := crypto.SHA256Hex([]byte(content))
	assert.Equal(t, expected, hash)
}

func TestLoadPromptHashChangesOnModification(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompt.md")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o644))

	_, hash1, err := LoadPrompt(path, "token")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(path, []byte("modified"), 0o644))
	_, hash2, err := LoadPrompt(path, "token")
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2)
}

func TestParseEvalResponseValid(t *testing.T) {
	response := `{"decision": "ALLOW", "confidence": 0.95, "reasoning": "looks safe", "canary": "token123"}`
	result, err := ParseEvalResponse(response)
	require.NoError(t, err)
	assert.Equal(t, types.VerdictAllow, result.Decision)
	assert.InDelta(t, 0.95, result.Confidence, 0.001)
	assert.Equal(t, "looks safe", result.Reason)
}

func TestParseEvalResponseBlock(t *testing.T) {
	response := `{"decision": "BLOCK", "confidence": 0.88, "reasoning": "dangerous"}`
	result, err := ParseEvalResponse(response)
	require.NoError(t, err)
	assert.Equal(t, types.VerdictBlock, result.Decision)
}

func TestParseEvalResponseMalformedJSON(t *testing.T) {
	_, err := ParseEvalResponse("not json at all")
	assert.Error(t, err)
}

func TestParseEvalResponseStripsCodeFences(t *testing.T) {
	response := "```json\n{\"decision\": \"ALLOW\", \"confidence\": 0.9, \"reasoning\": \"ok\"}\n```"
	result, err := ParseEvalResponse(response)
	require.NoError(t, err)
	assert.Equal(t, types.VerdictAllow, result.Decision)
}

func TestCanaryMissingMeansBlock(t *testing.T) {
	// This tests the logic that would be in Evaluator.Evaluate.
	// If canary is not in the response, the evaluator returns BLOCK.
	canary := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	response := `{"decision": "ALLOW", "confidence": 0.9, "reasoning": "safe"}`
	assert.False(t, crypto.VerifyCanary(response, canary))
}

func TestCanaryPresentPasses(t *testing.T) {
	canary := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	response := `{"decision": "ALLOW", "canary": "` + canary + `"}`
	assert.True(t, crypto.VerifyCanary(response, canary))
}
