package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestConfig(t *testing.T, dir string, content string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(content), 0o644)
	require.NoError(t, err)
	return path
}

func TestLoadValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, `
workspace: ./workspace
llm:
  provider: anthropic
  model: claude-sonnet-4-20250514
  api_key_env: ANTHROPIC_API_KEY
shield:
  evaluator:
    provider: openai
    model: gpt-4o
    api_key_env: OPENAI_API_KEY
  policy_file: policies/default.yaml
general:
  fail_closed: true
  rate_limit: 30
`)

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "anthropic", cfg.LLM.Provider)
	assert.Equal(t, "claude-sonnet-4-20250514", cfg.LLM.Model)
	assert.Equal(t, "ANTHROPIC_API_KEY", cfg.LLM.APIKeyEnv)
	assert.Equal(t, "openai", cfg.Shield.Evaluator.Provider)
	assert.True(t, cfg.General.FailClosed)
	assert.Equal(t, 30, cfg.General.RateLimit)

	// Check that workspace path is resolved relative to config dir.
	expected := filepath.Clean(filepath.Join(dir, "workspace"))
	assert.Equal(t, expected, cfg.Workspace)
}

func TestLoadMissingProvider(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, `
workspace: .
llm:
  model: gpt-4o
  api_key_env: OPENAI_API_KEY
`)

	_, err := Load(path)
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrConfigInvalid)
	assert.Contains(t, err.Error(), "llm.provider")
}

func TestLoadMissingModel(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, `
workspace: .
llm:
  provider: anthropic
  api_key_env: ANTHROPIC_API_KEY
`)

	_, err := Load(path)
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrConfigInvalid)
	assert.Contains(t, err.Error(), "llm.model")
}

func TestLoadMissingAPIKeyForNonOllama(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, `
workspace: .
llm:
  provider: openai
  model: gpt-4o
`)

	_, err := Load(path)
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrConfigInvalid)
	assert.Contains(t, err.Error(), "api_key_env")
}

func TestLoadOllamaNoAPIKey(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, `
workspace: .
llm:
  provider: ollama
  model: llama3.2
`)

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "ollama", cfg.LLM.Provider)
}

func TestLoadInvalidProvider(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, `
workspace: .
llm:
  provider: invalid_provider
  model: test
  api_key_env: TEST_KEY
`)

	_, err := Load(path)
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrConfigInvalid)
	assert.Contains(t, err.Error(), "unsupported LLM provider")
}

func TestLoadInvalidOnnxThreshold(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, `
workspace: .
llm:
  provider: ollama
  model: llama3.2
shield:
  onnx_threshold: 1.5
`)

	_, err := Load(path)
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrConfigInvalid)
	assert.Contains(t, err.Error(), "onnx_threshold")
}

func TestLoadInvalidRateLimit(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, `
workspace: .
llm:
  provider: ollama
  model: llama3.2
general:
  rate_limit: 0
`)

	_, err := Load(path)
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrConfigInvalid)
	assert.Contains(t, err.Error(), "rate_limit")
}

func TestResolvePathTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	result := resolvePath("~/workspace", "/ignored")
	assert.Equal(t, filepath.Join(home, "workspace"), result)
}

func TestResolvePathRelative(t *testing.T) {
	result := resolvePath("workspace", "/etc")
	assert.Equal(t, filepath.Clean("/etc/workspace"), result)
}

func TestResolvePathAbsolute(t *testing.T) {
	result := resolvePath("/absolute/path", "/ignored")
	assert.Equal(t, "/absolute/path", result)
}

func TestCrossModelWarning(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, `
workspace: .
llm:
  provider: openai
  model: gpt-4o
  api_key_env: OPENAI_API_KEY
shield:
  evaluator:
    provider: openai
    model: gpt-4o
    api_key_env: OPENAI_API_KEY
`)

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	_, err := Load(path)
	require.NoError(t, err)

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	assert.Contains(t, buf.String(), "Cross-model evaluation is recommended")
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrConfigNotFound)
}

func TestDefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, `
workspace: .
llm:
  provider: ollama
  model: llama3.2
`)

	cfg, err := Load(path)
	require.NoError(t, err)

	// These should have default values
	assert.Equal(t, 0.85, cfg.Shield.OnnxThreshold)
	assert.True(t, cfg.Shield.HeuristicEnabled)
	assert.Equal(t, 100, cfg.Chronicle.MaxSnapshots)
	assert.Equal(t, 30, cfg.Chronicle.MaxAgeDays)
	assert.True(t, cfg.Web.Enabled)
	assert.Equal(t, 3100, cfg.Web.Port)
	assert.True(t, cfg.General.FailClosed)
	assert.Equal(t, 60, cfg.General.VerdictTTLSeconds)
	assert.Equal(t, 100, cfg.General.DailyBudget)
}
