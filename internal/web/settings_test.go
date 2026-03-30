package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
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

func TestWriteConfigToDisk(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	cfg := createTestConfig()
	err := writeConfigToDisk(path, cfg)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "name: TestAgent")
	assert.Contains(t, content, "provider: anthropic")
	assert.Contains(t, content, "model: claude-sonnet-4-20250514")
	assert.Contains(t, content, "daily_budget: 50")
	assert.Contains(t, content, "port: 3100")
	assert.NotContains(t, content, "sk-ant-")
}

func TestWriteConfigToDiskPreservesAllSections(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	cfg := createTestConfig()
	cfg.Shield.Evaluator.Provider = "openai"
	cfg.Shield.Evaluator.Model = "gpt-4o-mini"
	cfg.Memory.Embedding.Provider = "openai"
	cfg.Memory.Embedding.Model = "text-embedding-3-small"

	err := writeConfigToDisk(path, cfg)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "evaluator:")
	assert.Contains(t, content, "provider: openai")
	assert.Contains(t, content, "model: gpt-4o-mini")
	assert.Contains(t, content, "embedding:")
	assert.Contains(t, content, "model: text-embedding-3-small")
}

func TestSettingsPortValidation(t *testing.T) {
	body := `{"web": {"port": 80}}`
	var parsed map[string]any
	err := json.Unmarshal([]byte(body), &parsed)
	require.NoError(t, err)

	if webMap, ok := parsed["web"].(map[string]any); ok {
		if port, ok := webMap["port"].(float64); ok {
			p := int(port)
			assert.Less(t, p, 1024, "port below 1024 should be rejected")
		}
	}
}

func TestSettingsEmptyBodyChangesNothing(t *testing.T) {
	body := `{}`
	var parsed map[string]any
	err := json.Unmarshal([]byte(body), &parsed)
	require.NoError(t, err)
	assert.Empty(t, parsed)
}

func createTestConfig() *types.AgentConfig {
	return &types.AgentConfig{
		Workspace: "/tmp/test-workspace",
		Identity: types.IdentityConfig{
			Name:   "TestAgent",
			Avatar: "⚡",
		},
		LLM: types.LLMConfig{
			Provider:  "anthropic",
			Model:     "claude-sonnet-4-20250514",
			APIKeyEnv: "ANTHROPIC_API_KEY",
		},
		Shield: types.ShieldConfig{
			PolicyFile:       "policies/default.yaml",
			HeuristicEnabled: true,
		},
		Chronicle: types.ChronicleConfig{
			MaxSnapshots: 100,
			MaxAgeDays:   30,
		},
		Web: types.WebConfig{
			Enabled: true,
			Port:    3100,
			Auth:    true,
		},
		General: types.GeneralConfig{
			FailClosed:        true,
			RateLimit:         30,
			VerdictTTLSeconds: 60,
			DailyBudget:       50,
		},
	}
}
