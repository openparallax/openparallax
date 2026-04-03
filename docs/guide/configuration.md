# Configuration

OpenParallax is configured through a `config.yaml` file in your workspace directory. The `openparallax init` wizard generates this file with sensible defaults. You can edit it directly at any time — changes take effect on the next agent restart.

## File Location

The config file lives at `<workspace>/config.yaml`. The default workspace path is `~/.openparallax/<agent-name>/`.

You can specify a custom config path when starting:

```bash
./dist/openparallax start -c /path/to/config.yaml
```

## Full Reference

### workspace

The root directory for all agent data: sessions, memory, policies, skills, and the `.openparallax/` internal directory.

```yaml
workspace: /home/user/.openparallax/atlas
```

Paths can be absolute or relative to the config file's directory. The `~` prefix is expanded to the user's home directory.

---

### identity

Agent identity settings displayed in the UI and used in system prompts.

```yaml
identity:
  name: Atlas          # Agent display name (default: "Atlas")
  avatar: "⬡"         # Emoji avatar shown in UI
```

The name is substituted into workspace template files (SOUL.md, IDENTITY.md, etc.) during `init`.

---

### llm

Primary LLM provider configuration for conversations and tool use.

```yaml
llm:
  provider: anthropic                # anthropic, openai, google, ollama
  model: claude-sonnet-4-20250514    # Model identifier
  api_key_env: ANTHROPIC_API_KEY     # Environment variable containing the API key
  base_url: ""                       # Custom API endpoint (for OpenAI-compatible APIs)
```

| Field | Description | Required |
|-------|-------------|----------|
| `provider` | LLM provider identifier | Yes |
| `model` | Model name as recognized by the provider | Yes |
| `api_key_env` | Name of the environment variable holding the API key | Yes (except Ollama) |
| `base_url` | Custom API base URL. Use this for OpenAI-compatible providers like LM Studio, Together AI, Groq, or vLLM. Leave empty for the official API endpoint. | No |

**Supported providers and default models:**

| Provider | Default Chat Model | Default Shield Model | Key Env Var |
|----------|-------------------|---------------------|-------------|
| `anthropic` | `claude-sonnet-4-20250514` | `claude-haiku-4-5-20251001` | `ANTHROPIC_API_KEY` |
| `openai` | `gpt-4o` | `gpt-4o-mini` | `OPENAI_API_KEY` |
| `google` | `gemini-2.0-flash` | `gemini-2.0-flash` | `GOOGLE_API_KEY` |
| `ollama` | `llama3.2` | `llama3.2` | (none) |

---

### shield

Security pipeline configuration. Shield evaluates every tool call through up to three tiers before execution.

```yaml
shield:
  evaluator:
    provider: anthropic                  # Provider for Tier 2 LLM evaluator
    model: claude-haiku-4-5-20251001     # Model for Tier 2 evaluation
    api_key_env: ANTHROPIC_API_KEY       # API key env var
    base_url: ""                         # Custom endpoint (optional)
  policy_file: policies/default.yaml     # Path to YAML policy file
  heuristic_enabled: true                # Enable Tier 1 heuristic rules
  onnx_threshold: 0.85                   # ONNX classifier confidence threshold
  classifier_addr: ""                    # Address of external classifier service (optional)
```

| Field | Description | Default |
|-------|-------------|---------|
| `evaluator.provider` | LLM provider for Tier 2 security evaluation | (from init) |
| `evaluator.model` | Model for Tier 2. A cheaper/faster model is recommended. Cross-model evaluation (different from chat model) provides stronger security. | (from init) |
| `evaluator.api_key_env` | API key for the evaluator | (from init) |
| `evaluator.base_url` | Custom API endpoint for the evaluator | `""` |
| `policy_file` | Path to the YAML policy file (relative to workspace) | `policies/default.yaml` |
| `heuristic_enabled` | Enable pattern-matching heuristic rules in Tier 1 | `true` |
| `onnx_threshold` | Confidence threshold for the ONNX classifier (0.0-1.0) | `0.85` |
| `classifier_addr` | gRPC address for an external Shield classifier service. Leave empty for in-process classification. | `""` |

Three policy files are included:

- **`default.yaml`** — balanced security. Allows workspace reads, evaluates shell commands and external communication at higher tiers, blocks access to sensitive system paths.
- **`permissive.yaml`** — minimal friction. Approves most operations without evaluation. Only evaluates external communication. Suitable for trusted development environments.
- **`strict.yaml`** — maximum security. Evaluates everything through higher tiers. Blocks destructive operations by default. Suitable for environments handling sensitive data.

See [Security](/guide/security) for full policy reference.

---

### web

Web UI server configuration.

```yaml
web:
  enabled: true         # Enable the web UI server
  port: 3100            # HTTP port
  grpc_port: 13100      # gRPC port (auto-assigned during init)
  auth: true            # Enable authentication
  password_hash: ""     # bcrypt hash of the web UI password
  host: ""              # Bind address (default: all interfaces)
```

| Field | Description | Default |
|-------|-------------|---------|
| `enabled` | Start the HTTP/WebSocket server | `true` |
| `port` | Port for the web UI and REST API | `3100` |
| `grpc_port` | Port for the gRPC server (CLI <-> engine communication) | auto-assigned |
| `auth` | Require authentication for web access | `true` |
| `password_hash` | bcrypt hash of the password. Generate with `openparallax auth set-password`. When auth is enabled and no hash is set, the agent generates a one-time password on first start. | `""` |
| `host` | Bind address. Empty string binds to all interfaces. Set to `127.0.0.1` to restrict to localhost only. | `""` |

---

### channels

Multi-channel messaging configuration. Each channel can be enabled independently. See [Channels](/guide/channels) for detailed setup instructions.

```yaml
channels:
  whatsapp:
    enabled: false
    phone_number_id: ""
    access_token_env: WHATSAPP_ACCESS_TOKEN
    verify_token: ""
    webhook_path: /webhooks/whatsapp

  telegram:
    enabled: false
    bot_token_env: TELEGRAM_BOT_TOKEN
    webhook_url: ""
    allowed_users: []

  discord:
    enabled: false
    bot_token_env: DISCORD_BOT_TOKEN
    application_id: ""
    guild_ids: []

  slack:
    enabled: false
    bot_token_env: SLACK_BOT_TOKEN
    app_token_env: SLACK_APP_TOKEN
    signing_secret_env: SLACK_SIGNING_SECRET

  signal:
    enabled: false
    signal_cli_path: ""
    phone_number: ""
    allowed_numbers: []

  teams:
    enabled: false
    app_id: ""
    app_password_env: TEAMS_APP_PASSWORD
    tenant_id: ""
```

---

### memory

Memory and embedding configuration for semantic search.

```yaml
memory:
  embedding:
    provider: openai                    # openai, google, ollama
    model: text-embedding-3-small       # Embedding model
    api_key_env: OPENAI_API_KEY         # API key env var
    base_url: ""                        # Custom endpoint (optional)
```

| Field | Description | Default |
|-------|-------------|---------|
| `embedding.provider` | Provider for text embeddings | (from init) |
| `embedding.model` | Embedding model identifier | `text-embedding-3-small` |
| `embedding.api_key_env` | API key for embedding requests | `OPENAI_API_KEY` |
| `embedding.base_url` | Custom API endpoint | `""` |

**Default embedding models by provider:**

| Provider | Model |
|----------|-------|
| `openai` | `text-embedding-3-small` |
| `google` | `text-embedding-004` |
| `ollama` | `nomic-embed-text` |

If no embedding provider is configured, memory search falls back to FTS5 keyword search only. Semantic (vector) search requires an embedding provider.

See [Memory](/guide/memory) for details on how memory works.

---

### mcp

Model Context Protocol server integration. MCP servers provide additional tools from external services.

```yaml
mcp:
  servers:
    - name: github
      command: npx
      args: ["-y", "@modelcontextprotocol/server-github"]
      env:
        GITHUB_TOKEN: "${GITHUB_TOKEN}"

    - name: filesystem
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/directory"]
```

| Field | Description |
|-------|-------------|
| `name` | Identifier for the MCP server |
| `command` | Executable to run |
| `args` | Command-line arguments |
| `env` | Environment variables passed to the server. Use `${VAR}` syntax to reference shell environment variables. |

MCP tools are registered alongside built-in tools and go through the same Shield evaluation pipeline.

---

### chronicle

Copy-on-write workspace snapshot configuration.

```yaml
chronicle:
  max_snapshots: 100    # Maximum number of snapshots to retain
  max_age_days: 30      # Delete snapshots older than this
```

| Field | Description | Default |
|-------|-------------|---------|
| `max_snapshots` | Maximum number of snapshots kept. Oldest snapshots are pruned when the limit is reached. | `100` |
| `max_age_days` | Snapshots older than this are automatically deleted. | `30` |

Chronicle takes a snapshot before every file-modifying tool call, enabling rollback if something goes wrong.

---

### email

Email configuration for sending and reading mail.

```yaml
email:
  smtp:
    host: smtp.gmail.com
    port: 587
    username_env: EMAIL_USERNAME
    password_env: EMAIL_PASSWORD
    from: user@example.com
  imap:
    host: imap.gmail.com
    port: 993
    username_env: EMAIL_USERNAME
    password_env: EMAIL_PASSWORD
```

---

### calendar

Calendar integration for reading and managing events.

```yaml
calendar:
  provider: google          # google or caldav
  credentials_env: ""       # Path to OAuth credentials (Google)
  caldav_url: ""            # CalDAV server URL
  caldav_username_env: ""   # CalDAV username
  caldav_password_env: ""   # CalDAV password
```

---

### general

Global operational settings.

```yaml
general:
  fail_closed: true            # Block on any Shield error (never set to false)
  rate_limit: 30               # Max tool calls per minute
  verdict_ttl_seconds: 60      # Cache Shield verdicts for this duration
  daily_budget: 100            # Maximum Tier 2 evaluations per day
```

| Field | Description | Default |
|-------|-------------|---------|
| `fail_closed` | When true, any Shield pipeline error results in BLOCK. This is a critical security setting. | `true` |
| `rate_limit` | Maximum tool call executions per minute across all sessions | `30` |
| `verdict_ttl_seconds` | How long to cache Shield verdicts for identical actions | `60` |
| `daily_budget` | Maximum number of Tier 2 (LLM evaluator) calls per day. Prevents runaway evaluation costs. | `100` |

---

## Environment Variables

OpenParallax uses environment variables for API keys and sensitive credentials. The convention:

- **Third-party API keys** keep their standard names: `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GOOGLE_AI_API_KEY`
- **OpenParallax-specific variables** use the `OP_` prefix: `OP_WEB_PASSWORD`, `OP_LOG_LEVEL`
- **Channel credentials** use their platform names: `WHATSAPP_ACCESS_TOKEN`, `TELEGRAM_BOT_TOKEN`, `DISCORD_BOT_TOKEN`, etc.

The `api_key_env` fields in config.yaml contain the **name** of the environment variable, not the key itself. This means your API keys live in your shell profile, not in the config file.

## Editing Config at Runtime

Changes to `config.yaml` require an agent restart to take effect:

```bash
./dist/openparallax restart
```

Or from a conversation:

```
/restart
```

## Example: Minimal Config

```yaml
workspace: /home/user/.openparallax/atlas

identity:
  name: Atlas

llm:
  provider: anthropic
  model: claude-sonnet-4-20250514
  api_key_env: ANTHROPIC_API_KEY

shield:
  evaluator:
    provider: anthropic
    model: claude-haiku-4-5-20251001
    api_key_env: ANTHROPIC_API_KEY
  policy_file: policies/default.yaml

web:
  enabled: true
  port: 3100
```

## Example: Full Config with Channels

```yaml
workspace: /home/user/.openparallax/atlas

identity:
  name: Atlas
  avatar: "⬡"

llm:
  provider: anthropic
  model: claude-sonnet-4-20250514
  api_key_env: ANTHROPIC_API_KEY

shield:
  evaluator:
    provider: openai
    model: gpt-4o-mini
    api_key_env: OPENAI_API_KEY
  policy_file: policies/default.yaml
  heuristic_enabled: true

memory:
  embedding:
    provider: openai
    model: text-embedding-3-small
    api_key_env: OPENAI_API_KEY

chronicle:
  max_snapshots: 100
  max_age_days: 30

web:
  enabled: true
  port: 3100
  auth: true

channels:
  telegram:
    enabled: true
    bot_token_env: TELEGRAM_BOT_TOKEN
    allowed_users: ["123456789"]

  discord:
    enabled: true
    bot_token_env: DISCORD_BOT_TOKEN
    application_id: "1234567890"

mcp:
  servers:
    - name: github
      command: npx
      args: ["-y", "@modelcontextprotocol/server-github"]
      env:
        GITHUB_TOKEN: "${GITHUB_TOKEN}"

general:
  fail_closed: true
  rate_limit: 30
  daily_budget: 100
```

## Next Steps

- [CLI Commands](/guide/cli) — start, init, doctor, and all other commands
- [Security](/guide/security) — Shield policies in detail
- [Channels](/guide/channels) — per-channel setup guides
