---
description: Configure OpenParallax via config.yaml — LLM providers, Shield settings, channels, memory backends, and web UI options explained with examples.
---

# Configuration

OpenParallax is configured through a `config.yaml` file in your workspace directory. The `openparallax init` wizard generates this file with sensible defaults. You can edit it directly at any time — changes take effect on the next agent restart.

## File Location

The config file lives at `<workspace>/config.yaml`. The default workspace path is `~/.openparallax/<agent-name>/`.

You can specify a custom config path when starting:

```bash
openparallax start -c /path/to/config.yaml
```

## Full Reference

### workspace

The root directory for all agent data: sessions, memory, security policies, skills, and the `.openparallax/` internal directory.

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

### models and roles

Every LLM the agent can use is declared once in the `models` pool, and
each functional role (`chat`, `shield`, `embedding`, `sub_agent`) maps
to a model name from that pool. The legacy `llm:` top-level key is no
longer accepted — any config containing it will fail to load.

```yaml
models:
  - name: chat
    provider: anthropic
    model: claude-sonnet-4-6
    api_key_env: ANTHROPIC_API_KEY
    purpose: balanced reasoning, multi-file context
  - name: shield
    provider: openai
    model: gpt-5.4-mini
    api_key_env: OPENAI_API_KEY
    purpose: fast, cheap, scans and lookups
  - name: embedding
    provider: openai
    model: text-embedding-3-small
    api_key_env: OPENAI_API_KEY

roles:
  chat: chat
  shield: shield
  embedding: embedding
  sub_agent: chat
```

| Field | Description | Required |
|-------|-------------|----------|
| `models[].name` | Unique identifier for this model in the pool | Yes |
| `models[].provider` | LLM provider identifier | Yes |
| `models[].model` | Model name as recognized by the provider | Yes |
| `models[].api_key_env` | Environment variable holding the API key | Yes (except Ollama) |
| `models[].base_url` | Custom API base URL for OpenAI-compatible providers | No |
| `models[].purpose` | Optional hint surfaced to the LLM in the `create_agent` tool description so it can pick a sub-agent model intentionally. Free-form text. Entries without a `purpose` are still selectable; the LLM judges from the model name. | No |
| `roles.chat` | Model name to use for the main conversation | Yes |
| `roles.shield` | Model name to use for Tier 2 Shield evaluation | No (defaults to chat) |
| `roles.embedding` | Model name to use for vector embeddings | No |
| `roles.sub_agent` | Model name to use for sub-agent tasks | No (defaults to chat) |

**Supported providers and default models:**

| Provider | Default Chat Model | Default Shield Model | Key Env Var |
|----------|-------------------|---------------------|-------------|
| `anthropic` | `claude-sonnet-4-6` | `claude-haiku-4-5-20251001` | `ANTHROPIC_API_KEY` |
| `openai` | `gpt-5.4` | `gpt-5.4-mini` | `OPENAI_API_KEY` |
| `google` | `gemini-3.1-pro` | `gemini-3.1-flash-lite` | `GOOGLE_API_KEY` |
| `ollama` | `llama3.2` | `llama3.2` | (none) |

---

### shield

Security pipeline configuration. Shield evaluates every tool call through up to three tiers before execution.

```yaml
shield:
  policy_file: security/shield/default.yaml     # Path to YAML policy file
  heuristic_enabled: true                # Enable Tier 1 heuristic rules
  onnx_threshold: 0.85                   # ONNX classifier confidence threshold
  classifier_enabled: false              # ML classifier via sidecar (disabled by default)
  classifier_mode: sidecar               # "sidecar" (external classifier service)
  classifier_addr: ""                    # Address of classifier sidecar (optional)
```

The Tier 2 evaluator's provider and model are configured via `roles.shield`, which maps to an entry in the `models[]` pool. There is no `shield.evaluator` config block.

| Field | Description | Default |
|-------|-------------|---------|
| `policy_file` | Path to the YAML policy file (relative to workspace) | `security/shield/default.yaml` |
| `heuristic_enabled` | Enable pattern-matching heuristic rules in Tier 1 | `true` |
| `onnx_threshold` | Confidence threshold for the ONNX classifier (0.0-1.0) | `0.85` |
| `classifier_enabled` | Enable the ML classifier (requires sidecar) | `false` |
| `classifier_mode` | How the classifier runs: `"sidecar"` connects to an external classifier service via `classifier_addr`. Classifier is disabled by default. | `"sidecar"` |
| `classifier_addr` | Address of the classifier sidecar when `classifier_mode: sidecar` | `""` |

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
  allowed_origins: []   # CORS/WebSocket origins (empty = localhost only)
```

| Field | Description | Default |
|-------|-------------|---------|
| `enabled` | Start the HTTP/WebSocket server | `true` |
| `port` | Port for the web UI and REST API | `3100` |
| `grpc_port` | Port for the gRPC server (CLI <-> engine communication) | auto-assigned |
| `auth` | Require authentication for web access | `true` |
| `password_hash` | bcrypt hash of the password. Generate with `openparallax auth set-password`. When auth is enabled and no hash is set, the agent generates a one-time password on first start. | `""` |
| `host` | Bind address. Empty string resolves to `127.0.0.1` (localhost only). Set to `"0.0.0.0"` for remote access. | `""` |
| `allowed_origins` | List of origins permitted for CORS and WebSocket connections. When empty, only localhost origins are accepted. | `[]` |

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
    webhook_port: 8443

  telegram:
    enabled: false
    token_env: TELEGRAM_BOT_TOKEN
    allowed_users: []
    allowed_groups: []
    private_only: true

  discord:
    enabled: false
    token_env: DISCORD_BOT_TOKEN
    allowed_guilds: []

  slack:
    enabled: false
    bot_token_env: SLACK_BOT_TOKEN
    app_token_env: SLACK_APP_TOKEN

  signal:
    enabled: false
    cli_path: ""
    account: ""
    allowed_numbers: []

  teams:
    enabled: false
    app_id_env: TEAMS_APP_ID
    password_env: TEAMS_APP_PASSWORD
```

---

### Embedding

The embedding provider for semantic vector search is configured via `roles.embedding`, which maps to an entry in the `models[]` pool. There is no separate `memory` top-level config key.

```yaml
models:
  - name: embed
    provider: openai
    model: text-embedding-3-small
    api_key_env: OPENAI_API_KEY

roles:
  embedding: embed
```

**Default embedding models by provider:**

| Provider | Model |
|----------|-------|
| `openai` | `text-embedding-3-small` |
| `google` | `text-embedding-004` |
| `ollama` | `nomic-embed-text` |

If no embedding role is configured, memory search falls back to FTS5 keyword search only. Semantic (vector) search requires an embedding provider.

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
    username: "user@example.com"
    password: "app-password"
    from: user@example.com
  imap:
    host: imap.gmail.com
    port: 993
    username: "user@example.com"
    password: "app-password"
```

---

### calendar

Calendar integration for reading and managing events.

```yaml
calendar:
  provider: google                  # google, caldav, or microsoft
  google_credentials_file: ""       # Path to OAuth credentials file (Google)
  caldav_url: ""                    # CalDAV server URL
  caldav_username: ""               # CalDAV username
  caldav_password: ""               # CalDAV password
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
  output_sanitization: false   # Wrap tool results in data boundaries
```

| Field | Description | Default |
|-------|-------------|---------|
| `fail_closed` | When true, any Shield pipeline error results in BLOCK. This is a critical security setting. | `true` |
| `rate_limit` | Maximum tool call executions per minute across all sessions | `30` |
| `verdict_ttl_seconds` | How long to cache Shield verdicts for identical actions | `60` |
| `daily_budget` | Maximum number of Tier 2 (LLM evaluator) calls per day. Prevents runaway evaluation costs. | `100` |
| `output_sanitization` | Wrap tool results and memory content in explicit data boundaries to defend against prompt injection from untrusted content. | `false` |

---

### agents

Sub-agent orchestration, LLM limits, tool timeouts, and crash recovery.

```yaml
agents:
  sub_agent_model: ""                    # Override model for sub-agents (empty = auto-detect)
  max_rounds: 20                         # Max LLM calls per sub-agent
  max_tool_rounds: 25                    # Max tool-call round-trips per message
  context_window: 128000                 # Assumed model context window in tokens
  compaction_threshold: 70               # Compact history at this % of context budget
  max_response_tokens: 4096              # Max tokens per LLM response
  shell_timeout_seconds: 30              # Shell command timeout
  browser_nav_timeout_seconds: 30        # Browser page navigation timeout
  browser_idle_minutes: 5                # Idle minutes before browser shutdown
  sub_agent_timeout_seconds: 900         # Max sub-agent runtime (15 min)
  max_concurrent_sub_agents: 10          # Cap on simultaneous sub-agents
  crash_restart_budget: 5                # Max crashes before giving up
  crash_window_seconds: 60               # Time window for crash counting
  max_consecutive_nav_failures: 3        # Nav failures before disabling browser
```

| Field | Description | Default |
|-------|-------------|---------|
| `sub_agent_model` | Model override for sub-agents. Empty means auto-detect the cheapest model from the configured provider. | `""` |
| `max_rounds` | Maximum number of LLM calls each sub-agent can make before stopping. | `20` |
| `max_tool_rounds` | Maximum tool-call round-trips per message in the main agent loop. | `25` |
| `context_window` | Assumed model context window size in tokens. Used for compaction calculations. | `128000` |
| `compaction_threshold` | Percentage (0-100) of context budget usage that triggers both history compaction and the history/current-turn split inside the compactor. | `70` |
| `max_response_tokens` | Maximum tokens the LLM may produce per response. | `4096` |
| `shell_timeout_seconds` | Default timeout for shell commands. Increase for long builds or large git operations. | `30` |
| `browser_nav_timeout_seconds` | Browser page navigation timeout. Increase for slow pages on poor connections. | `30` |
| `browser_idle_minutes` | Minutes of inactivity before the headless browser session shuts down to free memory. | `5` |
| `sub_agent_timeout_seconds` | Max time a sub-agent can run (default 15 minutes). Individual sub-agents can override this via the `timeout` parameter on `create_agent`. | `900` |
| `max_concurrent_sub_agents` | Cap on simultaneously running sub-agent processes. Once reached, `create_agent` returns an error until one completes. Raise it if you intentionally orchestrate large fan-outs. | `10` |
| `max_sub_agent_rounds` | Maximum number of LLM calls each sub-agent can make before stopping. | `20` |
| `crash_restart_budget` | Max agent or engine crashes within `crash_window_seconds` before the process manager stops restarting. | `5` |
| `crash_window_seconds` | Time window for crash counting. | `60` |
| `max_consecutive_nav_failures` | Consecutive browser navigation failures before the executor disables navigation for the session. Prevents wasted LLM round-trips on hosts where the browser cannot load pages. | `3` |

---

### security

Security subsystem policy paths. The subsystems themselves (Shield, IFC) are non-negotiable — only the policies are tunable. See [Security Architecture](/security/) for the full defense map.

```yaml
security:
  ifc_policy: security/ifc/default.yaml      # Path to IFC policy file
  override_mode: ""                           # "", "audit", or "enforce"
  memory_block_levels:                        # Sensitivity levels that block memory writes
    - critical
    - restricted
```

| Field | Description | Default |
|-------|-------------|---------|
| `ifc_policy` | Path to the IFC policy YAML file (relative to workspace). Three presets ship: `default`, `permissive`, `strict`. | `security/ifc/default.yaml` |
| `override_mode` | Overrides the mode declared in the IFC policy file. Empty = use the policy's own mode. `audit` logs violations but doesn't block. `enforce` blocks violations. **Not settable via `/config set` — requires restart.** | `""` |
| `memory_block_levels` | Sensitivity levels that block `memory_write` when the session has seen data at that level. Overridden by `memory_block_levels` in the IFC policy file if present. Empty = use the policy's setting or the built-in default `[critical, restricted]`. See [IFC reference](/security/ifc#memory_block_levels). | `[]` |

---

### tools

Tool group availability configuration.

```yaml
tools:
  disabled_groups:
    - browser
    - email
```

| Field | Description | Default |
|-------|-------------|---------|
| `disabled_groups` | List of tool group names to hide from the LLM. Disabled groups are not registered as available tools. Valid group names: `file`, `shell`, `git`, `browser`, `email`, `calendar`, `canvas`, `memory`, `http`, `schedule`. | `[]` |

---

### skills

Skill availability configuration.

```yaml
skills:
  disabled:
    - example-skill
```

| Field | Description | Default |
|-------|-------------|---------|
| `disabled` | List of skill names to exclude. Disabled skills are not discoverable by the LLM and cannot be loaded. | `[]` |

---

## Environment Variables

OpenParallax uses environment variables for API keys and sensitive credentials. The convention:

- **Third-party API keys** keep their standard names: `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GOOGLE_AI_API_KEY`
- **OpenParallax-specific variables** use the `OP_` prefix: `OP_WEB_PASSWORD`, `OP_LOG_LEVEL`
- **Channel credentials** use their platform names: `WHATSAPP_ACCESS_TOKEN`, `TELEGRAM_BOT_TOKEN`, `DISCORD_BOT_TOKEN`, etc.

The `api_key_env` fields in config.yaml contain the **name** of the environment variable, not the key itself. This means your API keys live in your shell profile, not in the config file.

## Editing Config at Runtime

There are three supported ways to mutate `config.yaml`. All three go through a single canonical writer (`config.Save`) that:

1. Marshals the config via `yaml.Marshal`.
2. Writes atomically (`<path>.tmp` + rename).
3. Re-loads the file through `Load()` to verify the round-trip succeeds.
4. Backs up the previous file to `<workspace>/.openparallax/backups/config-<timestamp>.yaml`. The backup directory rotates to the 100 most recent.
5. Emits a `ConfigChanged` audit entry containing the source (`slash-config`, `slash-model`, etc.), the changed key list, and the SHA-256 of the previous and new file contents — so the diff is cryptographically attested even if a backup file rotates out of the window.
6. Rolls back on any failure (the on-disk file is left untouched).

Identity values (`identity.name`, `identity.avatar`) are validated against `^[a-zA-Z0-9 _-]{1,40}$` before they are written: newlines and ANSI escapes are rejected, since both fields are rendered into the LLM system prompt and the TUI status line. The `chat.base_url` setting is constrained to loopback addresses when the chat role's model uses the `ollama` provider — Ollama is local-first and does not require an `api_key_env`, so an unconstrained base URL would be a secret-exfiltration vector.

If the writer drifts from the loader, `openparallax doctor` catches it on the next run via the **Config writer** check, before your next restart turns it into a startup failure.

### From the CLI / web UI: `/config set`

```
/config set chat.model claude-haiku-4-5-20251001
/config set identity.name Nova
```

Identity changes take effect immediately. Model and provider changes write to disk and require a restart to take effect on the live engine.

**Settable keys:**

| Key | Effect |
|---|---|
| `identity.name` | Immediate |
| `identity.avatar` | Immediate |
| `chat.provider` | Restart |
| `chat.model` | Restart |
| `chat.api_key_env` | Restart |
| `chat.base_url` | Restart |
| `shield.provider` | Restart |
| `shield.model` | Restart |
| `shield.api_key_env` | Restart |
| `embedding.provider` | Restart |
| `embedding.model` | Restart |
| `roles.chat` | Restart |
| `roles.shield` | Restart |
| `roles.embedding` | Restart |
| `roles.sub_agent` | Restart |

`chat.*`, `shield.*`, and `embedding.*` mutate the model in `models[]` referenced by the corresponding role (`roles.chat`, `roles.shield`, `roles.embedding`). `roles.*` swaps which named model the role points at — the target must already exist in the pool.

**Read-only keys** (must be edited in `config.yaml` directly): `general.fail_closed`, `general.rate_limit`, `general.daily_budget`, `general.verdict_ttl_seconds`, `general.output_sanitization`, `agents.max_tool_rounds`, `agents.context_window`, `agents.compaction_threshold`, `agents.max_response_tokens`, `shield.policy_file`, `shield.onnx_threshold`, `shield.heuristic_enabled`, `web.host`, `web.port`, `web.password_hash`. These are gated to filesystem-level access on purpose — they constrain the trust boundary and shouldn't be settable from a web request or chat message.

### From the CLI / web UI: `/model`

```
/model chat sonnet-pool-entry
/model shield haiku-pool-entry
```

Swaps which named model from your `models[]` pool a role points at. The target name must already exist in the pool. Persisted to disk via the same writer; restart required to bind the new model on the live engine.

### The web UI settings panel is read-only

The settings panel in the web UI displays the current configuration as labels and values — no editors, no Save button. There is no `PUT /api/settings` HTTP endpoint. To change a setting from the web UI, type the slash command in the chat input (`/config set chat.model …`, `/model chat <pool-entry>`). Slash commands work in the web chat the same way they work in the TUI, and they go through the same canonical writer.

### After editing the file directly

Changes to `config.yaml` made by an editor still require an agent restart:

```bash
openparallax restart
```

Or from a conversation:

```
/restart
```

The loader runs in strict YAML mode (`KnownFields(true)`). Any unknown top-level key — most notably the legacy `llm:` block — produces a clear parse error rather than being silently ignored.

## Example: Minimal Config

```yaml
workspace: /home/user/.openparallax/atlas

identity:
  name: Atlas

models:
  - name: chat
    provider: anthropic
    model: claude-sonnet-4-6
    api_key_env: ANTHROPIC_API_KEY
  - name: shield
    provider: anthropic
    model: claude-haiku-4-5-20251001
    api_key_env: ANTHROPIC_API_KEY

roles:
  chat: chat
  shield: shield

shield:
  policy_file: security/shield/default.yaml

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

models:
  - name: chat
    provider: anthropic
    model: claude-sonnet-4-6
    api_key_env: ANTHROPIC_API_KEY
  - name: shield
    provider: openai
    model: gpt-5.4-mini
    api_key_env: OPENAI_API_KEY

roles:
  chat: chat
  shield: shield
  policy_file: security/shield/default.yaml
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
    token_env: TELEGRAM_BOT_TOKEN
    allowed_users: ["123456789"]
    private_only: true

  discord:
    enabled: true
    token_env: DISCORD_BOT_TOKEN
    allowed_guilds: ["9876543210"]

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
