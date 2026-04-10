# Web UI

OpenParallax includes a browser-based interface built with Svelte 4 and a glassmorphism design system. The web UI connects to the engine via WebSocket for real-time streaming and REST for session management.

## Accessing the Web UI

The web UI starts automatically with the engine (unless `web.enabled` is set to `false` in config.yaml). The URL is printed at startup:

```
Web UI available at http://127.0.0.1:3100
```

The port is configurable via `web.port` in config.yaml or the `--port` flag on `openparallax start`.

If authentication is enabled (`web.auth: true`), you will be prompted for a password. On first start with no password configured, the engine generates a one-time password and prints it to the console.

## Two-Panel Layout

The interface uses a two-panel layout:

```
┌──────────┬──────────────────────────────────────┐
│          │                                      │
│ Sidebar  │           Chat Panel                 │
│  240px   │           (flex: 1)                  │
│          │                                      │
│          │                                      │
│          │                                      │
└──────────┴──────────────────────────────────────┘
```

### Sidebar (240px)

The left panel provides navigation and session management:

- **New Session** button — starts a fresh conversation
- **Session list** — previous sessions displayed with auto-generated titles and timestamps
- **Active session** highlighted with accent color
- **Settings** — gear icon opens the settings panel
- **OTR indicator** — amber badge when in Off-the-Record mode

Click a session to switch to it. Full message history and tool call results are preserved.

### Chat Panel (flex: 1)

The main panel is the conversation interface:

- **Message input** at the bottom with a send button
- **Message stream** showing user messages, agent responses, and tool call envelopes
- **Tool call envelopes** — collapsible sections showing the action type, Shield verdict, and result
- **Streaming** — agent responses stream in real-time via WebSocket

### Drag-to-Resize

Panel widths are adjustable by dragging the divider between panels. The widths are stored as CSS custom properties (`--sw` for sidebar width) and persist in localStorage.

## Responsive Breakpoints

The layout adapts to different screen sizes:

| Breakpoint | Width | Layout |
|------------|-------|--------|
| Full | > 1200px | Both panels visible |
| Compact | 800-1200px | Sidebar collapses to icons, chat panel fills space |
| Mobile | < 800px | Single panel view with navigation tabs |

On compact and mobile layouts, the sidebar can be toggled with a hamburger menu button.

## Glassmorphism Design

The UI uses a glassmorphism design system with translucent panels, backdrop blur effects, and layered depth. Key design tokens:

- **Background** — dark gradient base with frosted-glass panel overlays
- **Accent colors** — 8 CSS custom property variants (`--accent-base`, `--accent-dim`, `--accent-subtle`, `--accent-ghost`, `--accent-glow`, `--accent-glow-strong`, `--accent-border`, `--accent-border-active`)
- **Typography** — Exo 2 for body text, JetBrains Mono for code and badges
- **Borders** — subtle translucent borders with accent-colored highlights for active elements

## OTR Mode

When an OTR (Off-the-Record) session is active, the UI transforms visually:

- All accent colors shift from **cyan to amber** via the `.otr` CSS class on the document root
- This affects borders, glows, badges, buttons, and highlights throughout the interface
- An "OTR" badge appears in the sidebar
- The visual change serves as a constant reminder that the session is ephemeral and restricted

The color change is applied by overriding all 8 `--accent-*` CSS tokens. See [Sessions](/guide/sessions) for details on OTR behavior.

## Real-Time Events

The web UI receives 7 event types from the engine via WebSocket:

| Event | Description |
|-------|-------------|
| `llm_token` | Streaming text token from the LLM response |
| `action_started` | A tool call has been proposed and is being evaluated |
| `shield_verdict` | Shield has evaluated the tool call (ALLOW/BLOCK/ESCALATE) |
| `action_completed` | A tool call has been executed |
| `response_complete` | The LLM has finished its response |
| `otr_blocked` | A tool call was blocked because the session is OTR |
| `error` | An error occurred during processing |

Events are filtered by `session_id` to prevent cross-session corruption. The `log_entry` event type is global and processed before the session filter for the console log.

## Tool Call Display

When the agent calls a tool, the UI displays a collapsible envelope showing:

1. **Action type** — the tool being called (e.g., `write_file`, `git_commit`)
2. **Parameters** — the arguments passed to the tool
3. **Shield verdict** — the security evaluation result (tier, decision, confidence)
4. **Result** — the tool execution output or error

Envelopes start collapsed for completed actions and expanded for in-progress actions. Multiple tool calls in the same LLM turn are grouped together.

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Enter` | Send message |
| `Shift+Enter` | New line in message input |
| `Ctrl+N` / `Cmd+N` | New session |
| `Escape` | Close settings / cancel current action |

## Console & Logging

The web UI includes a real-time console that streams structured engine events as they happen. It is one of OpenParallax's most powerful diagnostic tools — every Shield verdict, tool execution, sub-agent spawn, memory write, and error flows through it live.

### What the console shows

The console streams the same structured JSON events that power the chat UI, plus engine-level log entries that the chat panel does not render:

- **Shield verdicts** — every tool call shows the tier that evaluated it, the decision (ALLOW/BLOCK/ESCALATE), the confidence score, and the reasoning. Use this to understand *why* an action was blocked and which policy rule or heuristic fired.
- **Tool execution** — start, complete, duration, and error for every tool call. See which tools are slow and which are failing.
- **Sub-agent lifecycle** — spawn, status, completion, and failure events for every sub-agent. Monitor parallel work in real time.
- **Memory writes** — compaction flushes and session summarizations as they land in MEMORY.md.
- **Connection events** — WebSocket connections, agent gRPC stream status, reconnections.
- **Errors** — pipeline errors, LLM call failures, and agent crashes with full error messages.

### Using logs to tune security

The console is the fastest way to tune your Shield policy:

1. **Identify over-blocking.** If legitimate actions are blocked, the console shows exactly which tier and rule fired. A Tier 0 block names the policy rule; a Tier 1 block names the heuristic rule and its description; a Tier 2 block shows the LLM evaluator's reasoning.
2. **Identify under-blocking.** If actions you expected to be caught are passing, check the tier that evaluated them and the confidence score. Low confidence on a Tier 1 ALLOW may indicate the classifier needs a policy override.
3. **Budget monitoring.** The console shows when the daily Tier 2 budget is exhausted and whether the engine fails closed (blocks) or open (allows with reduced confidence). Adjust `general.daily_budget` based on observed usage.
4. **Rate limit tuning.** Shield rate-limit-hit events appear in the console. If legitimate bursts are being throttled, increase `general.rate_limit`.

### CLI logging

From the CLI, use the `/logs` command or the `openparallax logs` subcommand:

```bash
# Tail the last 50 lines, filtered by event type
openparallax logs --lines 50 --event shield_verdict

# Filter by log level
openparallax logs --level warn
```

When the engine is started with `-v` (verbose), all structured events are written to `<workspace>/.openparallax/engine.log`. This file is the definitive record of everything the engine did — it persists across restarts and can be searched with standard tools (`grep`, `jq`).

### Audit log

The audit log (`<workspace>/.openparallax/audit.jsonl`) is a separate, append-only, hash-chained record of security-relevant events only. Use it for compliance verification:

```bash
# Verify the hash chain is intact
openparallax audit --verify

# Query by event type
openparallax audit --type ACTION_BLOCKED --lines 20
```

See the [audit documentation](/audit/go) for the full event type catalog and verification protocol.

## Connection Management

The WebSocket connection includes automatic reconnection with exponential backoff. Connection status is displayed in the UI:

- **Connected** — green indicator, real-time events flowing
- **Reconnecting** — yellow indicator, attempting to restore connection
- **Disconnected** — red indicator, manual reconnection may be needed

If the engine restarts (via `/restart` or crash recovery), the WebSocket reconnects automatically and resumes the current session.

## Settings Panel

The settings panel (gear icon in the sidebar) is **read-only**. It displays the current configuration as labels and values — no editors, no Save button. A banner at the top points at the slash command path for changes.

What you see:

- **Agent info** — name, avatar
- **Chat model** — provider, model, API key configured, base URL
- **Shield** — policy, evaluator provider/model, Tier 2 budget and usage
- **Memory** — embedding provider/model
- **MCP servers** — configured servers
- **Email and calendar** — configured channels
- **Web** — port
- **Sandbox** — kernel sandbox mode and capabilities

To **change** a setting from the web UI, type the slash command in the chat input:

- `/config set chat.model claude-haiku-4-5-20251001` — change a model on the active role
- `/model chat <pool-entry>` — switch which model from the pool a role points at
- `/config set identity.name Bear` — change the agent's display name

Slash commands work in the web chat the same way they work in the TUI. There is no HTTP write endpoint for settings — the read-only design closes the secret-exfiltration and Shield-disarm vectors that an HTTP write surface would expose. See [Configuration → Editing Config at Runtime](/guide/configuration#editing-config-at-runtime) for the complete list of settable keys.

## Next Steps

- [Sessions](/guide/sessions) — normal and OTR session management
- [CLI Commands](/guide/cli) — the terminal-based alternative
- [Configuration](/guide/configuration) — customize the web UI port, auth, and more
