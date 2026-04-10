# WebSocket Protocol

The web UI communicates with the Engine through a persistent WebSocket connection. This page documents the protocol for clients that want to build custom WebSocket integrations.

## Connection

```
ws://localhost:3100/api/ws
```

The connection is long-lived. The Engine sends events as they occur. The client sends messages as JSON frames.

### Authentication

When `web.host` is set to a non-localhost address, the WebSocket upgrade request must include a valid `op_session` cookie obtained from `POST /api/login`. Unauthenticated upgrade attempts receive a `401 Unauthorized` response and the connection is not established.

On localhost, authentication is not required for WebSocket connections (matching the REST API behavior).

### Message Size Limit

The server enforces a 10MB maximum message size (`conn.SetReadLimit`). Any WebSocket frame exceeding this limit is rejected and the connection is closed.

## Client → Engine Messages

### `message`

Send a user message to a session.

```json
{
  "type": "message",
  "session_id": "01HWXYZ...",
  "content": "Read the main.go file",
  "mode": "normal"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | Always `"message"` |
| `session_id` | string | yes | Target session. Must exist (created via REST). |
| `content` | string | yes | The message text. |
| `mode` | string | no | `"normal"` (default) or `"otr"`. Only relevant when creating a new OTR session. |

The server generates a unique message ID internally. There is no subscription step — events for a session are delivered to all connected clients automatically, filtered by `session_id`.

### `command`

Send a slash command.

```json
{
  "type": "command",
  "session_id": "01HWXYZ...",
  "content": "/status"
}
```

The server responds with a `command_result` event.

### `cancel`

Cancel the currently active LLM response for a session.

```json
{
  "type": "cancel",
  "session_id": "01HWXYZ..."
}
```

### `tier3_decision`

Respond to a Tier 3 human-in-the-loop approval request. Sent after the client receives a `tier3_approval_required` event.

```json
{
  "type": "tier3_decision",
  "action_id": "01HWDEF...",
  "decision": "approve"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | Always `"tier3_decision"` |
| `action_id` | string | yes | The action ID from the `tier3_approval_required` event. |
| `decision` | string | yes | `"approve"` or `"deny"`. |

### `ping`

Keepalive. The Engine responds with a `pong` frame.

```json
{
  "type": "ping"
}
```

## Engine → Client Messages

Every event from the Engine follows this envelope format:

```json
{
  "type": "event_type_here",
  "session_id": "01HWXYZ...",
  "timestamp": 1711929600000,
  "data": { ... }
}
```

### Session Filtering

Events are filtered by session before delivery:

1. `log_entry` events are **global** — delivered to all connected clients regardless of subscription.
2. All other events are delivered only to clients whose active `session_id` matches the event's `session_id`.

This prevents cross-session event leakage. A client connected to session A never sees events from session B.

### Event: `llm_token`

```json
{
  "type": "llm_token",
  "session_id": "01HWXYZ...",
  "timestamp": 1711929600123,
  "data": {
    "token": "Here's ",
    "message_id": "01HWABC..."
  }
}
```

Arrives rapidly during LLM streaming. Append `data.token` to the response buffer.

### Event: `action_started`

```json
{
  "type": "action_started",
  "session_id": "01HWXYZ...",
  "timestamp": 1711929601000,
  "data": {
    "action_id": "01HWDEF...",
    "tool_name": "read_file",
    "arguments": {"path": "src/main.go"}
  }
}
```

A tool call is being evaluated. Show a loading/thinking indicator.

### Event: `shield_verdict`

```json
{
  "type": "shield_verdict",
  "session_id": "01HWXYZ...",
  "timestamp": 1711929601050,
  "data": {
    "action_id": "01HWDEF...",
    "decision": "ALLOW",
    "tier": 0,
    "reason": "read_file: allowed by default policy rule 'allow-reads'",
    "tier_results": [
      {"tier": 0, "decision": "ALLOW", "reason": "matched rule: allow-reads"}
    ]
  }
}
```

### Event: `action_completed`

```json
{
  "type": "action_completed",
  "session_id": "01HWXYZ...",
  "timestamp": 1711929601200,
  "data": {
    "action_id": "01HWDEF...",
    "tool_name": "read_file",
    "result": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}",
    "is_error": false,
    "duration_ms": 15
  }
}
```

### Event: `response_complete`

```json
{
  "type": "response_complete",
  "session_id": "01HWXYZ...",
  "timestamp": 1711929605000,
  "data": {
    "content": "Here's the contents of main.go...",
    "message_id": "01HWABC...",
    "thoughts": [
      {
        "stage": "reasoning",
        "summary": "The user wants to read main.go. I'll use the read_file tool."
      },
      {
        "stage": "tool_call",
        "summary": "read_file → package main\\nimport...",
        "detail": {"tool_name": "read_file", "success": true}
      }
    ],
    "token_usage": {
      "input_tokens": 1200,
      "output_tokens": 350,
      "total_tokens": 1550
    }
  }
}
```

Signals the response is complete. Stop streaming indicators, finalize the message display.

### Event: `otr_blocked`

```json
{
  "type": "otr_blocked",
  "session_id": "01HWXYZ...",
  "timestamp": 1711929602000,
  "data": {
    "tool_name": "write_file",
    "reason": "OTR mode is read-only — write_file requires write access."
  }
}
```

### Event: `error`

```json
{
  "type": "error",
  "session_id": "01HWXYZ...",
  "timestamp": 1711929603000,
  "data": {
    "code": "LLM_CALL_FAILED",
    "message": "Anthropic API returned 429: rate limit exceeded",
    "recoverable": true
  }
}
```

### Event: `tier3_approval_required`

```json
{
  "type": "tier3_approval_required",
  "session_id": "01HWXYZ...",
  "timestamp": 1711929604000,
  "data": {
    "action_id": "01HWDEF...",
    "tool_name": "shell_execute",
    "target": "rm -rf /tmp/cache",
    "reasoning": "Destructive shell command targeting filesystem",
    "timeout_secs": 300
  }
}
```

Requests human-in-the-loop approval for an action that Shield escalated to Tier 3. The client should display an approval prompt and respond with a `tier3_decision` message before `timeout_secs` expires. If no decision is received before the timeout, the action is denied automatically.

### Event: `log_entry`

```json
{
  "type": "log_entry",
  "session_id": "",
  "timestamp": 1711929600000,
  "data": {
    "level": "info",
    "event": "shield_verdict",
    "message": "Tier 0: ALLOW read_file src/main.go",
    "fields": {"tier": 0, "decision": "ALLOW", "tool": "read_file"}
  }
}
```

Global event — not filtered by session. The web UI displays these in the developer console panel.

### Event: `pong`

```json
{
  "type": "pong"
}
```

Response to client `ping`.

## Connection Lifecycle

```
Client                          Engine
  │                               │
  ├── WebSocket handshake ───────►│
  │   (include op_session cookie) │
  │                               │
  │◄── connection accepted ───────┤
  │                               │
  ├── message {session_id} ──────►│
  │                               │
  │◄── llm_token ────────────────┤
  │◄── llm_token ────────────────┤
  │◄── action_started ───────────┤
  │◄── shield_verdict ───────────┤
  │◄── action_completed ─────────┤
  │◄── llm_token ────────────────┤
  │◄── response_complete ────────┤
  │                               │
  ├── ping ──────────────────────►│
  │◄── pong ─────────────────────┤
  │                               │
  ├── close ─────────────────────►│
  └───────────────────────────────┘
```

## Error Handling

- If the WebSocket connection drops, the client should reconnect with exponential backoff.
- Events emitted while the client is disconnected are lost — they are not buffered or replayed.
- After reconnecting, the client should fetch recent history via `GET /api/sessions/:id/messages` to catch up on any missed messages.

## Multiple Clients

Multiple WebSocket clients can connect simultaneously. The `EventBroadcaster` fans out events to all connected clients, filtered by session. This means:

- Two browser tabs viewing the same session both see the same events in real-time.
- A CLI client and a web client can be connected to different sessions simultaneously.
- Channel adapters (WhatsApp, Telegram, etc.) use their own `EventSender` implementations rather than WebSocket, but the event content is identical.
