package llm

import (
	"fmt"
	"os"

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
