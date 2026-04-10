package types

import (
	"github.com/openparallax/openparallax/llm"
	"github.com/openparallax/openparallax/mcp"
)

// ModelEntry defines a single LLM provider+model in the model pool.
type ModelEntry struct {
	// Name is the unique identifier for this model (e.g., "claude-sonnet", "gemini-flash").
	Name string `yaml:"name" json:"name"`

	// Provider is the LLM provider ("anthropic", "openai", "google", "ollama").
	Provider string `yaml:"provider" json:"provider"`

	// Model is the provider-specific model identifier.
	Model string `yaml:"model" json:"model"`

	// APIKeyEnv is the environment variable holding the API key.
	APIKeyEnv string `yaml:"api_key_env,omitempty" json:"api_key_env,omitempty"`

	// BaseURL overrides the provider's default API endpoint.
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"`

	// Purpose is an optional human-written hint about when to prefer
	// this model. Surfaced to the LLM in the create_agent tool
	// description so it can pick a sub-agent model intentionally.
	// Examples: "fast, cheap, scans and lookups", "stronger reasoning,
	// multi-file context".
	Purpose string `yaml:"purpose,omitempty" json:"purpose,omitempty"`
}

// LLMConfig converts the model entry into the runtime llm.Config DTO
// used by llm.NewProvider and friends.
func (m ModelEntry) LLMConfig() llm.Config {
	return llm.Config{
		Provider:  m.Provider,
		Model:     m.Model,
		APIKeyEnv: m.APIKeyEnv,
		BaseURL:   m.BaseURL,
	}
}

// RolesConfig maps functional roles to model names from the model pool.
type RolesConfig struct {
	// Chat is the model used for the main conversation.
	Chat string `yaml:"chat" json:"chat"`

	// Shield is the model used for Tier 2 Shield evaluation.
	Shield string `yaml:"shield,omitempty" json:"shield,omitempty"`

	// Embedding is the model used for vector embeddings.
	Embedding string `yaml:"embedding,omitempty" json:"embedding,omitempty"`

	// SubAgent is the model used for sub-agent tasks.
	SubAgent string `yaml:"sub_agent,omitempty" json:"sub_agent,omitempty"`

	// Image is the model used for image generation.
	Image string `yaml:"image,omitempty" json:"image,omitempty"`

	// Video is the model used for video generation.
	Video string `yaml:"video,omitempty" json:"video,omitempty"`
}

// AgentConfig is the complete agent configuration loaded from config.yaml.
type AgentConfig struct {
	// Workspace is the root directory for the agent's workspace files.
	Workspace string `yaml:"workspace" json:"workspace"`

	// Models defines the available LLM provider+model pool.
	Models []ModelEntry `yaml:"models" json:"models"`

	// Roles maps functional roles (chat, shield, embedding, sub_agent) to model names.
	Roles RolesConfig `yaml:"roles" json:"roles"`

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

	// Agents configures sub-agent orchestration defaults.
	Agents AgentsConfig `yaml:"agents,omitempty" json:"agents,omitempty"`

	// General holds global settings.
	General GeneralConfig `yaml:"general" json:"general"`

	// MCP configures external MCP server connections.
	MCP MCPConfig `yaml:"mcp,omitempty" json:"mcp,omitempty"`

	// Email configures the email executor.
	Email EmailConfig `yaml:"email,omitempty" json:"email,omitempty"`

	// Calendar configures the calendar executor.
	Calendar CalendarConfig `yaml:"calendar,omitempty" json:"calendar,omitempty"`

	// Tools configures tool group availability.
	Tools ToolsConfig `yaml:"tools,omitempty" json:"tools,omitempty"`

	// Skills configures skill availability.
	Skills SkillsConfig `yaml:"skills,omitempty" json:"skills,omitempty"`

	// OAuth configures OAuth2 providers for email and calendar integrations.
	OAuth OAuthConfig `yaml:"oauth,omitempty" json:"oauth,omitempty"`

	// Security configures the security subsystem paths. The subsystems
	// themselves (Shield, IFC) are non-negotiable — only the policies are
	// tunable. Paths are relative to the workspace.
	Security SecurityConfig `yaml:"security,omitempty" json:"security,omitempty"`
}

// ModelByName returns the named model entry from the pool.
func (c *AgentConfig) ModelByName(name string) (ModelEntry, bool) {
	return c.modelByName(name)
}

// modelByName returns the named model entry from the pool.
func (c *AgentConfig) modelByName(name string) (ModelEntry, bool) {
	if name == "" {
		return ModelEntry{}, false
	}
	for _, m := range c.Models {
		if m.Name == name {
			return m, true
		}
	}
	return ModelEntry{}, false
}

// ChatModel returns the model assigned to the chat role.
// The bool reports whether the role is mapped and the model exists in the pool.
func (c *AgentConfig) ChatModel() (ModelEntry, bool) {
	return c.modelByName(c.Roles.Chat)
}

// ShieldModel returns the model assigned to the Tier 2 Shield evaluator role.
// Falls back to the chat model if no shield role is mapped.
func (c *AgentConfig) ShieldModel() (ModelEntry, bool) {
	if m, ok := c.modelByName(c.Roles.Shield); ok {
		return m, true
	}
	return c.ChatModel()
}

// EmbeddingModel returns the model assigned to the embedding role.
// The bool reports whether the role is mapped and the model exists in the pool.
func (c *AgentConfig) EmbeddingModel() (ModelEntry, bool) {
	return c.modelByName(c.Roles.Embedding)
}

// SubAgentModel returns the model assigned to the sub-agent role.
// Falls back to the chat model if no sub-agent role is mapped.
func (c *AgentConfig) SubAgentModel() (ModelEntry, bool) {
	if m, ok := c.modelByName(c.Roles.SubAgent); ok {
		return m, true
	}
	return c.ChatModel()
}

// ImageModel returns the model assigned to the image generation role.
func (c *AgentConfig) ImageModel() (ModelEntry, bool) {
	return c.modelByName(c.Roles.Image)
}

// VideoModel returns the model assigned to the video generation role.
func (c *AgentConfig) VideoModel() (ModelEntry, bool) {
	return c.modelByName(c.Roles.Video)
}

// OAuthConfig holds OAuth2 client credentials per provider.
type OAuthConfig struct {
	// Google configures Google OAuth2 (Gmail IMAP, Google Calendar).
	Google *OAuthProviderConfig `yaml:"google,omitempty" json:"google,omitempty"`

	// Microsoft configures Microsoft OAuth2 (MS365 IMAP, Graph Calendar).
	Microsoft *OAuthProviderConfig `yaml:"microsoft,omitempty" json:"microsoft,omitempty"`
}

// OAuthProviderConfig holds client credentials for one OAuth2 provider.
type OAuthProviderConfig struct {
	// ClientID is the OAuth2 application client ID.
	ClientID string `yaml:"client_id" json:"client_id"`

	// ClientSecret is the OAuth2 application client secret.
	ClientSecret string `yaml:"client_secret" json:"client_secret"`

	// TenantID is the Azure AD tenant ID (Microsoft only, default "common").
	TenantID string `yaml:"tenant_id,omitempty" json:"tenant_id,omitempty"`
}

// EmailConfig configures email sending and reading.
type EmailConfig struct {
	Provider string     `yaml:"provider,omitempty" json:"provider,omitempty"` // "smtp" for now; "gmail", "outlook" future
	SMTP     SMTPConfig `yaml:"smtp,omitempty" json:"smtp,omitempty"`
	IMAP     IMAPConfig `yaml:"imap,omitempty" json:"imap,omitempty"`
}

// IMAPConfig configures IMAP email reading.
type IMAPConfig struct {
	// Host is the IMAP server hostname (e.g. "imap.gmail.com").
	Host string `yaml:"host" json:"host"`

	// Port is the IMAP server port (typically 993 for TLS).
	Port int `yaml:"port" json:"port"`

	// TLS enables TLS encryption (default true).
	TLS bool `yaml:"tls" json:"tls"`

	// Username is the IMAP login username (for password auth).
	Username string `yaml:"username,omitempty" json:"username,omitempty"`

	// Password is the IMAP login password or app password (for password auth).
	Password string `yaml:"password,omitempty" json:"password,omitempty"`

	// AuthMode is the authentication mode: "password" or "oauth2".
	AuthMode string `yaml:"auth_mode" json:"auth_mode"`

	// Account is the email address used for OAuth2 token lookup.
	Account string `yaml:"account,omitempty" json:"account,omitempty"`
}

// SMTPConfig holds SMTP connection settings.
type SMTPConfig struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
	From     string `yaml:"from" json:"from"`
	TLS      bool   `yaml:"tls" json:"tls"`
}

// CalendarConfig configures calendar access.
type CalendarConfig struct {
	Provider         string `yaml:"provider,omitempty" json:"provider,omitempty"` // "google", "caldav", "microsoft"
	GoogleCredFile   string `yaml:"google_credentials_file,omitempty" json:"google_credentials_file,omitempty"`
	CalendarID       string `yaml:"calendar_id,omitempty" json:"calendar_id,omitempty"`
	CalDAVURL        string `yaml:"caldav_url,omitempty" json:"caldav_url,omitempty"`
	CalDAVUsername   string `yaml:"caldav_username,omitempty" json:"caldav_username,omitempty"`
	CalDAVPassword   string `yaml:"caldav_password,omitempty" json:"caldav_password,omitempty"`
	MicrosoftAccount string `yaml:"microsoft_account,omitempty" json:"microsoft_account,omitempty"`
}

// MCPConfig holds MCP server configurations.
type MCPConfig struct {
	Servers []MCPServerConfig `yaml:"servers,omitempty" json:"servers,omitempty"`
}

// MCPServerConfig is an alias for the public mcp.ServerConfig type.
type MCPServerConfig = mcp.ServerConfig

// ShieldConfig configures the Shield evaluation pipeline.
//
// The Tier 2 evaluator's provider/model are not stored here — they
// come from the Models pool via the role mapping in RolesConfig.Shield.
// Use AgentConfig.ShieldModel() to resolve them.
type ShieldConfig struct {
	// PolicyFile is the path to the YAML policy file.
	PolicyFile string `yaml:"policy_file" json:"policy_file"`

	// OnnxThreshold is the confidence threshold for the ONNX classifier (0.0-1.0).
	OnnxThreshold float64 `yaml:"onnx_threshold" json:"onnx_threshold"`

	// HeuristicEnabled enables the heuristic regex classifier.
	HeuristicEnabled bool `yaml:"heuristic_enabled" json:"heuristic_enabled"`

	// ClassifierEnabled enables the ONNX injection classifier. Default is true.
	// Set to false to run Tier 1 in heuristic-only mode (lower attack
	// detection on encoding/obfuscation/multi-agent categories).
	ClassifierEnabled bool `yaml:"classifier_enabled" json:"classifier_enabled"`

	// ClassifierMode selects how the ONNX classifier runs when enabled:
	// "local" loads the model in-process via the optional get-classifier
	// download; "sidecar" connects to a CGo classifier subprocess via
	// ClassifierAddr. Ignored when ClassifierEnabled is false.
	ClassifierMode string `yaml:"classifier_mode,omitempty" json:"classifier_mode,omitempty"`

	// ClassifierAddr is the address of the ONNX classifier sidecar (e.g., "localhost:8090").
	ClassifierAddr string `yaml:"classifier_addr,omitempty" json:"classifier_addr,omitempty"`

	// ClassifierSkipTypes lists action types where ONNX classification is
	// skipped because the model over-fires on benign payloads. Heuristics
	// and policy rules still run for these types.
	ClassifierSkipTypes []string `yaml:"classifier_skip_types,omitempty" json:"classifier_skip_types,omitempty"`

	// Tier3 configures human-in-the-loop approval for uncertain verdicts.
	Tier3 Tier3Config `yaml:"tier3,omitempty" json:"tier3,omitempty"`
}

// Tier3Config configures the human-in-the-loop approval tier.
type Tier3Config struct {
	// MaxPerHour is the maximum number of Tier 3 prompts per hour (default 10).
	MaxPerHour int `yaml:"max_per_hour,omitempty" json:"max_per_hour,omitempty"`

	// TimeoutSeconds is how long to wait for user response before auto-deny (default 300).
	TimeoutSeconds int `yaml:"timeout_seconds,omitempty" json:"timeout_seconds,omitempty"`
}

// IdentityConfig provides agent identity overrides.
type IdentityConfig struct {
	// Name overrides the agent name from IDENTITY.md.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Avatar is an emoji displayed alongside the agent name.
	Avatar string `yaml:"avatar,omitempty" json:"avatar,omitempty"`
}

// ChannelsConfig configures messaging platform adapters.
type ChannelsConfig struct {
	WhatsApp *WhatsAppConfig `yaml:"whatsapp,omitempty" json:"whatsapp,omitempty"`
	Telegram *TelegramConfig `yaml:"telegram,omitempty" json:"telegram,omitempty"`
	Discord  *DiscordConfig  `yaml:"discord,omitempty" json:"discord,omitempty"`
	Slack    *SlackConfig    `yaml:"slack,omitempty" json:"slack,omitempty"`
	Signal   *SignalConfig   `yaml:"signal,omitempty" json:"signal,omitempty"`
	Teams    *TeamsConfig    `yaml:"teams,omitempty" json:"teams,omitempty"`
	IMessage *IMessageConfig `yaml:"imessage,omitempty" json:"imessage,omitempty"`
}

// IMessageConfig configures the iMessage channel adapter (macOS only).
type IMessageConfig struct {
	// Enabled controls whether the iMessage adapter starts.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// AppleID is the Apple ID email used in Messages.app.
	AppleID string `yaml:"apple_id" json:"apple_id"`
}

// WhatsAppConfig configures the WhatsApp channel adapter.
type WhatsAppConfig struct {
	Enabled        bool     `yaml:"enabled" json:"enabled"`
	PhoneNumberID  string   `yaml:"phone_number_id,omitempty" json:"phone_number_id,omitempty"`
	AccessTokenEnv string   `yaml:"access_token_env,omitempty" json:"access_token_env,omitempty"`
	VerifyToken    string   `yaml:"verify_token,omitempty" json:"verify_token,omitempty"`
	WebhookPort    int      `yaml:"webhook_port,omitempty" json:"webhook_port,omitempty"`
	AllowedNumbers []string `yaml:"allowed_numbers,omitempty" json:"allowed_numbers,omitempty"`
}

// TelegramConfig configures the Telegram channel adapter.
type TelegramConfig struct {
	Enabled         bool    `yaml:"enabled" json:"enabled"`
	TokenEnv        string  `yaml:"token_env" json:"token_env"`
	AllowedUsers    []int64 `yaml:"allowed_users,omitempty" json:"allowed_users,omitempty"`
	AllowedGroups   []int64 `yaml:"allowed_groups,omitempty" json:"allowed_groups,omitempty"`
	PrivateOnly     *bool   `yaml:"private_only,omitempty" json:"private_only,omitempty"`
	PollingInterval int     `yaml:"polling_interval,omitempty" json:"polling_interval,omitempty"`
}

// DiscordConfig configures the Discord channel adapter.
type DiscordConfig struct {
	Enabled           bool     `yaml:"enabled" json:"enabled"`
	TokenEnv          string   `yaml:"token_env" json:"token_env"`
	AllowedGuilds     []string `yaml:"allowed_guilds,omitempty" json:"allowed_guilds,omitempty"`
	AllowedChannels   []string `yaml:"allowed_channels,omitempty" json:"allowed_channels,omitempty"`
	AllowedUsers      []string `yaml:"allowed_users,omitempty" json:"allowed_users,omitempty"`
	RespondToMentions bool     `yaml:"respond_to_mentions" json:"respond_to_mentions"`
}

// SlackConfig configures the Slack channel adapter.
type SlackConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	BotTokenEnv string `yaml:"bot_token_env" json:"bot_token_env"`
	AppTokenEnv string `yaml:"app_token_env" json:"app_token_env"`
}

// SignalConfig configures the Signal channel adapter.
type SignalConfig struct {
	Enabled        bool     `yaml:"enabled" json:"enabled"`
	CLIPath        string   `yaml:"cli_path" json:"cli_path"`
	Account        string   `yaml:"account" json:"account"`
	AllowedNumbers []string `yaml:"allowed_numbers,omitempty" json:"allowed_numbers,omitempty"`
}

// TeamsConfig configures the Microsoft Teams channel adapter.
type TeamsConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	AppIDEnv    string `yaml:"app_id_env" json:"app_id_env"`
	PasswordEnv string `yaml:"password_env" json:"password_env"`
}

// ChronicleConfig is defined as an alias in chronicle.go.

// WebConfig configures the Web UI server.
type WebConfig struct {
	// Enabled controls whether the Web UI is served.
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Host is the bind address. Default "127.0.0.1" (localhost only).
	// Set to "0.0.0.0" for remote access (requires PasswordHash).
	Host string `yaml:"host,omitempty" json:"host,omitempty"`

	// Port is the HTTP listen port.
	Port int `yaml:"port" json:"port"`

	// GRPCPort is the gRPC listen port for CLI-Engine communication.
	// When zero, the engine binds a dynamic port.
	GRPCPort int `yaml:"grpc_port,omitempty" json:"grpc_port,omitempty"`

	// Auth enables cookie-based authentication.
	Auth bool `yaml:"auth" json:"auth"`

	// PasswordHash is the bcrypt hash of the web UI password.
	// Required when Host is non-localhost.
	PasswordHash string `yaml:"password_hash,omitempty" json:"password_hash,omitempty"`

	// AllowedOrigins is a list of origins permitted for CORS and WebSocket connections.
	// When empty, only localhost origins (http://localhost:* and http://127.0.0.1:*) are allowed.
	AllowedOrigins []string `yaml:"allowed_origins,omitempty" json:"allowed_origins,omitempty"`
}

// AgentsConfig configures sub-agent orchestration defaults.
type AgentsConfig struct {
	// MaxRounds limits how many LLM calls each sub-agent can make (default 20).
	MaxRounds int `yaml:"max_rounds,omitempty" json:"max_rounds,omitempty"`

	// MaxToolRounds limits tool-call round-trips per message (default 25).
	MaxToolRounds int `yaml:"max_tool_rounds,omitempty" json:"max_tool_rounds,omitempty"`

	// ContextWindow is the assumed model context window in tokens (default 128000).
	ContextWindow int `yaml:"context_window,omitempty" json:"context_window,omitempty"`

	// CompactionThreshold is the percentage (0-100) of context budget usage
	// that triggers history compaction (default 70).
	CompactionThreshold int `yaml:"compaction_threshold,omitempty" json:"compaction_threshold,omitempty"`

	// MaxResponseTokens is the max tokens per LLM response (default 4096).
	MaxResponseTokens int `yaml:"max_response_tokens,omitempty" json:"max_response_tokens,omitempty"`

	// ShellTimeoutSeconds is the default shell command timeout (default 30).
	ShellTimeoutSeconds int `yaml:"shell_timeout_seconds,omitempty" json:"shell_timeout_seconds,omitempty"`

	// BrowserNavTimeoutSeconds is the browser navigation timeout (default 30).
	BrowserNavTimeoutSeconds int `yaml:"browser_nav_timeout_seconds,omitempty" json:"browser_nav_timeout_seconds,omitempty"`

	// BrowserIdleMinutes is how long the browser session stays alive after
	// the last action before being shut down to free memory (default 5).
	BrowserIdleMinutes int `yaml:"browser_idle_minutes,omitempty" json:"browser_idle_minutes,omitempty"`

	// SubAgentTimeoutSeconds is the max time a sub-agent can run (default 900,
	// i.e. 15 minutes). Individual sub-agents can override this via the
	// timeout parameter on create_agent.
	SubAgentTimeoutSeconds int `yaml:"sub_agent_timeout_seconds,omitempty" json:"sub_agent_timeout_seconds,omitempty"`

	// MaxConcurrentSubAgents is the cap on simultaneously running sub-agent
	// processes. Once reached, create_agent returns an error until one
	// completes (default 10). The cap protects against runaway delegation
	// loops and bounds peak resource use; raise it if you intentionally
	// orchestrate large fan-outs.
	MaxConcurrentSubAgents int `yaml:"max_concurrent_sub_agents,omitempty" json:"max_concurrent_sub_agents,omitempty"`

	// MaxSubAgentRounds is the maximum number of LLM calls each sub-agent
	// can make before stopping (default 20).
	MaxSubAgentRounds int `yaml:"max_sub_agent_rounds,omitempty" json:"max_sub_agent_rounds,omitempty"`

	// CrashRestartBudget is the max agent/engine crashes within
	// CrashWindowSeconds before the process manager gives up (default 5).
	CrashRestartBudget int `yaml:"crash_restart_budget,omitempty" json:"crash_restart_budget,omitempty"`

	// CrashWindowSeconds is the time window for crash counting (default 60).
	CrashWindowSeconds int `yaml:"crash_window_seconds,omitempty" json:"crash_window_seconds,omitempty"`

	// MaxConsecutiveNavFailures is the number of consecutive browser
	// navigation failures before the executor disables navigation for
	// the session (default 3).
	MaxConsecutiveNavFailures int `yaml:"max_consecutive_nav_failures,omitempty" json:"max_consecutive_nav_failures,omitempty"`
}

// ToolsConfig configures tool group availability.
type ToolsConfig struct {
	// DisabledGroups lists tool group names that should not be available to the LLM.
	DisabledGroups []string `yaml:"disabled_groups,omitempty" json:"disabled_groups,omitempty"`
}

// SkillsConfig configures skill availability.
type SkillsConfig struct {
	// Disabled lists skill names that should not be available to the LLM.
	Disabled []string `yaml:"disabled,omitempty" json:"disabled,omitempty"`
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

	// OutputSanitization wraps tool results and memory content in explicit
	// data boundaries before they enter the LLM context. This mitigates
	// prompt injection via web pages, emails, or file contents at the cost
	// of slightly increased token usage per tool result. Disabled by default.
	OutputSanitization bool `yaml:"output_sanitization" json:"output_sanitization"`
}

// SecurityConfig holds paths for security subsystem policy files.
// The subsystems themselves are non-negotiable (controlled by the
// compiled-in security.Sealed type); only the policies are tunable.
type SecurityConfig struct {
	// IFCPolicy is the path (relative to workspace) to the IFC policy YAML.
	// Default: "security/ifc/default.yaml".
	IFCPolicy string `yaml:"ifc_policy,omitempty" json:"ifc_policy,omitempty"`
	// OverrideMode overrides the mode declared in the IFC policy file.
	// Empty string = use the policy's own mode. "audit" or "enforce".
	// NOT exposed via /config set — requires restart.
	OverrideMode string `yaml:"override_mode,omitempty" json:"override_mode,omitempty"`
}
