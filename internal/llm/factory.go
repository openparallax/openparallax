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
