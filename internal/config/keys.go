package config

import (
	"fmt"
	"regexp"

	"github.com/openparallax/openparallax/internal/types"
)

// SettableKey describes a config key that can be mutated via the
// /config set or /model slash commands (CLI and web chat channels).
// Each key has an optional validator that runs before the setter, the
// setter that mutates the in-memory AgentConfig, and a flag indicating
// whether the change requires a restart to take effect (model/provider
// changes do; identity changes do not). The web settings panel is
// read-only — there is no HTTP write surface.
type SettableKey struct {
	Path            string
	RequiresRestart bool
	// Validator runs before Setter. It receives the current cfg so
	// validators can make decisions based on adjacent state (e.g. the
	// Ollama base_url check needs to know the model's provider). May
	// be nil for keys without a validator.
	Validator func(cfg *types.AgentConfig, value string) error
	Setter    func(cfg *types.AgentConfig, value string) error
}

// identityNameRe is the allowlist for the agent's display name and avatar.
// Letters, digits, spaces, dash, underscore, length 1-40. The name is
// rendered into the LLM system prompt and the TUI status line, so newlines
// (= prompt injection) and terminal escape sequences must be rejected.
var identityNameRe = regexp.MustCompile(`^[a-zA-Z0-9 _-]{1,40}$`)

func validateIdentityField(field string) func(*types.AgentConfig, string) error {
	return func(_ *types.AgentConfig, value string) error {
		if !identityNameRe.MatchString(value) {
			return fmt.Errorf("%s must match %s", field, identityNameRe.String())
		}
		return nil
	}
}

// validateBaseURLForRole returns a validator that rejects a base_url change
// when the underlying model's provider is ollama and the URL does not point
// at loopback. Ollama needs no api_key_env, so the model-pool validator's
// "needs api key" gate doesn't apply — without this check, /config set
// chat.base_url to attacker.example.com would silently succeed and the next
// chat turn would ship every prompt to the attacker.
func validateBaseURLForRole(roleAccessor func(*types.AgentConfig) string) func(*types.AgentConfig, string) error {
	return func(cfg *types.AgentConfig, value string) error {
		role := roleAccessor(cfg)
		if role == "" {
			return nil
		}
		m, ok := cfg.ModelByName(role)
		if !ok || m.Provider != "ollama" {
			return nil
		}
		return ValidateOllamaBaseURL(value)
	}
}

// SettableKeys enumerates every config key writable through the public
// surface. Adding a new settable key is a one-line change here.
// EngineAdapter.ConfigSet (the /config set slash command dispatcher)
// is the sole consumer.
var SettableKeys = map[string]SettableKey{
	"identity.name": {
		Path:      "identity.name",
		Validator: validateIdentityField("identity.name"),
		Setter: func(cfg *types.AgentConfig, value string) error {
			cfg.Identity.Name = value
			return nil
		},
	},
	"identity.avatar": {
		Path:      "identity.avatar",
		Validator: validateIdentityField("identity.avatar"),
		Setter: func(cfg *types.AgentConfig, value string) error {
			cfg.Identity.Avatar = value
			return nil
		},
	},
	"chat.provider": {
		Path:            "chat.provider",
		RequiresRestart: true,
		Setter: func(cfg *types.AgentConfig, value string) error {
			return mutateRoleModel(cfg, cfg.Roles.Chat, func(m *types.ModelEntry) { m.Provider = value })
		},
	},
	"chat.model": {
		Path:            "chat.model",
		RequiresRestart: true,
		Setter: func(cfg *types.AgentConfig, value string) error {
			return mutateRoleModel(cfg, cfg.Roles.Chat, func(m *types.ModelEntry) { m.Model = value })
		},
	},
	"chat.api_key_env": {
		Path:            "chat.api_key_env",
		RequiresRestart: true,
		Setter: func(cfg *types.AgentConfig, value string) error {
			return mutateRoleModel(cfg, cfg.Roles.Chat, func(m *types.ModelEntry) { m.APIKeyEnv = value })
		},
	},
	"chat.base_url": {
		Path:            "chat.base_url",
		RequiresRestart: true,
		Validator:       validateBaseURLForRole(func(cfg *types.AgentConfig) string { return cfg.Roles.Chat }),
		Setter: func(cfg *types.AgentConfig, value string) error {
			return mutateRoleModel(cfg, cfg.Roles.Chat, func(m *types.ModelEntry) { m.BaseURL = value })
		},
	},
	"shield.provider": {
		Path:            "shield.provider",
		RequiresRestart: true,
		Setter: func(cfg *types.AgentConfig, value string) error {
			return mutateRoleModel(cfg, cfg.Roles.Shield, func(m *types.ModelEntry) { m.Provider = value })
		},
	},
	"shield.model": {
		Path:            "shield.model",
		RequiresRestart: true,
		Setter: func(cfg *types.AgentConfig, value string) error {
			return mutateRoleModel(cfg, cfg.Roles.Shield, func(m *types.ModelEntry) { m.Model = value })
		},
	},
	"shield.api_key_env": {
		Path:            "shield.api_key_env",
		RequiresRestart: true,
		Setter: func(cfg *types.AgentConfig, value string) error {
			return mutateRoleModel(cfg, cfg.Roles.Shield, func(m *types.ModelEntry) { m.APIKeyEnv = value })
		},
	},
	"embedding.provider": {
		Path:            "embedding.provider",
		RequiresRestart: true,
		Setter: func(cfg *types.AgentConfig, value string) error {
			return mutateRoleModel(cfg, cfg.Roles.Embedding, func(m *types.ModelEntry) { m.Provider = value })
		},
	},
	"embedding.model": {
		Path:            "embedding.model",
		RequiresRestart: true,
		Setter: func(cfg *types.AgentConfig, value string) error {
			return mutateRoleModel(cfg, cfg.Roles.Embedding, func(m *types.ModelEntry) { m.Model = value })
		},
	},
	"roles.chat": {
		Path:            "roles.chat",
		RequiresRestart: true,
		Setter: func(cfg *types.AgentConfig, value string) error {
			if _, ok := cfg.ModelByName(value); !ok {
				return fmt.Errorf("model %q is not in the model pool", value)
			}
			cfg.Roles.Chat = value
			return nil
		},
	},
	"roles.shield": {
		Path:            "roles.shield",
		RequiresRestart: true,
		Setter: func(cfg *types.AgentConfig, value string) error {
			if _, ok := cfg.ModelByName(value); !ok {
				return fmt.Errorf("model %q is not in the model pool", value)
			}
			cfg.Roles.Shield = value
			return nil
		},
	},
	"roles.embedding": {
		Path:            "roles.embedding",
		RequiresRestart: true,
		Setter: func(cfg *types.AgentConfig, value string) error {
			if _, ok := cfg.ModelByName(value); !ok {
				return fmt.Errorf("model %q is not in the model pool", value)
			}
			cfg.Roles.Embedding = value
			return nil
		},
	},
	"roles.sub_agent": {
		Path:            "roles.sub_agent",
		RequiresRestart: true,
		Setter: func(cfg *types.AgentConfig, value string) error {
			if _, ok := cfg.ModelByName(value); !ok {
				return fmt.Errorf("model %q is not in the model pool", value)
			}
			cfg.Roles.SubAgent = value
			return nil
		},
	},
}

// mutateRoleModel finds the model in cfg.Models referenced by roleName
// and applies fn to it. Returns an error if the role is not mapped or
// the referenced model does not exist in the pool.
func mutateRoleModel(cfg *types.AgentConfig, roleName string, fn func(*types.ModelEntry)) error {
	if roleName == "" {
		return fmt.Errorf("role is not mapped to any model in the pool")
	}
	for i := range cfg.Models {
		if cfg.Models[i].Name == roleName {
			fn(&cfg.Models[i])
			return nil
		}
	}
	return fmt.Errorf("role %q references model %q which is not in the pool", roleName, roleName)
}
