# Environment Variables

OpenParallax uses two categories of environment variables: **third-party API keys** (standard names, shared with other tools) and **OpenParallax-specific variables** (prefixed with `OP_`).

## Third-Party API Keys

These use standard names so you don't need to duplicate credentials already in your shell profile. OpenParallax reads them but doesn't own the naming convention.

| Variable | Purpose | Required? |
|----------|---------|-----------|
| `ANTHROPIC_API_KEY` | Anthropic Claude API key | If using Anthropic for chat or shield |
| `OPENAI_API_KEY` | OpenAI API key (chat, embeddings) | If using OpenAI for chat, shield, or embeddings |
| `GOOGLE_AI_API_KEY` | Google Gemini API key | If using Google for chat or shield |

These variables are referenced by `config.yaml` through the `api_key_env` field. You can use any variable name — the config just says "read the key from this env var":

```yaml
models:
  - name: chat
    provider: anthropic
    model: claude-sonnet-4-6
    api_key_env: MY_CUSTOM_KEY_VAR  # reads $MY_CUSTOM_KEY_VAR at runtime
```

## OpenParallax Variables (`OP_*`)

All OpenParallax-specific environment variables use the `OP_` prefix to avoid collisions with other tools.

### Core

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `OP_DATA_DIR` | path | `~/.openparallax` | Root data directory. All workspaces, models, and global config live under this path. |
| `OP_WORKSPACE` | path | — | Override the workspace path for the current session. When set, OpenParallax uses this directory instead of resolving from the agent name. |
| `OP_LOG_LEVEL` | string | `info` | Logging level. One of: `debug`, `info`, `warn`, `error`. Affects engine.log verbosity. |
| `OP_NO_COLOR` | bool | `false` | Disable colored output in CLI. Set to `1` or `true`. Respected by both the CLI and the Bubbletea TUI. |

### Web Server

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `OP_WEB_PORT` | int | — | Override the web server port. Takes precedence over `config.yaml`. Useful for running multiple agents or in CI. |
| `OP_WEB_HOST` | string | — | Override the web server bind address. |

### Shield

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `OP_SHIELD_POLICY` | path | — | Override the policy file path. Takes precedence over `config.yaml`. |
| `OP_SHIELD_CLASSIFIER_DIR` | path | `~/.openparallax/models/prompt-injection` | Directory containing the ONNX classifier model, tokenizer, and runtime library. |

### gRPC

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `OP_GRPC_PORT` | int | — | Override the gRPC server port. By default, the engine allocates a dynamic port and communicates it to the process manager via stdout. |

### Development

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `OP_NO_SANDBOX` | bool | `false` | Disable kernel sandboxing. **Development only.** When set, the agent starts without sandbox restrictions. Canary probes are skipped. Never use in production — it removes a critical security layer. |

### Channel Credentials

Channel adapters read credentials from environment variables specified in `config.yaml`:

| Variable (conventional) | Platform | Description |
|------------------------|----------|-------------|
| `WHATSAPP_ACCESS_TOKEN` | WhatsApp | Meta Cloud API access token |
| `TELEGRAM_BOT_TOKEN` | Telegram | Bot token from BotFather |
| `DISCORD_BOT_TOKEN` | Discord | Bot token from Developer Portal |
| `SLACK_BOT_TOKEN` | Slack | Bot OAuth token |
| `SLACK_SIGNING_SECRET` | Slack | Request signing secret |
| `SLACK_APP_TOKEN` | Slack | App-level token for Socket Mode |
| `TEAMS_APP_PASSWORD` | Teams | App password from Azure AD |

These are conventional names — you can use any variable name by changing `api_key_env` or the equivalent field in `config.yaml`.

## Precedence

When a value is specified in multiple places, this precedence applies (highest to lowest):

1. **Environment variable** (`OP_*` override)
2. **config.yaml** (workspace-level)
3. **Built-in default**

For example, if `config.yaml` sets `web.port: 3100` but `OP_WEB_PORT=3200` is set, the server binds to port 3200.

## Shell Profile Setup

Add your API keys to your shell profile for persistence:

```bash
# ~/.bashrc or ~/.zshrc

# LLM API keys
export ANTHROPIC_API_KEY="sk-ant-..."
export OPENAI_API_KEY="sk-..."
export GOOGLE_AI_API_KEY="AI..."

# OpenParallax (optional overrides)
export OP_LOG_LEVEL="info"
export OP_DATA_DIR="$HOME/.openparallax"
```

::: tip Security
API keys are read from environment variables, never stored in `config.yaml`. The config file only stores the *name* of the environment variable (`api_key_env: ANTHROPIC_API_KEY`), not the key itself. This means `config.yaml` is safe to commit or share — it contains no secrets.
:::

## Internal Variables

These are set automatically at runtime and should not be configured manually:

| Variable | Purpose |
|----------|---------|
| `OPENPARALLAX_AGENT_TOKEN` | Ephemeral auth token for Agent → Engine gRPC. Generated per spawn. |
| `OPENPARALLAX_SUB_AGENT_TOKEN` | Ephemeral auth token for Sub-Agent → Engine gRPC. Generated per spawn. |
