package config

import (
	"fmt"
	"net"
	"net/url"

	"github.com/openparallax/openparallax/internal/types"
)

// ValidateOllamaBaseURL rejects an Ollama base_url that does not point at
// loopback. Ollama does not require an api_key_env, so the model-pool
// validator's "needs api key" gate is not enough to stop a hostile config
// from quietly redirecting every chat turn to a remote attacker. The
// constraint is intentionally narrow: Ollama is a local-first inference
// runtime; remote Ollama deployments need a reverse proxy that requires
// authentication, and that proxy belongs in front of a non-ollama provider
// entry. An empty base_url is allowed because the Ollama client falls back
// to its default loopback address.
func ValidateOllamaBaseURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("ollama base_url %q is not a valid URL: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("ollama base_url %q must use http or https scheme", raw)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("ollama base_url %q has no host", raw)
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("ollama base_url %q must point at loopback (localhost, 127.0.0.1, ::1) — remote Ollama servers should be fronted by an authenticated provider", raw)
	}
	return nil
}

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
		if m.Provider == "ollama" {
			if err := ValidateOllamaBaseURL(m.BaseURL); err != nil {
				return fmt.Errorf("%w: model %q: %s", types.ErrConfigInvalid, m.Name, err)
			}
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

	if cfg.Web.Enabled && cfg.Web.Host != "" && cfg.Web.Host != "127.0.0.1" && cfg.Web.Host != "localhost" && cfg.Web.Host != "::1" {
		if cfg.Web.PasswordHash == "" {
			return fmt.Errorf("%w: web.host is set to %q (remote access) but web.password_hash is empty — set a password with 'openparallax auth set-password' or restrict to localhost",
				types.ErrConfigInvalid, cfg.Web.Host)
		}
	}

	return nil
}
