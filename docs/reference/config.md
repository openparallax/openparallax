# Configuration Reference

OpenParallax is configured through a `config.yaml` file in your workspace directory (typically `~/.openparallax/<agent-name>/config.yaml`). This file is created by the `openparallax init` wizard and can be edited manually at any time.

Every field is documented below with its type, default value, and behavior.

## Full Schema

```yaml
# ─── LLM Provider (Chat) ───────────────────────────────────────────
llm:
  provider: anthropic          # anthropic | openai | google | ollama
  model: claude-sonnet-4-20250514  # Model identifier (provider-specific)
  api_key_env: ANTHROPIC_API_KEY  # Environment variable holding the API key
  base_url: ""                 # Custom API base URL (optional, for proxies/self-hosted)
  max_tokens: 4096             # Maximum tokens per LLM response
  temperature: 0.7             # Sampling temperature (0.0–1.0)

# ─── Agent Identity ────────────────────────────────────────────────
identity:
  name: Atlas                  # Display name shown in CLI and web UI

# ─── Shield (Security Pipeline) ────────────────────────────────────
shield:
  policy_file: policies/default.yaml  # Path to YAML policy file (relative to workspace)
  classifier_addr: ""          # HTTP address for external ONNX classifier (legacy)
  evaluator:
    provider: anthropic        # LLM provider for Tier 2 evaluation
    model: claude-sonnet-4-20250514  # Model for Tier 2
    api_key_env: ANTHROPIC_API_KEY
    base_url: ""
    daily_budget: 100          # Maximum Tier 2 evaluations per day
    rate_limit: 10             # Maximum Tier 2 evaluations per minute

# ─── Web UI ─────────────────────────────────────────────────────────
web:
  enabled: true                # Enable/disable the web interface
  port: 3100                   # HTTP port for the web server
  host: ""                     # Bind address ("" = localhost only)
  password_hash: ""            # bcrypt hash for remote access authentication

# ─── Memory & Embeddings ───────────────────────────────────────────
memory:
  embedding:
    provider: openai           # Embedding provider (openai | google | ollama)
    model: text-embedding-3-small  # Embedding model
    api_key_env: OPENAI_API_KEY
    base_url: ""
    dimensions: 1536           # Embedding dimensions (model-dependent)

# ─── Chronicle (Workspace Snapshots) ───────────────────────────────
chronicle:
  max_snapshots: 50            # Maximum snapshots to retain before pruning

# ─── Channels ──────────────────────────────────────────────────────
channels:
  whatsapp:
    enabled: false
    phone_number_id: ""
    access_token_env: WHATSAPP_ACCESS_TOKEN
    verify_token: ""
    webhook_path: /webhook/whatsapp

  telegram:
    enabled: false
    bot_token_env: TELEGRAM_BOT_TOKEN
    webhook_path: /webhook/telegram

  discord:
    enabled: false
    bot_token_env: DISCORD_BOT_TOKEN
    guild_id: ""
    channel_id: ""

  slack:
    enabled: false
    bot_token_env: SLACK_BOT_TOKEN
    signing_secret_env: SLACK_SIGNING_SECRET
    app_token_env: SLACK_APP_TOKEN

  signal:
    enabled: false
    phone_number: ""
    signal_cli_path: /usr/local/bin/signal-cli

  teams:
    enabled: false
    app_id: ""
    app_password_env: TEAMS_APP_PASSWORD
    tenant_id: ""

# ─── MCP Servers ────────────────────────────────────────────────────
mcp:
  servers:
    - name: example
      transport: stdio          # stdio | streamable-http
      command: npx              # For stdio transport
      args: ["@example/mcp-server"]
      env:                      # Additional environment variables
        API_KEY: "${EXAMPLE_API_KEY}"

# ─── Agents (Sub-Agents) ───────────────────────────────────────────
agents:
  max_concurrent: 3            # Maximum concurrent sub-agent processes
  timeout: 300                 # Sub-agent timeout in seconds
```

## Section Details

### `llm` — Chat LLM Provider

The primary LLM used for conversation and reasoning.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `provider` | string | `"anthropic"` | LLM provider. One of: `anthropic`, `openai`, `google`, `ollama`. |
| `model` | string | — | Model identifier. Examples: `claude-sonnet-4-20250514`, `gpt-4o`, `gemini-2.0-flash`, `llama3.1`. No default — must be specified during init. |
| `api_key_env` | string | — | Name of the environment variable containing the API key. The config stores the *variable name*, not the key itself. |
| `base_url` | string | `""` | Custom API base URL. Leave empty for official endpoints. Use for proxies, Azure OpenAI, or self-hosted Ollama (`http://localhost:11434`). |
| `max_tokens` | int | `4096` | Maximum tokens the LLM can generate per response. |
| `temperature` | float | `0.7` | Sampling temperature. Lower = more deterministic, higher = more creative. |

**Provider-specific notes:**

- **Anthropic**: Uses the Messages API. Supports Claude model family. API key from `ANTHROPIC_API_KEY`.
- **OpenAI**: Uses the Chat Completions API. Supports GPT and o-series models. API key from `OPENAI_API_KEY`. For Azure, set `base_url` to your Azure endpoint.
- **Google**: Uses the Gemini API. API key from `GOOGLE_AI_API_KEY`.
- **Ollama**: Local inference. Set `base_url` to `http://localhost:11434` (Ollama's default). No API key needed — leave `api_key_env` empty.

### `shield` — Security Pipeline

Controls the 3-tier security evaluation applied to every tool call.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `policy_file` | string | `"policies/default.yaml"` | Path to the YAML policy file, relative to workspace root. This file must exist — the engine refuses to start if it's missing. |
| `classifier_addr` | string | `""` | HTTP address for an external ONNX classifier sidecar (legacy). If empty, the engine tries to load the local ONNX model from `~/.openparallax/models/prompt-injection/`. If no model is found, Shield runs in heuristic-only mode. |

**`shield.evaluator` — Tier 2 LLM Evaluator:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `provider` | string | — | LLM provider for security evaluation. Can differ from the chat provider — a common pattern is using Anthropic for chat and OpenAI for evaluation (diversity reduces correlated failures). |
| `model` | string | — | Model for Tier 2 evaluation. |
| `api_key_env` | string | — | Environment variable for the evaluator API key. |
| `base_url` | string | `""` | Custom base URL for the evaluator provider. |
| `daily_budget` | int | `100` | Maximum Tier 2 evaluations per calendar day. Tier 2 calls an external LLM, so this controls cost. When exhausted, actions that would normally escalate to Tier 2 are blocked instead (fail-closed). |
| `rate_limit` | int | `10` | Maximum Tier 2 evaluations per minute. Prevents burst spending. |

::: warning Startup Validation
The engine validates security-critical files at startup:
- **Policy file**: Must exist at the configured path. Missing = engine refuses to start.
- **Evaluator prompt** (`prompts/evaluator-v1.md`): Must exist if `evaluator.provider` is configured. Missing = engine refuses to start.
- **Skills directory**: Missing or empty = warning logged, but engine starts normally.
:::

### `web` — Web UI

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable the web interface. When disabled, only CLI and channel adapters are available. |
| `port` | int | `3100` | HTTP port for the web server. Each agent gets a unique port from the registry. |
| `host` | string | `""` | Bind address. Empty or `localhost` = local only. Set to `0.0.0.0` for remote access (requires `password_hash`). |
| `password_hash` | string | `""` | bcrypt hash of the access password. Required when `host` is non-localhost. Generate with: `openparallax config set-password`. Authentication uses an HttpOnly, Secure, SameSite=Strict cookie. |

::: danger Remote Access Security
When exposing the web UI to a network (`host: 0.0.0.0`), always set a strong `password_hash`. Without it, anyone on the network can control your agent. The cookie is HttpOnly (no JavaScript access), Secure (HTTPS only in production), and SameSite=Strict (no cross-site requests).
:::

### `memory.embedding` — Embedding Provider

Controls the embedding model used for vector search in semantic memory.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `provider` | string | `"openai"` | Embedding provider. One of: `openai`, `google`, `ollama`. |
| `model` | string | `"text-embedding-3-small"` | Embedding model. Must match the dimensions field. |
| `api_key_env` | string | — | Environment variable for the embedding API key. |
| `base_url` | string | `""` | Custom base URL. |
| `dimensions` | int | `1536` | Embedding vector dimensions. Must match the model's output. Common values: `1536` (OpenAI small), `3072` (OpenAI large), `768` (many open-source models). |

**Common embedding configurations:**

```yaml
# OpenAI text-embedding-3-small (default, good balance)
memory:
  embedding:
    provider: openai
    model: text-embedding-3-small
    api_key_env: OPENAI_API_KEY
    dimensions: 1536

# OpenAI text-embedding-3-large (higher quality, higher cost)
memory:
  embedding:
    provider: openai
    model: text-embedding-3-large
    api_key_env: OPENAI_API_KEY
    dimensions: 3072

# Local embeddings via Ollama (no API cost, runs on your machine)
memory:
  embedding:
    provider: ollama
    model: nomic-embed-text
    base_url: http://localhost:11434
    dimensions: 768
```

### `channels` — Messaging Platforms

Each channel adapter has an `enabled` boolean and platform-specific credentials. Channels are independent — enable as many as you want.

All channel adapters follow the same pattern:
1. Receive messages via webhook or polling
2. Normalize to a common format
3. Call `engine.ProcessMessageForWeb()` with a channel-specific `EventSender`
4. Stream responses back through the platform's API

See [Channels](/channels/) for detailed setup instructions per platform.

### `mcp` — External MCP Servers

Connect to Model Context Protocol servers for additional tool capabilities.

```yaml
mcp:
  servers:
    # Stdio transport — launches a subprocess
    - name: filesystem
      transport: stdio
      command: npx
      args: ["@modelcontextprotocol/server-filesystem", "/home/user"]
      env:
        NODE_ENV: production

    # HTTP transport — connects to a remote server
    - name: custom-api
      transport: streamable-http
      url: https://mcp.example.com
      headers:
        Authorization: "Bearer ${MCP_API_KEY}"
```

MCP tools are discovered at startup and registered alongside built-in tools. Every MCP tool call passes through the Shield pipeline — external tools get the same security evaluation as built-in ones.

## File Paths

Relative paths in `config.yaml` are resolved relative to the workspace root:

```
~/.openparallax/my-agent/
├── config.yaml              ← workspace root
├── policies/
│   └── default.yaml         ← shield.policy_file: "policies/default.yaml"
├── prompts/
│   └── evaluator-v1.md      ← shield evaluator prompt
├── skills/
│   └── deploy/
│       └── SKILL.md
├── IDENTITY.md
├── SOUL.md
├── USER.md
├── MEMORY.md
├── TOOLS.md
├── BOOT.md
├── HEARTBEAT.md
├── AGENTS.md
├── canary.token
├── audit.jsonl
└── openparallax.db
```

## Reloading Configuration

Configuration is read at engine startup. To apply changes:

1. Edit `config.yaml`
2. Restart the agent: `openparallax restart` or `/restart` in the UI
3. The engine re-reads the config on startup

There is no hot-reload — this is intentional. Security-critical configuration (Shield policy, evaluator settings) should not change without a deliberate restart.
