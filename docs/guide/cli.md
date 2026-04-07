# CLI Commands

OpenParallax provides a single CLI with subcommands for managing the agent lifecycle, sessions, memory, audit, and diagnostics. After [installation](/guide/installation), the binary is on your PATH and every command below is invoked as `openparallax …`.

Source: [`cmd/agent/*.go`](https://github.com/openparallax/openparallax/tree/main/cmd/agent)

## Agent Lifecycle

### `start`

Start the agent engine, all configured channels, and optionally the web UI.

```
openparallax start [name] [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | — | Path to config.yaml |
| `--verbose` | `-v` | `false` | Enable verbose pipeline logging (writes engine.log) |
| `--daemon` | `-d` | `false` | Start in background (daemon mode) |
| `--tui` | — | `false` | Auto-attach interactive TUI in foreground |
| `--port` | — | `0` | Override web UI port |

The process manager spawns `internal-engine` as a child process. If the engine crashes, it is restarted automatically (max 5 crashes in 60 seconds). Exit code 75 triggers a clean restart (not counted as a crash).

**Examples:**

```bash
openparallax start              # Start default agent
openparallax start atlas        # Start agent named "atlas"
openparallax start -v           # Start with verbose logging
openparallax start -d           # Start as background daemon
openparallax start --tui        # Start and attach TUI
```

### `stop`

Gracefully stop a running engine by sending SIGTERM.

```
openparallax stop [name] [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | — | Path to config.yaml |

Waits up to 10 seconds for clean shutdown before sending SIGKILL.

### `restart`

Stop and restart the engine.

```
openparallax restart [name] [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | — | Path to config.yaml |

### `init`

Interactive wizard that creates a new workspace with smart defaults.

```
openparallax init [name]
```

Prompts for LLM provider, model, API key, agent name, and web UI password. Pass an agent name as an argument to skip the name prompt. Takes under 60 seconds.

**Examples:**

```bash
openparallax init           # Interactive setup
openparallax init atlas     # Pre-set agent name to "atlas"
```

### `delete`

Permanently delete an agent and its entire workspace.

```
openparallax delete <name> [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--yes` | `-y` | `false` | Skip confirmation prompt |

Requires the agent to be stopped. Prompts for confirmation by typing the agent slug.

### `list`

List all agents registered on this machine.

```
openparallax list
```

Aliases: `ls`

Displays a table with name, status (running/stopped), port, provider, model, and workspace path.

## Channels

### `attach`

Attach a UI channel to a running agent.

```
openparallax attach <channel> [name]
```

Supported channels: `tui` (interactive terminal UI).

**Examples:**

```bash
openparallax attach tui           # Attach TUI to default agent
openparallax attach tui atlas     # Attach TUI to agent "atlas"
```

### `detach`

Detach a running channel adapter from a running agent.

```
openparallax detach <channel> [name]
```

Sends a request to the agent's REST API to gracefully stop the channel.

**Examples:**

```bash
openparallax detach telegram
openparallax detach discord atlas
```

## Information

### `status`

Show workspace status including memory files, sessions, snapshots, and audit counts.

```
openparallax status [name] [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | — | Path to config.yaml |

### `config`

Print the current configuration with secrets masked.

```
openparallax config [name] [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | — | Path to config.yaml |

Fields containing `api_key`, `password`, `secret`, or `token` are automatically masked (first 4 + last 4 characters shown).

### `doctor`

Run a 14-point system health check.

```
openparallax doctor [name] [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | — | Path to config.yaml |

Checks: Config, **Config writer (round-trip through `Load()`)**, Workspace, SQLite, LLM Provider, Shield, Embedding, Browser, Email, Calendar, HEARTBEAT, Audit (chain integrity), Sandbox, Web UI.

The Config writer check serializes the loaded config back to a temp file via the canonical writer and re-loads it. It catches schema drift between writer and loader before the next restart turns it into a startup failure.

## Sessions

### `session delete`

Delete a session and its messages.

```
openparallax session delete <id> [flags]
openparallax session delete --all [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | — | Path to config.yaml |
| `--all` | — | `false` | Delete all sessions (prompts for confirmation) |

## Memory

### `memory`

List all memory files and their sizes.

```
openparallax memory [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | — | Path to config.yaml |

### `memory show`

Print a memory file's content.

```
openparallax memory show <file>
```

**Examples:**

```bash
openparallax memory show MEMORY.md
openparallax memory show PREFS.md
```

### `memory search`

Full-text search across memory.

```
openparallax memory search <query>
```

Returns up to 20 results with path, section, and snippet.

## Logs

### `logs`

Tail the engine log (requires verbose mode to be enabled).

```
openparallax logs [name] [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | — | Path to config.yaml |
| `--level` | — | — | Filter by level: `info`, `warn`, `error` |
| `--event` | — | — | Filter by event type (substring match) |
| `--lines` | `-n` | `50` | Number of lines to show |

**Examples:**

```bash
openparallax logs --level error
openparallax logs --event shield -n 100
openparallax logs atlas --level warn
```

## Audit

### `audit`

Query and verify the audit log.

```
openparallax audit [name] [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | — | Path to config.yaml |
| `--verify` | — | `false` | Verify hash chain integrity |
| `--session` | — | — | Filter by session ID |
| `--type` | — | `0` | Filter by audit event type (integer) |

**Examples:**

```bash
openparallax audit --verify              # Check chain integrity
openparallax audit --session abc123      # Filter by session
openparallax audit --type 3              # Filter by event type
```

## Authentication

### `auth`

Run the OAuth2 authorization flow for email and calendar integrations.

```
openparallax auth <provider> [flags]
```

Supported providers: `google`, `microsoft`.

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--account` | — | — | Email address for the OAuth account (required) |

Opens a browser for authorization, starts a local callback server, exchanges the code for tokens, and stores them encrypted in the database.

**Examples:**

```bash
openparallax auth google --account user@gmail.com
openparallax auth microsoft --account user@outlook.com
```

Requires `oauth.google` or `oauth.microsoft` client credentials in config.yaml.

## Skills

### `skill list`

List installed skills (global and workspace-level).

```
openparallax skill list
```

### `skill install`

Install a skill from the official repository or a Git URL.

```
openparallax skill install <name-or-url>
```

Skills are installed to `~/.openparallax/skills/`. Each skill directory must contain a `SKILL.md` file.

**Examples:**

```bash
openparallax skill install developer
openparallax skill install https://github.com/user/my-skill.git
```

### `skill remove`

Remove an installed skill.

```
openparallax skill remove <name>
```

## MCP Servers

### `mcp list`

List installed MCP server binaries.

```
openparallax mcp list
```

### `mcp install`

Download an MCP server binary from the official repository.

```
openparallax mcp install <name>
```

Binaries are installed to `~/.openparallax/mcp/<name>/`. After installation, add the server to `mcp.servers` in config.yaml.

**Examples:**

```bash
openparallax mcp install rss
```

### `mcp remove`

Remove an installed MCP server.

```
openparallax mcp remove <name>
```

## Downloads

### `get-classifier`

Download the Shield prompt-injection classifier (DeBERTa ONNX model and ONNX Runtime).

```
openparallax get-classifier [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--variant` | — | `"base"` | Model variant: `base` (~700MB, 98.8% accuracy) or `small` (~250MB, faster) |
| `--force` | — | `false` | Re-download even if files already exist |

Downloads to `~/.openparallax/models/prompt-injection/`. Without the classifier, Shield runs in heuristic-only mode.

### `get-vector-ext`

Download the sqlite-vec extension for native in-database vector search.

```
openparallax get-vector-ext
```

Downloads the latest prebuilt sqlite-vec shared library for the current platform from GitHub releases. The extension is stored at `~/.openparallax/extensions/sqlite-vec.<ext>` and loaded automatically on next startup.

Both native and pure-Go cosine similarity produce identical results. Native is faster at scale (50K+ chunks).

## Chronicle

### `chronicle`

List all Chronicle snapshots.

```
openparallax chronicle [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | — | Path to config.yaml |

### `chronicle diff`

Show file changes since a specific snapshot.

```
openparallax chronicle diff <snapshot-id>
```

### `chronicle rollback`

Restore files from a snapshot to their pre-action state. Shows which files will be restored before executing.

```
openparallax chronicle rollback <snapshot-id>
```

### `chronicle verify`

Verify the integrity of the Chronicle snapshot chain.

```
openparallax chronicle verify
```

## Global Behavior

All commands that require a workspace resolve the config path in this priority order:

1. Explicit `--config` flag
2. Positional agent name argument (looked up in the agent registry at `~/.openparallax/agents.json`)
3. Auto-detection: single agent in registry, then scan `~/.openparallax/*/config.yaml`

Internal commands (`internal-engine`, `internal-agent`, `internal-sub-agent`) are not user-facing. They are spawned by the process manager and should not be called directly.
