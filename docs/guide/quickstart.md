# Quick Start

This guide takes you from a fresh build to your first conversation in under five minutes. It assumes you have already completed the [installation](/guide/installation) steps and have `dist/openparallax` ready.

## 1. Initialize a Workspace

The `init` command runs an interactive wizard that configures your agent:

```bash
./dist/openparallax init
```

The wizard walks through six steps:

1. **Agent name** — give your agent a name (default: "Atlas")
2. **Avatar** — pick an emoji avatar (hexagon, robot, brain, lightning, shield, or custom)
3. **LLM provider** — choose Anthropic, OpenAI, Google, or Ollama. The wizard auto-detects API keys in your environment.
4. **Chat model** — confirm or change the default model (e.g., `claude-sonnet-4-20250514` for Anthropic)
5. **Shield provider and model** — choose a provider for security evaluation. A faster/cheaper model works well here (e.g., `claude-haiku-4-5-20251001`). Cross-model evaluation is recommended for stronger security.
6. **Embedding provider** — choose OpenAI, Google, or Ollama for vector search, or skip for keyword-only search
7. **Workspace path** — where sessions, memory, and config are stored (default: `~/.openparallax/<agent-name>`)

Each step includes connection testing. If a connection fails, you can correct the settings inline.

At the end, the wizard asks whether to start the agent immediately.

You can also pass the agent name as an argument to skip the name prompt:

```bash
./dist/openparallax init Atlas
```

## 2. Start the Agent

If you did not start during init, launch manually:

```bash
./dist/openparallax start
```

You will see output like:

```
Engine started on localhost:13101 (LLM: anthropic/claude-sonnet-4-20250514)
Web UI available at http://127.0.0.1:3100
```

Three processes are now running:

- **Process Manager** — the `start` command itself, supervising the engine
- **Engine** — the privileged parent process running gRPC, HTTP/WebSocket, Shield, and all executors
- **Agent** — the sandboxed child process handling LLM conversation and TUI

### Start with the terminal TUI

To get an interactive terminal interface immediately:

```bash
./dist/openparallax start --tui
```

This attaches the Bubbletea TUI directly. When you quit the TUI (with `/quit` or Ctrl+C), the engine shuts down.

### Start in the background

```bash
./dist/openparallax start -d
```

Daemon mode prints the connection info and exits. The engine continues running in the background. Attach a TUI later with `openparallax attach tui`.

### Enable verbose logging

```bash
./dist/openparallax start -v
```

This writes structured JSON logs to `<workspace>/.openparallax/engine.log`, covering every pipeline step, Shield evaluation, and executor call.

## 3. Your First Conversation

### Via the Web UI

Open your browser to the URL printed at startup (default: `http://127.0.0.1:3100`).

The three-panel layout appears:

- **Sidebar** (left) — session list, new session button, settings
- **Artifact Canvas** (center) — displays generated files, code, diagrams
- **Chat Panel** (right) — conversation input and message stream

Type a message in the chat panel:

```
What can you do?
```

The agent responds with a summary of its capabilities based on your configuration (which tools are available, whether email/calendar/browser are set up, etc.).

### Via the CLI TUI

If you started with `--tui`, or attached with `openparallax attach tui`, type directly in the terminal. The Bubbletea interface provides real-time streaming, tool call rendering, and keyboard navigation.

## 4. Try a Tool Call

Ask the agent to do something that requires a tool. For example:

```
List the files in my workspace
```

Behind the scenes:

1. The agent calls `load_tools` to load the `files` tool group
2. It calls `list_directory` with the workspace path
3. Shield evaluates the action at Tier 0 — workspace reads are allowed by default policy
4. The directory listing is returned

Try something more involved:

```
Create a file called hello.txt with "Hello from OpenParallax!" in it
```

This time Shield evaluates the `write_file` action. With the default policy, file writes in the workspace are allowed through Tier 0/1. In the web UI, you can see the Shield verdict in the action envelope.

## 5. Check the Web UI Features

### Artifacts

When the agent creates files, generates code, or produces diagrams, they appear as tabs in the Artifact Canvas. Try:

```
Create an HTML page with a simple counter app
```

The generated HTML appears in a live-preview tab in the center panel. You can:

- Click tabs to switch between artifacts
- Right-click a tab to pin it (pinned tabs persist across sessions)
- Close unpinned tabs (maximum 6 unpinned tabs)

### Session Management

- Click **New Session** in the sidebar (or type `/new` in chat) to start a fresh conversation
- Previous sessions appear in the sidebar with auto-generated titles
- Click a session to switch back to it — full history is preserved

### Settings

Click the gear icon in the sidebar to access settings. You can view connection status, sandbox capabilities, and agent info.

## 6. Try OTR Mode

Off-the-Record mode creates a temporary session where nothing is persisted:

```
/otr
```

In OTR mode:

- The UI accent color changes from **cyan to amber** as a visual reminder
- **No memory persistence** — the conversation is not saved to the database or memory files
- **Read-only tools** — write operations (file writes, git commits, email sends, memory writes) are filtered out
- **Session data stays in memory** — stored in a `sync.Map`, never touching SQLite

OTR mode is useful for sensitive discussions, experimentation, or anything you do not want the agent to remember.

To return to normal mode, start a new session with `/new`.

## 7. Useful Slash Commands

These work in both the CLI and web UI:

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/new` | Start a new session |
| `/otr` | Start an OTR (off-the-record) session |
| `/status` | Show agent status and capabilities |
| `/sessions` | List recent sessions |
| `/export` | Export current session |
| `/clear` | Clear the display |
| `/quit` | Exit the TUI |

## 8. Run a Health Check

Verify everything is configured correctly:

```bash
./dist/openparallax doctor
```

This runs 13 checks covering config, workspace, SQLite, LLM provider, Shield, embedding, browser, email, calendar, heartbeat, audit chain integrity, sandbox, and web UI. Each check reports pass, warning (non-critical), or failure.

## 9. Stop the Agent

From the CLI TUI:

```
/quit
```

From any terminal:

```bash
./dist/openparallax stop
```

The process manager sends SIGTERM to the engine and waits up to 5 seconds for a clean shutdown.

## Next Steps

- [Configuration](/guide/configuration) — customize every aspect of your agent
- [CLI Commands](/guide/cli) — complete command reference
- [Web UI](/guide/web-ui) — layout, keyboard shortcuts, and features
- [Security](/guide/security) — understand Shield policies and sandbox
- [Memory](/guide/memory) — how the agent remembers across sessions
- [Skills](/guide/skills) — create custom domain-specific guidance
