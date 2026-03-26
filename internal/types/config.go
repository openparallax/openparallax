package types

// AgentConfig is the complete agent configuration loaded from config.yaml.
type AgentConfig struct {
	// Workspace is the root directory for the agent's workspace files.
	Workspace string `yaml:"workspace" json:"workspace"`

	// LLM configures the agent's LLM provider.
	LLM LLMConfig `yaml:"llm" json:"llm"`

	// Shield configures the Shield evaluation pipeline.
	Shield ShieldConfig `yaml:"shield" json:"shield"`

	// Identity provides agent identity overrides.
	Identity IdentityConfig `yaml:"identity" json:"identity"`

	// Channels configures messaging platform adapters.
	Channels ChannelsConfig `yaml:"channels" json:"channels"`

	// Chronicle configures state versioning.
	Chronicle ChronicleConfig `yaml:"chronicle" json:"chronicle"`

	// Web configures the Web UI server.
	Web WebConfig `yaml:"web" json:"web"`

	// Agents defines the multi-agent configuration.
	Agents []AgentEntry `yaml:"agents,omitempty" json:"agents,omitempty"`

	// General holds global settings.
	General GeneralConfig `yaml:"general" json:"general"`
}

// LLMConfig configures the agent's LLM provider.
type LLMConfig struct {
	// Provider is the LLM provider name: "anthropic", "openai", "google", or "ollama".
	Provider string `yaml:"provider" json:"provider"`

	// Model is the model identifier (e.g., "claude-sonnet-4-20250514").
	Model string `yaml:"model" json:"model"`

	// APIKeyEnv is the environment variable name containing the API key.
	APIKeyEnv string `yaml:"api_key_env" json:"api_key_env"`

	// BaseURL is an optional custom endpoint URL for OpenAI-compatible providers.
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"`
}

// ShieldConfig configures the Shield evaluation pipeline.
type ShieldConfig struct {
	// Evaluator configures the Tier 2 LLM evaluator.
	Evaluator EvaluatorConfig `yaml:"evaluator" json:"evaluator"`

	// PolicyFile is the path to the YAML policy file.
	PolicyFile string `yaml:"policy_file" json:"policy_file"`

	// OnnxThreshold is the confidence threshold for the ONNX classifier (0.0-1.0).
	OnnxThreshold float64 `yaml:"onnx_threshold" json:"onnx_threshold"`

	// HeuristicEnabled enables the heuristic regex classifier.
	HeuristicEnabled bool `yaml:"heuristic_enabled" json:"heuristic_enabled"`

	// ClassifierAddr is the address of the ONNX classifier sidecar (e.g., "localhost:8090").
	ClassifierAddr string `yaml:"classifier_addr,omitempty" json:"classifier_addr,omitempty"`
}

// EvaluatorConfig configures the Tier 2 LLM evaluator.
type EvaluatorConfig struct {
	// Provider is the evaluator's LLM provider name.
	Provider string `yaml:"provider" json:"provider"`

	// Model is the evaluator's model identifier.
	Model string `yaml:"model" json:"model"`

	// APIKeyEnv is the environment variable name containing the evaluator's API key.
	APIKeyEnv string `yaml:"api_key_env" json:"api_key_env"`

	// BaseURL is an optional custom endpoint URL.
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"`
}

// IdentityConfig provides agent identity overrides.
type IdentityConfig struct {
	// Name overrides the agent name from IDENTITY.md.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
}

// ChannelsConfig configures messaging platform adapters.
type ChannelsConfig struct {
	WhatsApp *WhatsAppConfig `yaml:"whatsapp,omitempty" json:"whatsapp,omitempty"`
	Telegram *TelegramConfig `yaml:"telegram,omitempty" json:"telegram,omitempty"`
	Discord  *DiscordConfig  `yaml:"discord,omitempty" json:"discord,omitempty"`
	Slack    *SlackConfig    `yaml:"slack,omitempty" json:"slack,omitempty"`
	Signal   *SignalConfig   `yaml:"signal,omitempty" json:"signal,omitempty"`
	Teams    *TeamsConfig    `yaml:"teams,omitempty" json:"teams,omitempty"`
}

// WhatsAppConfig configures the WhatsApp channel adapter.
type WhatsAppConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	DataDir string `yaml:"data_dir" json:"data_dir"`
}

// TelegramConfig configures the Telegram channel adapter.
type TelegramConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	TokenEnv string `yaml:"token_env" json:"token_env"`
}

// DiscordConfig configures the Discord channel adapter.
type DiscordConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	TokenEnv string `yaml:"token_env" json:"token_env"`
}

// SlackConfig configures the Slack channel adapter.
type SlackConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	BotTokenEnv string `yaml:"bot_token_env" json:"bot_token_env"`
	AppTokenEnv string `yaml:"app_token_env" json:"app_token_env"`
}

// SignalConfig configures the Signal channel adapter.
type SignalConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	CLIPath string `yaml:"cli_path" json:"cli_path"`
	Account string `yaml:"account" json:"account"`
}

// TeamsConfig configures the Microsoft Teams channel adapter.
type TeamsConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	AppIDEnv    string `yaml:"app_id_env" json:"app_id_env"`
	PasswordEnv string `yaml:"password_env" json:"password_env"`
}

// ChronicleConfig configures state versioning.
type ChronicleConfig struct {
	// MaxSnapshots is the maximum number of snapshots to retain.
	MaxSnapshots int `yaml:"max_snapshots" json:"max_snapshots"`

	// MaxAgeDays is the maximum age in days for retained snapshots.
	MaxAgeDays int `yaml:"max_age_days" json:"max_age_days"`
}

// WebConfig configures the Web UI server.
type WebConfig struct {
	// Enabled controls whether the Web UI is served.
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Port is the HTTP listen port.
	Port int `yaml:"port" json:"port"`

	// Auth enables cookie-based authentication.
	Auth bool `yaml:"auth" json:"auth"`
}

// AgentEntry configures a single agent in a multi-agent setup.
type AgentEntry struct {
	// Name is the agent identifier.
	Name string `yaml:"name" json:"name"`

	// Workspace is the agent's workspace directory.
	Workspace string `yaml:"workspace" json:"workspace"`

	// Channels is the list of channel names assigned to this agent.
	Channels []string `yaml:"channels" json:"channels"`

	// Default marks this agent as the default for unrouted messages.
	Default bool `yaml:"default,omitempty" json:"default,omitempty"`
}

// GeneralConfig holds global settings.
type GeneralConfig struct {
	// FailClosed causes all evaluation errors to result in BLOCK.
	FailClosed bool `yaml:"fail_closed" json:"fail_closed"`

	// RateLimit is the maximum actions per minute.
	RateLimit int `yaml:"rate_limit" json:"rate_limit"`

	// VerdictTTLSeconds is how long a verdict remains valid.
	VerdictTTLSeconds int `yaml:"verdict_ttl_seconds" json:"verdict_ttl_seconds"`

	// DailyBudget is the maximum number of Tier 2 LLM evaluator calls per day.
	DailyBudget int `yaml:"daily_budget" json:"daily_budget"`
}
