# Sessions

Every conversation with OpenParallax happens within a session. Sessions track message history, tool call results, artifacts, and metadata. There are two session types: normal and OTR (Off-the-Record).

## Normal Sessions

Normal sessions are the default. When you open the web UI or start a TUI conversation, a new session is created.

**Characteristics:**

- **Persistent** — all messages, tool calls, and results are stored in SQLite
- **Full tool access** — all configured tools are available
- **Memory integration** — conversations are logged to memory for future context
- **Titled automatically** — after 3 exchanges, the engine generates a title using the LLM
- **Resumable** — switch back to any previous session to continue the conversation

### Session Lifecycle

1. **Creation** — a session is created when you send your first message in a new conversation (or use `/new`)
2. **Active** — messages flow through the pipeline, tools are called, artifacts are generated
3. **Title generation** — after 3 user-agent exchanges, the LLM generates a concise title for the session
4. **Idle** — the session remains accessible in the sidebar/session list for future use
5. **Deletion** — sessions can be explicitly deleted via `/delete`, the CLI, or the web UI

### Session Storage

Normal sessions are stored in the SQLite database at `<workspace>/.openparallax/openparallax.db`. The database uses WAL mode for concurrent read/write access. Each session record includes:

- Session ID (unique identifier)
- Title (auto-generated or empty before 3 exchanges)
- Creation timestamp
- Last activity timestamp
- Message history (user messages, assistant responses, tool calls, results)
- Metadata (mode, channel source)

## OTR Sessions

OTR (Off-the-Record) mode creates a temporary session designed for sensitive or experimental conversations. Start an OTR session with the `/otr` slash command.

**Characteristics:**

- **Ephemeral** — session data is stored in a Go `sync.Map` in memory, never written to SQLite
- **Read-only tools** — write operations are filtered out at the tool definition level
- **No memory persistence** — the conversation is not logged to memory files or daily logs
- **Visual indicator** — UI accent colors change from cyan to amber
- **Lost on restart** — since data lives only in memory, restarting the engine discards OTR sessions

### Filtered Tools in OTR Mode

The following tools are removed in OTR sessions:

| Category | Filtered Tools |
|----------|---------------|
| File operations | `write_file`, `delete_file`, `move_file`, `copy_file`, `create_directory`, `delete_directory`, `move_directory`, `copy_directory` |
| Git | `git_commit`, `git_push` |
| Communication | `send_email` |
| Memory | `memory_write` |
| Canvas | `canvas_create`, `canvas_update`, `canvas_project` |
| Media | `generate_image`, `edit_image`, `generate_video` |
| Agents | `create_agent` |
| Email management | `email_move`, `email_mark` |
| System | `clipboard_write` |
| Utilities | `archive_create`, `spreadsheet_write` |

Read-only tools remain fully available: `read_file`, `list_directory`, `search_files`, `memory_search`, `git_status`, `git_diff`, `git_log`, `browser_navigate`, `browser_extract`, `read_calendar`, etc.

### OTR Visual Indicator

When OTR mode is active, the web UI applies the `.otr` CSS class to the document root. This overrides all 8 accent color tokens from cyan to amber:

- Borders, glows, and highlights shift to warm amber tones
- An "OTR" badge appears in the sidebar
- The color change is immediately visible across all UI components

This constant visual reminder prevents accidentally treating an OTR session as a normal session.

## Managing Sessions

### Listing Sessions

**Web UI:** Sessions appear in the sidebar, sorted by last activity. Each shows the auto-generated title (or "Untitled" before 3 exchanges) and a relative timestamp.

**CLI:**

```bash
openparallax session list
```

**Slash command:**

```
/sessions
```

### Switching Sessions

**Web UI:** Click any session in the sidebar to switch to it. The chat panel loads the full message history and the artifact canvas restores any associated artifacts.

**CLI TUI:** Use `/sessions` to list sessions, then start a new TUI connection specifying the session.

### Exporting Sessions

Export the current session as JSON:

```
/export
```

The export includes all messages, tool calls with parameters and results, Shield verdicts, timestamps, and session metadata. Useful for sharing conversations, debugging, or archival.

### Deleting Sessions

**From a conversation:**

```
/delete
```

Deletes the current session and starts a new one.

**From the CLI:**

```bash
# Delete a specific session by ID
openparallax session delete sess_abc123

# Delete all sessions
openparallax session delete --all
```

Deleting a session removes it from SQLite, including all messages and tool call history. This operation is irreversible. OTR sessions cannot be deleted via the CLI since they do not exist in the database.

## Session Titles

Session titles are generated automatically by the LLM after 3 user-agent exchanges. This delay ensures the title reflects the actual conversation topic rather than just the first message.

The title generation happens asynchronously and does not block the conversation. Titles appear in the sidebar and session listings once generated.

## Session IDs

Each session has a unique ID in the format `sess_<random>`. Session IDs are used in:

- WebSocket event filtering (events include a `session_id` field)
- Audit log entries (linking actions to their session)
- CLI commands for session management
- REST API endpoints

## Cross-Channel Sessions

When multiple channels are connected (web UI, CLI TUI, Telegram, etc.), each channel creates independent sessions. Sessions are not shared across channels — a Telegram conversation and a web UI conversation are separate sessions with separate histories.

The engine routes events to the correct channel adapter based on the session's `EventSender` implementation.

## Next Steps

- [Memory](/guide/memory) — how conversations feed into long-term memory
- [Security](/guide/security) — how OTR affects Shield evaluation
- [Web UI](/guide/web-ui) — visual features of session management
