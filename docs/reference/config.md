---
description: Complete config.yaml reference for OpenParallax — all keys, types, defaults, and descriptions for models, roles, Shield, channels, memory, and more.
---

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
| `shield.policy_file` | `string` | `"security/shield/default.yaml"` | Path to the YAML policy file (Tier 0). **Read-only via API** |
| `shield.onnx_threshold` | `float64` | `0.85` | Confidence threshold for the ONNX classifier (0.0-1.0) |
| `shield.heuristic_enabled` | `bool` | `true` | Enable the heuristic regex classifier (Tier 1) |
| `shield.classifier_enabled` | `bool` | `false` | Enable the ML classifier. Requires `classifier_mode: sidecar` and a running sidecar binary. Default is heuristic-only (79 rules). |
| `shield.classifier_mode` | `string` | `"sidecar"` | How the ONNX classifier runs when enabled. `"sidecar"` connects to an external classifier service via `classifier_addr`. This is the only supported mode. |
| `shield.classifier_addr` | `string` | — | Address of the ONNX classifier sidecar when `classifier_mode: sidecar` (e.g. `"localhost:8090"`) |
| `shield.classifier_skip_types` | `[]string` | see below | Action types where ONNX classification is bypassed because the trained model over-fires on benign payloads. Heuristics and policy rules still run. Default: `[write_file, delete_file, move_file, copy_file, send_email, send_message, http_request]`. See [Shield Tier 1 → Per-Action-Type ONNX Skip List](/shield/tier1#per-action-type-onnx-skip-list). |

### Shield Evaluator

The Tier 2 evaluator is configured via `roles.shield`, which maps to a model entry in the `models[]` pool. There is no `shield.evaluator` config block. Use `AgentConfig.ShieldModel()` to resolve the evaluator model programmatically.

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
| `web.host` | `string` | `""` | Bind address. Empty resolves to `127.0.0.1` (localhost only). Set to `"0.0.0.0"` for remote access (requires `password_hash`) |
| `web.port` | `int` | `3100` | HTTP listen port. **Read-only via API** |
| `web.grpc_port` | `int` | `0` (dynamic) | gRPC listen port for CLI-Engine communication |
| `web.auth` | `bool` | `true` | Enable cookie-based authentication |
| `web.password_hash` | `string` | — | Bcrypt hash of the web UI password. Required when host is non-localhost |
| `web.allowed_origins` | `[]string` | — | Origins permitted for CORS and WebSocket. Empty = localhost only |

## Agents (`agents`)

Configures sub-agent orchestration, LLM call limits, tool timeouts, and crash recovery.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `agents.sub_agent_model` | `string` | — | Override the default sub-agent model. Empty = auto-detect cheapest |
| `agents.max_rounds` | `int` | — | Max LLM calls per sub-agent (default 20) |
| `agents.max_tool_rounds` | `int` | `25` | Max tool-call round-trips per message |
| `agents.context_window` | `int` | `128000` | Assumed model context window in tokens |
| `agents.compaction_threshold` | `int` | `70` | Context budget percentage (0-100) that triggers both compaction and the history/current-turn split inside the compactor |
| `agents.max_response_tokens` | `int` | `4096` | Max tokens per LLM response |
| `agents.shell_timeout_seconds` | `int` | `30` | Default shell command timeout. Increase for long builds or large git operations |
| `agents.browser_nav_timeout_seconds` | `int` | `30` | Browser page navigation timeout. Increase for slow pages on poor connections |
| `agents.browser_idle_minutes` | `int` | `5` | Minutes of inactivity before the headless browser session is shut down to free memory |
| `agents.sub_agent_timeout_seconds` | `int` | `900` | Max time a sub-agent can run before being killed (default 15 minutes). Override per-spawn via the `timeout` parameter on `create_agent` |
| `agents.max_concurrent_sub_agents` | `int` | `10` | Cap on simultaneously running sub-agent processes. Once reached, `create_agent` returns an error until one completes. Raise it if you intentionally orchestrate large fan-outs |
| `agents.max_sub_agent_rounds` | `int` | `20` | Maximum number of LLM calls each sub-agent can make before stopping |
| `agents.crash_restart_budget` | `int` | `5` | Max agent (or engine) crashes within `crash_window_seconds` before the process manager stops restarting |
| `agents.crash_window_seconds` | `int` | `60` | Time window for crash counting |
| `agents.max_consecutive_nav_failures` | `int` | `3` | Consecutive browser navigation failures before the executor disables navigation for the session. Prevents wasted LLM round-trips on hosts where the browser fundamentally cannot load pages (e.g. Flatpak sandbox) |

## General (`general`)

Global operational settings.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `general.fail_closed` | `bool` | `true` | All Shield evaluation errors result in BLOCK |
| `general.rate_limit` | `int` | `30` | Maximum actions per minute |
| `general.verdict_ttl_seconds` | `int` | `60` | How long a Shield verdict remains valid |
| `general.daily_budget` | `int` | `100` | Maximum Tier 2 LLM evaluator calls per day. **Read-only via API** |
| `general.output_sanitization` | `bool` | `false` | Wrap tool results in data boundaries to mitigate prompt injection. Increases token usage slightly |

## Security (`security`)

Security subsystem policy paths. The subsystems are non-negotiable; only the policies are tunable.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `security.ifc_policy` | `string` | `"security/ifc/default.yaml"` | Path to the IFC policy YAML file. Three presets ship: `default`, `permissive`, `strict`. See [IFC reference](/security/ifc) |
| `security.override_mode` | `string` | `""` | Overrides the mode declared in the IFC policy. Empty = use policy's own mode. `"audit"` = log but don't block. `"enforce"` = block. **Not in SettableKeys — requires restart** |
| `security.memory_block_levels` | `[]string` | `[]` | Sensitivity levels that block memory writes when session taint reaches them. Overridden by `memory_block_levels` in the IFC policy file if present. Empty = use the IFC policy's setting (or built-in default `[critical, restricted]`). See [IFC reference](/security/ifc#memory_block_levels) |

## Embedding

The embedding provider is configured via `roles.embedding`, which maps to a model entry in the `models[]` pool. There is no `memory` top-level config key. Use `AgentConfig.EmbeddingModel()` to resolve the embedding model programmatically.

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
  policy_file: security/shield/default.yaml
  heuristic_enabled: true

web:
  enabled: true
  port: 3100

general:
  fail_closed: true
  rate_limit: 30
  daily_budget: 100
```
