# Configuration Reference

OpenParallax is configured through a `config.yaml` file in your workspace directory (typically `~/.openparallax/<agent-name>/config.yaml`). This file is created by the `openparallax init` wizard and can be edited manually at any time.

Sources: [`internal/types/config.go`](https://github.com/openparallax/openparallax/blob/main/internal/types/config.go), [`internal/config/defaults.go`](https://github.com/openparallax/openparallax/blob/main/internal/config/defaults.go)

## Top-Level

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `workspace` | `string` | `"."` | Root directory for the agent's workspace files |

## Models (`models`)

The model pool defines all available LLM providers. This is a list of model entries.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `models[].name` | `string` | — | Unique identifier for this model (e.g. `"claude-sonnet"`) |
| `models[].provider` | `string` | — | LLM provider: `"anthropic"`, `"openai"`, `"google"`, `"ollama"` |
| `models[].model` | `string` | — | Provider-specific model identifier |
| `models[].api_key_env` | `string` | — | Environment variable holding the API key |
| `models[].base_url` | `string` | — | Override the provider's default API endpoint |

## Roles (`roles`)

Maps functional roles to model names from the model pool.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `roles.chat` | `string` | — | Model name for the main conversation |
| `roles.shield` | `string` | — | Model name for Tier 2 Shield evaluation |
| `roles.embedding` | `string` | — | Model name for vector embeddings |
| `roles.sub_agent` | `string` | — | Model name for sub-agent tasks |
| `roles.image` | `string` | — | Model name for image generation |
| `roles.video` | `string` | — | Model name for video generation |

## Shield (`shield`)

Configures the 4-tier Shield security pipeline. See [Security](/guide/security) for architecture details.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `shield.policy_file` | `string` | `"policies/default.yaml"` | Path to the YAML policy file (Tier 0). **Read-only via API** |
| `shield.onnx_threshold` | `float64` | `0.85` | Confidence threshold for the ONNX classifier (0.0-1.0) |
| `shield.heuristic_enabled` | `bool` | `true` | Enable the heuristic regex classifier (Tier 1) |
| `shield.classifier_addr` | `string` | — | Address of the ONNX classifier sidecar (e.g. `"localhost:8090"`) |

### Shield Evaluator (`shield.evaluator`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `shield.evaluator.provider` | `string` | — | LLM provider for Tier 2 evaluator |
| `shield.evaluator.model` | `string` | — | Model for Tier 2 evaluator |
| `shield.evaluator.api_key_env` | `string` | — | Environment variable for evaluator API key |
| `shield.evaluator.base_url` | `string` | — | Override evaluator API endpoint |

### Shield Tier 3 (`shield.tier3`)

Human-in-the-loop approval for uncertain Shield verdicts.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `shield.tier3.max_per_hour` | `int` | `10` | Maximum Tier 3 prompts per hour |
| `shield.tier3.timeout_seconds` | `int` | `300` | Seconds to wait for user response before auto-deny |

## Identity (`identity`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `identity.name` | `string` | `"Atlas"` | Agent display name |
| `identity.avatar` | `string` | — | Emoji displayed alongside the agent name |

## Channels (`channels`)

Configures messaging platform adapters. Each channel is optional and enabled independently.

### WhatsApp (`channels.whatsapp`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `channels.whatsapp.enabled` | `bool` | `false` | Enable the WhatsApp adapter |
| `channels.whatsapp.phone_number_id` | `string` | — | WhatsApp Business phone number ID |
| `channels.whatsapp.access_token_env` | `string` | — | Environment variable for the access token |
| `channels.whatsapp.verify_token` | `string` | — | Webhook verification token |
| `channels.whatsapp.webhook_port` | `int` | — | Webhook listen port |
| `channels.whatsapp.allowed_numbers` | `[]string` | — | Allowlist of phone numbers |

### Telegram (`channels.telegram`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `channels.telegram.enabled` | `bool` | `false` | Enable the Telegram adapter |
| `channels.telegram.token_env` | `string` | — | Environment variable for the bot token |
| `channels.telegram.allowed_users` | `[]int64` | — | Allowlist of Telegram user IDs |
| `channels.telegram.allowed_groups` | `[]int64` | — | Allowlist of Telegram group IDs |
| `channels.telegram.private_only` | `*bool` | — | When true, ignore all group messages |
| `channels.telegram.polling_interval` | `int` | — | Polling interval in seconds |

### Discord (`channels.discord`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `channels.discord.enabled` | `bool` | `false` | Enable the Discord adapter |
| `channels.discord.token_env` | `string` | — | Environment variable for the bot token |
| `channels.discord.allowed_guilds` | `[]string` | — | Allowlist of Discord server (guild) IDs |
| `channels.discord.allowed_channels` | `[]string` | — | Allowlist of Discord channel IDs |
| `channels.discord.allowed_users` | `[]string` | — | Allowlist of Discord user IDs |
| `channels.discord.respond_to_mentions` | `bool` | `false` | Respond when the bot is @mentioned |

### Slack (`channels.slack`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `channels.slack.enabled` | `bool` | `false` | Enable the Slack adapter |
| `channels.slack.bot_token_env` | `string` | — | Environment variable for the bot OAuth token |
| `channels.slack.app_token_env` | `string` | — | Environment variable for the app-level token |

### Signal (`channels.signal`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `channels.signal.enabled` | `bool` | `false` | Enable the Signal adapter |
| `channels.signal.cli_path` | `string` | — | Path to the `signal-cli` binary |
| `channels.signal.account` | `string` | — | Signal account phone number |
| `channels.signal.allowed_numbers` | `[]string` | — | Allowlist of phone numbers |

### Teams (`channels.teams`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `channels.teams.enabled` | `bool` | `false` | Enable the Microsoft Teams adapter |
| `channels.teams.app_id_env` | `string` | — | Environment variable for the Teams app ID |
| `channels.teams.password_env` | `string` | — | Environment variable for the Teams app password |

### iMessage (`channels.imessage`)

macOS only.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `channels.imessage.enabled` | `bool` | `false` | Enable the iMessage adapter |
| `channels.imessage.apple_id` | `string` | — | Apple ID email used in Messages.app |

## Chronicle (`chronicle`)

Configures copy-on-write workspace snapshots for state versioning and rollback.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `chronicle.max_snapshots` | `int` | `100` | Maximum number of snapshots to retain |
| `chronicle.max_age_days` | `int` | `30` | Maximum age of snapshots in days |

## Web (`web`)

Configures the Web UI server.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `web.enabled` | `bool` | `true` | Enable the Web UI |
| `web.host` | `string` | `"127.0.0.1"` | Bind address. Set to `"0.0.0.0"` for remote access (requires `password_hash`) |
| `web.port` | `int` | `3100` | HTTP listen port. **Read-only via API** |
| `web.grpc_port` | `int` | `0` (dynamic) | gRPC listen port for CLI-Engine communication |
| `web.auth` | `bool` | `true` | Enable cookie-based authentication |
| `web.password_hash` | `string` | — | Bcrypt hash of the web UI password. Required when host is non-localhost |
| `web.allowed_origins` | `[]string` | — | Origins permitted for CORS and WebSocket. Empty = localhost only |

## Agents (`agents`)

Configures sub-agent orchestration and LLM call limits.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `agents.sub_agent_model` | `string` | — | Override the default sub-agent model. Empty = auto-detect cheapest |
| `agents.max_rounds` | `int` | — | Max LLM calls per sub-agent (default 20) |
| `agents.max_tool_rounds` | `int` | `25` | Max tool-call round-trips per message |
| `agents.context_window` | `int` | `128000` | Assumed model context window in tokens |
| `agents.compaction_threshold` | `int` | `70` | Context budget percentage (0-100) that triggers history compaction |
| `agents.max_response_tokens` | `int` | `4096` | Max tokens per LLM response |

## General (`general`)

Global operational settings.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `general.fail_closed` | `bool` | `true` | All Shield evaluation errors result in BLOCK |
| `general.rate_limit` | `int` | `30` | Maximum actions per minute |
| `general.verdict_ttl_seconds` | `int` | `60` | How long a Shield verdict remains valid |
| `general.daily_budget` | `int` | `100` | Maximum Tier 2 LLM evaluator calls per day. **Read-only via API** |
| `general.output_sanitization` | `bool` | `false` | Wrap tool results in data boundaries to mitigate prompt injection. Increases token usage slightly |

## Memory (`memory`)

### Embedding (`memory.embedding`)

Configures the embedding provider for semantic vector search.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `memory.embedding.provider` | `string` | — | Embedding provider: `"openai"`, `"google"`, `"ollama"` |
| `memory.embedding.model` | `string` | — | Embedding model name |
| `memory.embedding.api_key_env` | `string` | — | Environment variable for the embedding API key |
| `memory.embedding.base_url` | `string` | — | Override the embedding API endpoint |

## MCP (`mcp`)

External Model Context Protocol server connections. Each server runs as a child process and provides additional tools.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `mcp.servers` | `[]MCPServerConfig` | — | List of MCP servers to connect |
| `mcp.servers[].name` | `string` | — | Server display name |
| `mcp.servers[].command` | `string` | — | Command to start the server |
| `mcp.servers[].args` | `[]string` | — | Command arguments |
| `mcp.servers[].env` | `map[string]string` | — | Environment variables for the server process |

## Email (`email`)

Configures email sending (SMTP) and reading (IMAP).

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `email.provider` | `string` | — | Email provider (`"smtp"` for now) |

### SMTP (`email.smtp`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `email.smtp.host` | `string` | — | SMTP server hostname |
| `email.smtp.port` | `int` | — | SMTP server port |
| `email.smtp.username` | `string` | — | SMTP login username |
| `email.smtp.password` | `string` | — | SMTP login password |
| `email.smtp.from` | `string` | — | Sender email address |
| `email.smtp.tls` | `bool` | — | Enable TLS encryption |

### IMAP (`email.imap`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `email.imap.host` | `string` | — | IMAP server hostname (e.g. `"imap.gmail.com"`) |
| `email.imap.port` | `int` | — | IMAP server port (typically 993 for TLS) |
| `email.imap.tls` | `bool` | — | Enable TLS encryption |
| `email.imap.username` | `string` | — | IMAP login username (for password auth) |
| `email.imap.password` | `string` | — | IMAP login password or app password (for password auth) |
| `email.imap.auth_mode` | `string` | — | Authentication mode: `"password"` or `"oauth2"` |
| `email.imap.account` | `string` | — | Email address for OAuth2 token lookup |

## Calendar (`calendar`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `calendar.provider` | `string` | — | Calendar provider: `"google"`, `"caldav"`, `"microsoft"` |
| `calendar.google_credentials_file` | `string` | — | Path to Google OAuth credentials file |
| `calendar.calendar_id` | `string` | — | Google calendar ID |
| `calendar.caldav_url` | `string` | — | CalDAV server URL |
| `calendar.caldav_username` | `string` | — | CalDAV username |
| `calendar.caldav_password` | `string` | — | CalDAV password |
| `calendar.microsoft_account` | `string` | — | Microsoft account email for OAuth2 token lookup |

## OAuth (`oauth`)

OAuth2 client credentials for email and calendar integrations. Tokens are obtained via `openparallax auth <provider>`.

### Google (`oauth.google`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `oauth.google.client_id` | `string` | — | Google OAuth2 application client ID |
| `oauth.google.client_secret` | `string` | — | Google OAuth2 application client secret |

### Microsoft (`oauth.microsoft`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `oauth.microsoft.client_id` | `string` | — | Microsoft OAuth2 application client ID |
| `oauth.microsoft.client_secret` | `string` | — | Microsoft OAuth2 application client secret |
| `oauth.microsoft.tenant_id` | `string` | `"common"` | Azure AD tenant ID |

## Tools (`tools`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `tools.disabled_groups` | `[]string` | — | Tool group names to disable. See [Action Types — Tool Groups](/reference/actions#tool-groups) |

## Skills (`skills`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `skills.disabled` | `[]string` | — | Skill names to disable |

## Read-Only Fields

The following fields cannot be changed through the web UI settings API. Edit `config.yaml` directly and restart:

- `web.port`
- `general.daily_budget` (`shield.tier2_budget` in the API)
- `shield.policy_file`

## Minimal Example

```yaml
workspace: /home/user/.openparallax/atlas

models:
  - name: claude-sonnet
    provider: anthropic
    model: claude-sonnet-4-6
    api_key_env: ANTHROPIC_API_KEY
  - name: claude-haiku
    provider: anthropic
    model: claude-haiku-4-5-20251001
    api_key_env: ANTHROPIC_API_KEY

roles:
  chat: claude-sonnet
  shield: claude-haiku

identity:
  name: Atlas

shield:
  policy_file: policies/default.yaml
  heuristic_enabled: true

web:
  enabled: true
  port: 3100

general:
  fail_closed: true
  rate_limit: 30
  daily_budget: 100
```
