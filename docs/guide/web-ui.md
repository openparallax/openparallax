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

## Console Log

The web UI includes a developer console accessible from the settings panel. It shows raw WebSocket events, connection status, and diagnostic information. Useful for debugging connectivity issues or inspecting the event stream.

## Connection Management

The WebSocket connection includes automatic reconnection with exponential backoff. Connection status is displayed in the UI:

- **Connected** — green indicator, real-time events flowing
- **Reconnecting** — yellow indicator, attempting to restore connection
- **Disconnected** — red indicator, manual reconnection may be needed

If the engine restarts (via `/restart` or crash recovery), the WebSocket reconnects automatically and resumes the current session.

## Settings Panel

The settings panel (gear icon in the sidebar) displays:

- **Agent info** — name, provider, model
- **Connection status** — WebSocket state, gRPC state
- **Sandbox status** — kernel sandbox mode and capabilities
- **Session info** — current session ID, message count, OTR status

## Next Steps

- [Sessions](/guide/sessions) — normal and OTR session management
- [CLI Commands](/guide/cli) — the terminal-based alternative
- [Configuration](/guide/configuration) — customize the web UI port, auth, and more
