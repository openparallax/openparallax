package config

import (
	"fmt"

	"github.com/openparallax/openparallax/internal/types"
)

// validProviders is the set of supported LLM provider names.
var validProviders = map[string]bool{
	"anthropic": true,
	"openai":    true,
	"google":    true,
	"ollama":    true,
}

// validate checks the configuration for required fields and valid values.
func validate(cfg *types.AgentConfig) error {
	if len(cfg.Models) == 0 {
		return fmt.Errorf("%w: models[] is required — define at least one model", types.ErrConfigInvalid)
	}

	if cfg.Roles.Chat == "" {
		return fmt.Errorf("%w: roles.chat is required", types.ErrConfigInvalid)
	}

	// Validate each model entry.
	names := make(map[string]bool, len(cfg.Models))
	for _, m := range cfg.Models {
		if m.Name == "" {
			return fmt.Errorf("%w: each model must have a name", types.ErrConfigInvalid)
		}
		if names[m.Name] {
			return fmt.Errorf("%w: duplicate model name %q", types.ErrConfigInvalid, m.Name)
		}
		names[m.Name] = true

		if !validProviders[m.Provider] {
			return fmt.Errorf("%w: model %q has unsupported provider %q (valid: anthropic, openai, google, ollama)",
				types.ErrConfigInvalid, m.Name, m.Provider)
		}
		if m.Model == "" {
			return fmt.Errorf("%w: model %q is missing the model field", types.ErrConfigInvalid, m.Name)
		}
		if m.Provider != "ollama" && m.APIKeyEnv == "" {
			return fmt.Errorf("%w: model %q requires api_key_env for provider %q",
				types.ErrConfigInvalid, m.Name, m.Provider)
		}
	}

	// Validate role references.
	if !names[cfg.Roles.Chat] {
		return fmt.Errorf("%w: roles.chat references unknown model %q", types.ErrConfigInvalid, cfg.Roles.Chat)
	}
	if cfg.Roles.Shield != "" && !names[cfg.Roles.Shield] {
		return fmt.Errorf("%w: roles.shield references unknown model %q", types.ErrConfigInvalid, cfg.Roles.Shield)
	}
	if cfg.Roles.Embedding != "" && !names[cfg.Roles.Embedding] {
		return fmt.Errorf("%w: roles.embedding references unknown model %q", types.ErrConfigInvalid, cfg.Roles.Embedding)
	}
	if cfg.Roles.SubAgent != "" && !names[cfg.Roles.SubAgent] {
		return fmt.Errorf("%w: roles.sub_agent references unknown model %q", types.ErrConfigInvalid, cfg.Roles.SubAgent)
	}
	if cfg.Roles.Image != "" && !names[cfg.Roles.Image] {
		return fmt.Errorf("%w: roles.image references unknown model %q", types.ErrConfigInvalid, cfg.Roles.Image)
	}
	if cfg.Roles.Video != "" && !names[cfg.Roles.Video] {
		return fmt.Errorf("%w: roles.video references unknown model %q", types.ErrConfigInvalid, cfg.Roles.Video)
	}

	if cfg.Shield.OnnxThreshold < 0 || cfg.Shield.OnnxThreshold > 1 {
		return fmt.Errorf("%w: shield.onnx_threshold must be between 0 and 1",
			types.ErrConfigInvalid)
	}

	if cfg.General.RateLimit < 1 {
		return fmt.Errorf("%w: general.rate_limit must be at least 1",
			types.ErrConfigInvalid)
	}

	return nil
}
