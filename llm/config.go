package llm

// Config holds LLM provider configuration.
type Config struct {
	// Provider is the LLM provider name ("anthropic", "openai", "google", "ollama").
	Provider string `yaml:"provider" json:"provider"`
	// Model is the model identifier (e.g., "claude-sonnet-4-20250514").
	Model string `yaml:"model" json:"model"`
	// APIKeyEnv is the environment variable holding the API key.
	APIKeyEnv string `yaml:"api_key_env" json:"api_key_env"`
	// BaseURL is an optional custom API endpoint URL.
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"`
}
