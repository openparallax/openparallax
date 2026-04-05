# REST API Reference

The Engine exposes a REST API on the web server port (default `:3100`). This API is used by the web UI and can be used by any HTTP client.

All endpoints return JSON. Authentication is required when the web server is bound to a non-localhost address (see [Configuration — Web](/reference/config#web-web-ui)).

## Authentication

When `web.host` is set to a non-localhost address, all endpoints (except `/api/login`) require a valid session cookie.

### `POST /api/login`

Authenticate and receive a session cookie.

**Request:**
```json
{
  "password": "your-password"
}
```

**Response (200):**
```json
{
  "ok": true
}
```

Sets an `op_session` cookie with attributes:
- `HttpOnly` — not accessible via JavaScript
- `Secure` — only sent over HTTPS
- `SameSite=Strict` — no cross-site requests
- `Path=/`
- `Max-Age=86400` (24 hours)

**Response (401):**
```json
{
  "error": "invalid password"
}
```

## Session Management

### `GET /api/sessions`

List all sessions.

**Query parameters:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `include_otr` | bool | `false` | Include OTR sessions in the response |

**Response (200):**
```json
{
  "sessions": [
    {
      "id": "01HWXYZ...",
      "title": "Debugging the auth middleware",
      "mode": "normal",
      "created_at": 1711929600,
      "updated_at": 1711933200,
      "message_count": 12
    }
  ]
}
```

### `POST /api/sessions`

Create a new session.

**Request:**
```json
{
  "mode": "normal"
}
```

`mode` is either `"normal"` or `"otr"`.

**Response (201):**
```json
{
  "id": "01HWXYZ...",
  "mode": "normal",
  "created_at": 1711929600
}
```

### `DELETE /api/sessions/:id`

Delete a session and all its messages.

**Response (200):**
```json
{
  "ok": true
}
```

### `DELETE /api/sessions`

Delete all sessions.

**Query parameters:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `confirm` | bool | required | Must be `true` to confirm bulk deletion |

**Response (200):**
```json
{
  "deleted": 15
}
```

### `GET /api/sessions/:id/messages`

Get message history for a session.

**Query parameters:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `limit` | int | `50` | Maximum messages to return |
| `offset` | int | `0` | Skip first N messages |

**Response (200):**
```json
{
  "messages": [
    {
      "id": "01HWXYZ...",
      "role": "user",
      "content": "Read the main.go file",
      "timestamp": 1711929600,
      "thoughts": null
    },
    {
      "id": "01HWABC...",
      "role": "assistant",
      "content": "Here's the contents of main.go...",
      "timestamp": 1711929605,
      "thoughts": [
        {
          "stage": "tool_call",
          "summary": "read_file → package main...",
          "detail": {"tool_name": "read_file", "success": true}
        }
      ]
    }
  ]
}
```

### `POST /api/sessions/:id/export`

Export a session as a downloadable file.

**Query parameters:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `format` | string | `"markdown"` | Export format: `"markdown"` or `"json"` |

**Response (200):** File download with appropriate Content-Type and Content-Disposition headers.

## Messages

### `POST /api/sessions/:id/messages`

Send a message to a session. This is an alternative to the WebSocket interface for clients that don't need streaming.

**Request:**
```json
{
  "content": "What files are in the src directory?",
  "message_id": "01HWXYZ..."
}
```

**Response (200):**
```json
{
  "message_id": "01HWABC...",
  "content": "Here are the files in src/...",
  "thoughts": [...],
  "token_usage": {
    "input_tokens": 1200,
    "output_tokens": 350,
    "total_tokens": 1550
  }
}
```

::: tip Prefer WebSocket
For real-time streaming, use the WebSocket interface. The REST endpoint waits for the complete response before returning — no streaming tokens. Use REST only for programmatic access or simple integrations.
:::

## Status & Health

### `GET /api/status`

Get the current agent status.

**Response (200):**
```json
{
  "name": "Atlas",
  "status": "running",
  "uptime_seconds": 3600,
  "version": "0.1.0",
  "provider": "anthropic",
  "model": "claude-sonnet-4-6",
  "sandbox": {
    "verified": true,
    "status": "sandboxed",
    "platform": "linux",
    "mechanism": "landlock",
    "probes": [
      {"name": "file_read", "status": "blocked"},
      {"name": "file_write", "status": "blocked"},
      {"name": "network", "status": "blocked"}
    ]
  },
  "shield": {
    "policy_file": "policies/default.yaml",
    "classifier_available": true,
    "evaluator_configured": true,
    "evaluator_budget_remaining": 87
  },
  "sessions": {
    "total": 5,
    "active": 1
  },
  "memory": {
    "records": 1234,
    "daily_logs": 30
  },
  "web": {
    "port": 3100,
    "host": "localhost"
  }
}
```

### `POST /api/restart`

Restart the engine. The engine exits with code 75, signaling the process manager to respawn.

**Response (200):**
```json
{
  "ok": true,
  "message": "Engine restarting..."
}
```

The WebSocket connection will close. Clients should reconnect after a brief delay.

## Memory

### `GET /api/memory`

List memory files.

**Response (200):**
```json
{
  "files": [
    {"name": "SOUL.md", "size": 1234, "protected": "read_only"},
    {"name": "IDENTITY.md", "size": 567, "protected": "read_only"},
    {"name": "MEMORY.md", "size": 2345, "protected": "write_tier1_min"},
    {"name": "USER.md", "size": 890, "protected": "write_tier1_min"}
  ]
}
```

### `GET /api/memory/:name`

Read a specific memory file.

**Response (200):**
```json
{
  "name": "MEMORY.md",
  "content": "# Session Memory\n\n- User prefers concise responses...",
  "protected": "write_tier1_min"
}
```

### `GET /api/memory/search`

Search semantic memory.

**Query parameters:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `q` | string | required | Search query |
| `limit` | int | `10` | Maximum results |
| `method` | string | `"hybrid"` | Search method: `"fts"` (full-text only), `"vector"` (vector only), `"hybrid"` (both) |

**Response (200):**
```json
{
  "results": [
    {
      "id": "01HWXYZ...",
      "content": "User deployed the app using Terraform...",
      "score": 0.87,
      "source": "daily_log",
      "timestamp": 1711929600
    }
  ]
}
```

## Audit

### `GET /api/audit`

Query the audit log.

**Query parameters:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `session_id` | string | — | Filter by session |
| `action_type` | string | — | Filter by action type |
| `decision` | string | — | Filter by Shield decision (`"ALLOW"` or `"BLOCK"`) |
| `limit` | int | `50` | Maximum entries |
| `offset` | int | `0` | Skip first N entries |

**Response (200):**
```json
{
  "entries": [
    {
      "id": "01HWXYZ...",
      "timestamp": 1711929600,
      "session_id": "01HWABC...",
      "action_type": "read_file",
      "arguments": {"path": "src/main.go"},
      "decision": "ALLOW",
      "tier": 0,
      "hash": "a1b2c3d4...",
      "prev_hash": "e5f6g7h8..."
    }
  ]
}
```

### `GET /api/audit/verify`

Verify the audit log hash chain integrity.

**Response (200):**
```json
{
  "valid": true,
  "entries_checked": 1234,
  "first_entry": "2024-03-15T10:00:00Z",
  "last_entry": "2024-04-03T14:30:00Z"
}
```

**Response (200, tampered):**
```json
{
  "valid": false,
  "entries_checked": 1234,
  "break_at": 567,
  "break_entry_id": "01HWXYZ...",
  "expected_hash": "a1b2c3d4...",
  "actual_hash": "x9y8z7w6..."
}
```

## MCP

### `GET /api/mcp/servers`

List configured MCP servers and their status.

**Response (200):**
```json
{
  "servers": [
    {
      "name": "filesystem",
      "transport": "stdio",
      "status": "connected",
      "tools": ["read_file", "write_file", "list_directory"]
    }
  ]
}
```

## Static Assets

| Path | Description |
|------|-------------|
| `GET /` | Web UI (Svelte SPA, served via `go:embed`) |
| `GET /assets/*` | Bundled JavaScript, CSS, fonts |

The entire web UI is embedded in the Go binary via `go:embed`. No external file serving, no CDN, no build step at deploy time.
