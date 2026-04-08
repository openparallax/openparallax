package config

import (
	"strings"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestSettableKey_IdentityName_Allowlist(t *testing.T) {
	key := SettableKeys["identity.name"]
	cfg := &types.AgentConfig{}

	good := []string{"Atlas", "Bear-2", "Code Helper", "x", strings.Repeat("a", 40)}
	for _, in := range good {
		assert.NoError(t, key.Validator(cfg, in), "should accept: %q", in)
	}

	bad := []string{
		"",
		strings.Repeat("a", 41),
		"hello\nrm -rf /",   // newline = prompt injection
		"hello\x1b[31mred",  // ANSI escape
		"name<script>",      // HTML
		"path/with/slashes", // slash
		"emoji😀",            // unicode out of allowlist
	}
	for _, in := range bad {
		assert.Error(t, key.Validator(cfg, in), "should reject: %q", in)
	}
}

func TestSettableKey_IdentityAvatar_Allowlist(t *testing.T) {
	key := SettableKeys["identity.avatar"]
	cfg := &types.AgentConfig{}
	assert.NoError(t, key.Validator(cfg, "Atlas"))
	assert.Error(t, key.Validator(cfg, "with\nnewline"))
}

func TestSettableKey_ChatBaseURL_OllamaLoopbackOnly(t *testing.T) {
	key := SettableKeys["chat.base_url"]

	cfg := &types.AgentConfig{
		Models: []types.ModelEntry{{Name: "chat", Provider: "ollama", Model: "llama3.2"}},
		Roles:  types.RolesConfig{Chat: "chat"},
	}

	assert.NoError(t, key.Validator(cfg, "http://localhost:11434"))
	assert.NoError(t, key.Validator(cfg, ""))
	assert.Error(t, key.Validator(cfg, "http://attacker.example.com"))
}

func TestSettableKey_ChatBaseURL_NonOllamaUnconstrained(t *testing.T) {
	key := SettableKeys["chat.base_url"]

	cfg := &types.AgentConfig{
		Models: []types.ModelEntry{{Name: "chat", Provider: "openai", Model: "gpt-5", APIKeyEnv: "OPENAI_API_KEY"}},
		Roles:  types.RolesConfig{Chat: "chat"},
	}

	// Non-Ollama providers can point at any URL — auth via api_key_env
	// makes the destination irrelevant for secret-exfil purposes.
	assert.NoError(t, key.Validator(cfg, "https://api.example.com"))
}
