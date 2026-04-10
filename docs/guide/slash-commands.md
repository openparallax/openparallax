# Slash Commands

Slash commands are typed inside an active conversation (CLI, web UI, or any messaging channel) to control the session, inspect state, or change settings without leaving the chat. They are different from the [CLI commands](/guide/cli) you run in your shell — those manage the agent process from outside; slash commands act on the conversation you are in.

OpenParallax ships with **19 slash commands**, organized below by what they do.

## Where they work

Most slash commands work in every channel (CLI TUI, web UI, Telegram, Discord, Signal, iMessage). A few are CLI- and web-only because they touch local files (logs, config) or need a richer display surface than a chat message.

## Session

Commands that act on the current conversation.

| Command | Description |
|---|---|
| `/help` | Show all available slash commands |
| `/new` | Start a new normal session |
| `/otr` | Start a new [OTR (Off-The-Record) session](/guide/sessions#otr-mode) — read-only, never persisted |
| `/quit` | **Close the current session and start a new one.** This does NOT exit the agent or close the TUI — to exit the CLI, press `Ctrl+C`. |
| `/clear` | Clear the chat view (does not delete messages) |
| `/sessions` | List recent sessions with their IDs and titles |
| `/switch <id-prefix>` | Switch to another session by ID prefix |
| `/delete` | Delete the current session and start a new one |
| `/title <new title>` | Rename the current session |
| `/history` | Show the most recent messages in the current session |
| `/export` | Export the current session as markdown |

## Status

Commands that report state.

| Command | Description |
|---|---|
| `/status` | Show system health: agent name, model, session count, Shield budget, sandbox state |
| `/usage` | Show today's LLM token usage broken down by role (chat, shield, embedding) |
| `/doctor` | Run the [15-point health check](/guide/cli#doctor) and report results inline |
| `/audit` | Verify the audit trail's hash chain integrity. Usage: `/audit verify` |

## Configuration

Commands that change agent behavior. **CLI and web UI only** — these are not exposed in messaging channels for security reasons (a compromised channel adapter shouldn't be able to swap your model or rewrite config from a chat message).

| Command | Description |
|---|---|
| `/config` | Show the current config. `/config set <key> <value>` updates a single key (non-security keys only). **Persists to `config.yaml`** through the canonical writer. |
| `/model` | Show the current model role mapping. `/model set <role> <model>` switches the model used for a role (chat, shield, embedding, sub_agent, image, video). **Persists to `config.yaml`** so the change survives a restart. |
| `/restart` | Restart the engine in place. The process manager respawns the engine without losing your workspace state. |
| `/logs` | Manage engine logs. `/logs flush [days]` removes entries older than N days (default 30) |

## Quick reference card

The fast version, for the back of your hand:

| Want to... | Type |
|---|---|
| See every command | `/help` |
| Start fresh, keep history | `/new` |
| Start fresh, **forget** what you say next | `/otr` |
| Close this session, open a new one | `/quit` |
| List your sessions | `/sessions` |
| Jump to another session | `/switch <id-prefix>` |
| Delete this session | `/delete` |
| Rename this session | `/title My Refactor Notes` |
| Save this session as markdown | `/export` |
| Check Shield's daily budget | `/status` |
| Check today's token spend | `/usage` |
| Run a health check | `/doctor` |
| Restart the engine | `/restart` |

## Exiting the agent

There is no slash command to exit the agent — that's intentional. Exiting is a process-level action that should not be triggered from a chat message, since the message could come from a compromised channel.

What `Ctrl+C` and closing windows actually do depends on **how you started the agent**:

| You ran... | What's running in your terminal | `Ctrl+C` in TUI | Close terminal | Close browser tab |
|---|---|---|---|---|
| `openparallax start` (default, foreground) | Engine + process manager | **Stops the engine** (process manager catches the signal) | **Stops the engine** (SIGHUP propagates) | n/a — engine still runs without the web UI client |
| `openparallax start --tui` | Engine + process manager + TUI | TUI exits → process manager shuts the engine down | Same | n/a |
| `openparallax start -d` (daemon) | Nothing — engine is detached | n/a — no TUI in this terminal | Engine continues (detached) | Engine continues |
| `openparallax start -d` then `openparallax attach tui` | Only the attach process | Only the attach process exits; **engine keeps running** | Same | n/a |

The web UI is always a passive client. **Closing the browser tab never stops the engine** — the WebSocket disconnects, your session stays alive, and you can reconnect by opening the web UI again.

### To fully stop the engine

Whatever mode you're in, this always works:

```bash
openparallax stop
```

It sends SIGTERM to the engine process (whose PID is recorded in your workspace) and waits up to 5 seconds for clean shutdown.

### Quick rules of thumb

- **Want to keep the agent running for messaging channels** while you walk away? Use `openparallax start -d` to launch it as a background daemon, then attach a TUI later if you want.
- **Want a quick interactive session that auto-cleans-up** when you're done? Use `openparallax start --tui`. Closing the TUI shuts everything down.
- **Already have an agent running** and just want to talk to it from a fresh terminal? Use `openparallax attach tui`. Ctrl+C only detaches you; the engine survives.

## See also

- [CLI Commands](/guide/cli) — `openparallax …` commands you run in your shell
- [Sessions & OTR](/guide/sessions) — how sessions persist (or don't, in OTR mode)
- [Configuration](/guide/configuration) — what `/config set` can and cannot change
