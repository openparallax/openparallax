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
| `/doctor` | Run the [13-point health check](/guide/cli#doctor) and report results inline |
| `/audit` | Verify the audit trail's hash chain integrity |

## Configuration

Commands that change agent behavior. **CLI and web UI only** — these are not exposed in messaging channels for security reasons (a compromised channel adapter shouldn't be able to swap your model or rewrite config from a chat message).

| Command | Description |
|---|---|
| `/config` | Show the current config. `/config set <key> <value>` updates a single key (non-security keys only) |
| `/model` | Show the current model role mapping. `/model set <role> <model>` switches the model used for a role (chat, shield, embedding) |
| `/restart` | Restart the engine in place. The process manager respawns the engine without losing your workspace state. |
| `/logs` | Manage engine logs — useful for flushing old entries when the log file gets large |

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

There is no slash command to exit the agent — that's intentional, because exiting the agent is a process-level action that should not be triggered from a chat message (which could come from a compromised channel).

To exit:

- **CLI TUI**: press `Ctrl+C` (or close the terminal)
- **Web UI**: close the browser tab
- **Background engine**: from another terminal, run `openparallax stop`

The engine continues running even after you close the CLI or web UI — your sessions stay alive and you can re-attach with `openparallax start --tui` or by opening the web UI again. Use `openparallax stop` from the shell to fully shut down the engine.

## See also

- [CLI Commands](/guide/cli) — `openparallax …` commands you run in your shell
- [Sessions & OTR](/guide/sessions) — how sessions persist (or don't, in OTR mode)
- [Configuration](/guide/configuration) — what `/config set` can and cannot change
