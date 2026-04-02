package llm

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/openparallax/openparallax/internal/types"
)

// NewProvider creates an LLM provider from configuration.
// The provider is lazily initialized — only the configured provider's SDK is used.
func NewProvider(cfg types.LLMConfig) (Provider, error) {
	apiKey := ""
	if cfg.APIKeyEnv != "" {
		apiKey = os.Getenv(cfg.APIKeyEnv)
		if apiKey == "" && cfg.Provider != "ollama" {
			return nil, fmt.Errorf("environment variable %s is not set", cfg.APIKeyEnv)
		}
	}

	switch cfg.Provider {
	case "anthropic":
		return NewAnthropicProvider(apiKey, cfg.Model)
	case "openai":
		return NewOpenAIProvider(apiKey, cfg.Model, cfg.BaseURL)
	case "google":
		return NewGoogleProvider(apiKey, cfg.Model)
	case "ollama":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return NewOllamaProvider(baseURL, cfg.Model)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Provider)
	}
}

// APIHost returns the host:port for the LLM API endpoint. Used by the
// sandbox to whitelist outbound connections.
func APIHost(cfg types.LLMConfig) string {
	switch cfg.Provider {
	case "anthropic":
		return "api.anthropic.com:443"
	case "openai":
		if cfg.BaseURL != "" {
			return hostFromURL(cfg.BaseURL)
		}
		return "api.openai.com:443"
	case "google":
		return "generativelanguage.googleapis.com:443"
	case "ollama":
		if cfg.BaseURL != "" {
			return hostFromURL(cfg.BaseURL)
		}
		return "localhost:11434"
	default:
		return ""
	}
}

func hostFromURL(rawURL string) string {
	// Strip scheme.
	u := rawURL
	for _, prefix := range []string{"https://", "http://"} {
		if len(u) > len(prefix) && u[:len(prefix)] == prefix {
			u = u[len(prefix):]
			break
		}
	}
	// Strip path.
	if idx := indexByte(u, '/'); idx >= 0 {
		u = u[:idx]
	}
	// Add default port if missing.
	if indexByte(u, ':') < 0 {
		if len(rawURL) > 5 && rawURL[:5] == "https" {
			return u + ":443"
		}
		return u + ":80"
	}
	return u
}

func indexByte(s string, b byte) int {
	for i := range s {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// TestConnection creates a provider from config and sends a minimal test
// prompt to verify connectivity. Returns nil on success.
func TestConnection(cfg types.LLMConfig, apiKey string) error {
	var p Provider
	var err error

	switch cfg.Provider {
	case "anthropic":
		p, err = NewAnthropicProvider(apiKey, cfg.Model)
	case "openai":
		p, err = NewOpenAIProvider(apiKey, cfg.Model, cfg.BaseURL)
	case "google":
		p, err = NewGoogleProvider(apiKey, cfg.Model)
	case "ollama":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		p, err = NewOllamaProvider(baseURL, cfg.Model)
	default:
		return fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
	if err != nil {
		return fmt.Errorf("provider init: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err = p.Complete(ctx, "Respond with OK", WithMaxTokens(5))
	if err != nil {
		return fmt.Errorf("connection test: %w", err)
	}
	return nil
}
