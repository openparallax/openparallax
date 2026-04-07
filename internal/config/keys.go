package config

import (
	"fmt"

	"github.com/openparallax/openparallax/internal/types"
)

// SettableKey describes a config key that can be mutated via /config set
// or PUT /api/settings. Each key has a setter that mutates the in-memory
// AgentConfig and a flag indicating whether the change requires a
// restart to take effect (model/provider changes do; identity changes
// do not).
type SettableKey struct {
	Path            string
	RequiresRestart bool
	Setter          func(cfg *types.AgentConfig, value string) error
}

// SettableKeys enumerates every config key writable through the public
// surface. Adding a new settable key is a one-line change here; both
// EngineAdapter.ConfigSet and Server.handlePutSettings dispatch through
// this map so the two stay in lockstep.
var SettableKeys = map[string]SettableKey{
	"identity.name": {
		Path: "identity.name",
		Setter: func(cfg *types.AgentConfig, value string) error {
			cfg.Identity.Name = value
			return nil
		},
	},
	"identity.avatar": {
		Path: "identity.avatar",
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
