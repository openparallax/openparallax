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
	if cfg.LLM.Provider == "" {
		return fmt.Errorf("%w: llm.provider is required", types.ErrConfigInvalid)
	}
	if cfg.LLM.Model == "" {
		return fmt.Errorf("%w: llm.model is required", types.ErrConfigInvalid)
	}

	if !validProviders[cfg.LLM.Provider] {
		return fmt.Errorf("%w: unsupported LLM provider %q (valid: anthropic, openai, google, ollama)",
			types.ErrConfigInvalid, cfg.LLM.Provider)
	}

	if cfg.LLM.Provider != "ollama" && cfg.LLM.APIKeyEnv == "" {
		return fmt.Errorf("%w: llm.api_key_env is required for provider %q",
			types.ErrConfigInvalid, cfg.LLM.Provider)
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
