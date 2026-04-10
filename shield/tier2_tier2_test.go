package shield

import (
	"testing"

	"github.com/openparallax/openparallax/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPromptInjectsCanary(t *testing.T) {
	prompt, _ := LoadPrompt("abc123")
	assert.Contains(t, prompt, "abc123")
	assert.NotContains(t, prompt, "{{CANARY_TOKEN}}")
}

func TestLoadPromptHashIsStable(t *testing.T) {
	_, hash1 := LoadPrompt("token-a")
	_, hash2 := LoadPrompt("token-b")
	// Hash is of the raw template (before canary injection), so it
	// must be identical regardless of the canary value.
	assert.Equal(t, hash1, hash2)
}

func TestLoadPromptContainsEvaluationCriteria(t *testing.T) {
	prompt, _ := LoadPrompt("token")
	assert.Contains(t, prompt, "Data Safety")
	assert.Contains(t, prompt, "Injection Indicators")
	assert.Contains(t, prompt, "Exfiltration Risk")
}

func TestLoadPromptHashIsNonEmpty(t *testing.T) {
	_, hash := LoadPrompt("token")
	assert.Len(t, hash, 64) // SHA-256 hex = 64 chars
}

func TestParseEvalResponseValid(t *testing.T) {
	response := `{"decision": "ALLOW", "confidence": 0.95, "reasoning": "looks safe", "canary": "token123"}`
	result, err := ParseEvalResponse(response)
	require.NoError(t, err)
	assert.Equal(t, VerdictAllow, result.Decision)
	assert.InDelta(t, 0.95, result.Confidence, 0.001)
	assert.Equal(t, "looks safe", result.Reason)
}

func TestParseEvalResponseBlock(t *testing.T) {
	response := `{"decision": "BLOCK", "confidence": 0.88, "reasoning": "dangerous"}`
	result, err := ParseEvalResponse(response)
	require.NoError(t, err)
	assert.Equal(t, VerdictBlock, result.Decision)
}

func TestParseEvalResponseMalformedJSON(t *testing.T) {
	_, err := ParseEvalResponse("not json at all")
	assert.Error(t, err)
}

func TestParseEvalResponseStripsCodeFences(t *testing.T) {
	response := "```json\n{\"decision\": \"ALLOW\", \"confidence\": 0.9, \"reasoning\": \"ok\"}\n```"
	result, err := ParseEvalResponse(response)
	require.NoError(t, err)
	assert.Equal(t, VerdictAllow, result.Decision)
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
