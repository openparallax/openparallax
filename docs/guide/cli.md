# CLI Commands

OpenParallax provides a Cobra-based CLI with subcommands for managing the agent lifecycle, sessions, memory, audit, and diagnostics. The CLI binary is at `dist/openparallax`.

## Commands

### start

Start the engine, agent, and all configured channels.

```bash
openparallax start [name] [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | `-c` | Path to config.yaml |
| `--verbose` | `-v` | Enable verbose pipeline logging to engine.log |
| `--daemon` | `-d` | Start in background (daemon mode) |
| `--tui` | | Auto-attach the terminal TUI in foreground |
| `--port` | | Override web UI port |

**Examples:**

```bash
# Start with the nearest config.yaml
openparallax start

# Start a named agent from the registry
openparallax start Atlas

# Start with explicit config
openparallax start -c ~/.openparallax/atlas/config.yaml

# Start in background with logging
openparallax start -d -v

# Start with TUI attached
openparallax start --tui
```

The process manager spawns the engine, which starts the gRPC server, HTTP/WebSocket server, and the sandboxed agent. It monitors for crashes (max 5 in 60 seconds before giving up) and handles restart requests (exit code 75).

If the agent is already running, `start` reports the existing PID and port.

---

### init

Interactive workspace setup wizard.

```bash
openparallax init [name]
```

Walks through agent name, avatar, LLM provider, model, Shield provider, embedding provider, and workspace path. Tests connections during setup. Creates the workspace directory, config.yaml, template files, SQLite database, and canary token.

Pass the agent name as an argument to skip the name prompt:

```bash
openparallax init Atlas
```

See [Quick Start](/guide/quickstart) for a detailed walkthrough.

---

### stop

Stop a running agent.

```bash
openparallax stop [name]
```

Sends SIGTERM to the engine process and waits for a clean shutdown.

---

### restart

Stop and restart the engine.

```bash
openparallax restart [name]
```

Equivalent to `stop` followed by `start`. Also available as the `/restart` slash command from within a conversation.

---

### attach

Attach a channel to a running agent.

```bash
openparallax attach <channel> [name]
```

Supported channels:

| Channel | Description |
|---------|-------------|
| `tui` | Interactive terminal UI (Bubbletea) |
| `telegram` | Telegram bot |
| `discord` | Discord bot |
| `signal` | Signal messenger |

**Example:**

```bash
# Attach a TUI to the running agent
openparallax attach tui

# Attach TUI to a named agent
openparallax attach tui Atlas
```

The TUI opens directly on `/dev/tty` for terminal I/O and connects to the engine via gRPC. When the TUI exits, the engine continues running.

---

### detach

Detach a channel from a running agent.

```bash
openparallax detach <channel> [name]
```

Stops the specified channel adapter without affecting the engine.

---

### status

Show workspace statistics.

```bash
openparallax status [name]
```

Displays agent name, workspace path, running state, session count, memory entries, and active channels.

---

### config

Print the current configuration (secrets are masked).

```bash
openparallax config [name] [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | `-c` | Path to config.yaml |

---

### session

Manage conversation sessions.

```bash
openparallax session <subcommand> [flags]
```

#### session delete

Delete one or all sessions.

```bash
# Delete a specific session
openparallax session delete <session-id>

# Delete all sessions
openparallax session delete --all
```

| Flag | Description |
|------|-------------|
| `--all` | Delete all sessions |

---

### list

List registered agents.

```bash
openparallax list
```

Shows all agents registered via `init`, with their names, workspace paths, ports, and running status.

---

### memory

View and search the agent's memory.

```bash
openparallax memory <subcommand> [name] [flags]
```

#### memory show

Display memory entries.

```bash
openparallax memory show
```

#### memory search

Search memory using full-text search (and vector search if embeddings are configured).

```bash
openparallax memory search "query text"
```

---

### logs

Tail the engine log. Requires the agent to be started with `-v` (verbose mode).

```bash
openparallax logs [name] [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--level` | Filter by log level: `debug`, `info`, `warn`, `error` | (all) |
| `--event` | Filter by event type (e.g., `shield_verdict`, `tool_call`) | (all) |
| `--lines` | Number of lines to show | `50` |

**Examples:**

```bash
# Show last 50 log entries
openparallax logs

# Show only errors
openparallax logs --level error

# Show Shield verdicts
openparallax logs --event shield_verdict

# Show last 100 entries for a named agent
openparallax logs Atlas --lines 100
```

---

### audit

Query the tamper-evident audit log.

```bash
openparallax audit [name] [flags]
```

| Flag | Description |
|------|-------------|
| `--verify` | Verify the SHA-256 hash chain integrity |
| `--session` | Filter by session ID |
| `--type` | Filter by audit entry type |

**Examples:**

```bash
# Show recent audit entries
openparallax audit

# Verify chain integrity
openparallax audit --verify

# Filter by session
openparallax audit --session "sess_abc123"

# Filter by type
openparallax audit --type ACTION_EXECUTED
```

Audit entry types include `ACTION_PROPOSED`, `ACTION_EVALUATED`, `ACTION_APPROVED`, `ACTION_BLOCKED`, `ACTION_EXECUTED`, and `ACTION_FAILED`.

---

### doctor

Run a 13-point system health check.

```bash
openparallax doctor [name] [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | `-c` | Path to config.yaml |

The doctor checks:

| # | Check | What It Verifies |
|---|-------|-----------------|
| 1 | Config | config.yaml loads without errors |
| 2 | Workspace | Workspace directory exists |
| 3 | SQLite | Database opens in WAL mode, reports size |
| 4 | LLM Provider | Provider and model are configured |
| 5 | Shield | Policy file exists, reports Tier 2 daily budget |
| 6 | Embedding | Embedding provider configured (warns if missing) |
| 7 | Browser | Chromium-based browser detected for automation |
| 8 | Email | SMTP host configured |
| 9 | Calendar | Calendar provider configured |
| 10 | HEARTBEAT | Parses HEARTBEAT.md, reports scheduled task count |
| 11 | Audit | Verifies hash chain integrity, reports entry count |
| 12 | Sandbox | Reports kernel sandbox mode and capabilities |
| 13 | Web UI | Reports configured web port |

Results are color-coded: green check for pass, yellow warning for non-critical issues, red X for failures.

---

### get-classifier

Download the Shield ONNX classifier model.

```bash
openparallax get-classifier [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--variant` | Model variant: `base` (~700MB, 98.8% accuracy) or `small` (~250MB, faster) | `base` |
| `--force` | Re-download even if files exist | `false` |

See [Installation](/guide/installation) for details.

---

### auth

Manage web UI authentication.

```bash
openparallax auth set-password
```

Sets a password for web UI access. The bcrypt hash is stored in config.yaml under `web.password_hash`.

---

## Slash Commands

Slash commands are available within conversations, in both the CLI TUI and the web UI chat panel. Type them as messages.

| Command | Description |
|---------|-------------|
| `/help` | Show all available slash commands |
| `/new` | Start a new normal session |
| `/otr` | Start a new OTR (off-the-record) session |
| `/quit` | Exit the TUI (CLI only) |
| `/clear` | Clear the chat display |
| `/status` | Show agent status, sandbox info, and active session |
| `/restart` | Restart the engine (reconnects automatically) |
| `/export` | Export the current session as JSON |
| `/delete` | Delete the current session |
| `/sessions` | List all sessions with titles and timestamps |

### /otr

Starts an Off-the-Record session:

- UI accents change from cyan to amber
- No data written to SQLite or memory files
- Write tools are filtered out (file writes, git commits, email sends, etc.)
- Session data lives in memory only

See [Sessions](/guide/sessions) for more details.

### /export

Exports the current session as a JSON document containing all messages, tool calls, Shield verdicts, and metadata. Useful for sharing conversations or debugging.

### /restart

Triggers an engine restart. The engine exits with code 75, which the process manager recognizes as a restart request (not a crash). The engine respawns without counting against the crash budget.

## Global Flags

These flags are available on all commands:

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | `-c` | Path to config.yaml (overrides auto-detection) |

When no `--config` is specified, the CLI searches for `config.yaml` in the current directory and parent directories, then checks the agent registry.

## Next Steps

- [Web UI](/guide/web-ui) — the browser-based interface
- [Sessions](/guide/sessions) — normal and OTR session management
- [Troubleshooting](/guide/troubleshooting) — diagnostics and common issues
