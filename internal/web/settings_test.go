package web

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyName(t *testing.T) {
	assert.Equal(t, "default", policyName("policies/default.yaml"))
	assert.Equal(t, "strict", policyName("policies/strict.yml"))
	assert.Equal(t, "default", policyName(""))
}

func TestIsKeyConfigured(t *testing.T) {
	assert.False(t, isKeyConfigured(""))
	assert.False(t, isKeyConfigured("NONEXISTENT_KEY_12345"))

	t.Setenv("TEST_API_KEY_SETTINGS", "sk-test")
	assert.True(t, isKeyConfigured("TEST_API_KEY_SETTINGS"))
}

func TestSettingsEmptyBodyChangesNothing(t *testing.T) {
	body := `{}`
	var parsed map[string]any
	err := json.Unmarshal([]byte(body), &parsed)
	require.NoError(t, err)
	assert.Empty(t, parsed)
}
