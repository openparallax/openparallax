package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Atlas", "atlas"},
		{"Nova", "nova"},
		{"My Agent", "my-agent"},
		{"  Spaced  Name  ", "spaced-name"},
		{"Agent123", "agent123"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, slugify(tt.input), "slugify(%q)", tt.input)
	}
}

func TestValidateAgentName(t *testing.T) {
	assert.NoError(t, validateAgentName("")) // empty = default
	assert.NoError(t, validateAgentName("Atlas"))
	assert.NoError(t, validateAgentName("My Agent 2"))
	assert.Error(t, validateAgentName("a very long name that exceeds twenty chars"))
	assert.Error(t, validateAgentName("bad!name"))
}

func TestExpandTilde(t *testing.T) {
	result := expandTilde("~/test", "/home/user")
	assert.Equal(t, "/home/user/test", result)

	result = expandTilde("/absolute/path", "/home/user")
	assert.Equal(t, "/absolute/path", result)
}

func TestWriteConfigCreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	info := providerInfo{
		label:       "Anthropic",
		model:       "claude-sonnet-4-20250514",
		shieldModel: "claude-haiku-4-5-20251001",
		apiKeyEnv:   "ANTHROPIC_API_KEY",
	}

	err := writeConfig(configPath, tmpDir, "Nova", "⚡",
		"anthropic", info,
		"openai", "text-embedding-3-small", "OPENAI_API_KEY",
		3100)
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "name: Nova")
	assert.Contains(t, content, "avatar: ⚡")
	assert.Contains(t, content, "provider: anthropic")
	assert.Contains(t, content, "model: claude-sonnet-4-20250514")
	assert.Contains(t, content, "model: claude-haiku-4-5-20251001")
	assert.Contains(t, content, "provider: openai")
	assert.Contains(t, content, "model: text-embedding-3-small")
	assert.Contains(t, content, "port: 3100")
}

func TestWriteConfigDoesNotOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create existing file.
	err := os.WriteFile(configPath, []byte("existing"), 0o644)
	require.NoError(t, err)

	info := providerInfo{model: "test", shieldModel: "test"}
	err = writeConfig(configPath, tmpDir, "Atlas", "",
		"anthropic", info, "", "", "", 3100)
	require.NoError(t, err)

	// File should not be overwritten.
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, "existing", string(data))
}

func TestCopyTemplatesSubstitutesName(t *testing.T) {
	tmpDir := t.TempDir()
	err := copyTemplates(tmpDir, "Nova")
	require.NoError(t, err)

	// IDENTITY.md should have Nova instead of Atlas.
	data, err := os.ReadFile(filepath.Join(tmpDir, "IDENTITY.md"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "Nova")
	assert.NotContains(t, content, "Atlas")
}

func TestFirstMessageTemplate(t *testing.T) {
	cfg := &types.AgentConfig{}
	msg := FirstMessageTemplate("Nova", cfg)

	assert.Contains(t, msg, "I'm Nova")
	assert.Contains(t, msg, "Read, write, and manage files")
	assert.Contains(t, msg, "Run shell commands")
	assert.NotContains(t, msg, "email")
	assert.NotContains(t, msg, "calendar")
}

func TestFirstMessageIncludesEmail(t *testing.T) {
	cfg := &types.AgentConfig{
		Email: types.EmailConfig{
			SMTP: types.SMTPConfig{Host: "smtp.example.com"},
		},
	}
	msg := FirstMessageTemplate("Atlas", cfg)
	assert.Contains(t, msg, "email")
}

func TestDetectAPIKeyFromEnv(t *testing.T) {
	// Save and clear existing.
	saved := os.Getenv("ANTHROPIC_API_KEY")
	t.Setenv("ANTHROPIC_API_KEY", "test-key-123")

	provider, key := detectAPIKey()
	assert.Equal(t, "anthropic", provider)
	assert.Equal(t, "test-key-123", key)

	// Restore.
	if saved != "" {
		t.Setenv("ANTHROPIC_API_KEY", saved)
	}
}

func TestProviderInfoDefaults(t *testing.T) {
	for name, info := range providers {
		assert.NotEmpty(t, info.label, "provider %s missing label", name)
		assert.NotEmpty(t, info.model, "provider %s missing model", name)
		assert.NotEmpty(t, info.shieldModel, "provider %s missing shieldModel", name)
	}
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "he..", truncate("hello world", 4))
	assert.Equal(t, "hello wor..", truncate("hello world foo", 11))
}
