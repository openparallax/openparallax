# REST API Reference

The Engine exposes a REST API on the web server port (default `:3100`). This API is used by the web UI and can be used by any HTTP client.

All endpoints return JSON. Authentication is required when the web server is bound to a non-localhost address (see [Configuration — Web](/reference/config#web)).

Source: [`internal/web/handlers.go`](https://github.com/openparallax/openparallax/blob/main/internal/web/handlers.go)

## Status

### `GET /api/status`

Returns system health information.

**Response:**

```json
{
  "agent_name": "Atlas",
  "agent_avatar": "",
  "model": "claude-sonnet-4-6",
  "session_count": 12,
  "workspace": "/home/user/.openparallax/atlas",
  "shield": { "active": true, "tier2_used": 5, "tier2_budget": 100 },
  "sandbox": { "active": true, "mode": "landlock", "filesystem": true, "network": true }
}
```

## Sessions

### `GET /api/sessions`

Lists all sessions ordered by most recent.

**Response:** Array of session objects.

### `POST /api/sessions`

Creates a new session.

**Request body:**

```json
{
  "mode": "normal"
}
```

`mode` is `"normal"` (default) or `"otr"`. OTR sessions are never persisted to the database.

**Response:** `201 Created` with the session object.

### `GET /api/sessions/{id}`

Returns a single session by ID.

**Response:** Session object or `404` if not found.

### `DELETE /api/sessions/{id}`

Deletes a session and all its messages.

**Response:** `204 No Content`.

### `PATCH /api/sessions/{id}`

Updates a session's title.

**Request body:**

```json
{
  "title": "New title"
}
```

**Response:**

```json
{ "status": "ok" }
```

### `GET /api/sessions/{id}/messages`

Returns all messages for a session.

**Response:** Array of message objects (empty array if session has no messages).

### `GET /api/sessions/search`

Full-text search across session content.

**Query parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `q` | `string` | Yes | Search query |

**Response:**

```json
{
  "results": [
    { "session_id": "...", "title": "...", "snippet": "..." }
  ]
}
```

## Tools

### `GET /api/tools`

Lists all available tools (across all groups).

**Response:** Array of tool objects with `name` and `description`.

## Settings

### `GET /api/settings`

Returns the current configuration (secrets masked).

**Response:**

```json
{
  "agent": { "name": "Atlas", "avatar": "" },
  "chat": { "provider": "anthropic", "model": "claude-sonnet-4-6", "api_key_configured": true, "base_url": "" },
  "shield": { "policy": "default", "evaluator": { "provider": "...", "model": "..." }, "tier2_budget": 100, "tier2_used_today": 5 },
  "memory": { "embedding": { "provider": "openai", "model": "text-embedding-3-small", "api_key_configured": true, "base_url": "" } },
  "mcp": { "servers": [...] },
  "email": { "provider": "smtp", "configured": true, "from": "...", "oauth_accounts": [...] },
  "calendar": { "provider": "google", "configured": true, "oauth_accounts": [...] },
  "web": { "port": 3100 },
  "sandbox": { "active": true, "mode": "landlock" }
}
```

### `PUT /api/settings`

Updates configuration fields. The handler flattens the nested JSON body into dot-paths, dispatches every key through the canonical `SettableKeys` registry, and persists via the same writer used by `/config set` and the CLI init wizard. Atomic across all keys in the body — either every key validates and the file is updated, or nothing changes.

**Request body:** Partial settings object (only include fields to change).

```json
{
  "agent": { "name": "NewName" },
  "chat": { "provider": "openai", "model": "gpt-5.4" }
}
```

**Accepted JSON paths** (mapped to canonical keys):

| JSON path | Canonical key | Restart? |
|---|---|---|
| `agent.name` | `identity.name` | Immediate |
| `agent.avatar` | `identity.avatar` | Immediate |
| `chat.provider` | `chat.provider` | Restart |
| `chat.model` | `chat.model` | Restart |
| `chat.api_key_env` | `chat.api_key_env` | Restart |
| `chat.base_url` | `chat.base_url` | Restart |
| `shield.evaluator.provider` | `shield.provider` | Restart |
| `shield.evaluator.model` | `shield.model` | Restart |
| `memory.embedding.provider` | `embedding.provider` | Restart |
| `memory.embedding.model` | `embedding.model` | Restart |

**Response:**

```json
{
  "success": true,
  "restart_required": true,
  "changed": ["chat.provider", "chat.model"],
  "immediate": [],
  "needs_restart": ["chat.provider", "chat.model"]
}
```

Identity changes take effect immediately on the live engine. Chat, Shield evaluator, and memory embedding changes require a restart.

**Errors:**

- `400 Bad Request` — unknown or read-only key (e.g. `web.port`, `shield.tier2_budget`, anything from the `general.*` block), value of the wrong type, or a model reference that doesn't resolve in the pool.
- `500 Internal Server Error` — `config.Save` failed (round-trip validation rejected the result; the on-disk file is left untouched).

Every successful save also creates a backup at `<workspace>/.openparallax/backups/config-<timestamp>.yaml` and rotates the directory to the 10 most recent.

### `POST /api/settings/test-mcp`

Tests an MCP server connection by starting it and discovering its tools.

**Request body:**

```json
{
  "name": "my-server",
  "command": "/path/to/server",
  "args": ["--flag"],
  "env": { "KEY": "value" }
}
```

**Response:**

```json
{
  "success": true,
  "tools": [{ "name": "tool_name", "description": "..." }]
}
```

Timeout: 15 seconds.

## Memory

### `GET /api/memory/search`

Searches memory via FTS5 full-text search.

**Query parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `q` | `string` | Yes | Search query |

**Response:** Array of search results with `path`, `section`, and `snippet` fields. Returns up to 10 results.

### `GET /api/memory/{type}`

Reads a specific memory file by type (e.g. `MEMORY.md`, `PREFS.md`).

**Response:**

```json
{
  "type": "MEMORY.md",
  "content": "..."
}
```

Returns `404` if the memory file does not exist.

## Logs

### `GET /api/logs`

Returns engine log entries (from `engine.log`).

**Query parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `lines` | `int` | `200` | Number of entries to return (max 1000) |
| `level` | `string` | — | Filter by level: `info`, `warn`, `error` |
| `event` | `string` | — | Filter by event type (substring match) |
| `offset` | `int` | `0` | Skip this many entries from the end (for pagination) |

**Response:**

```json
{
  "entries": [...],
  "total_lines": 1234,
  "has_more": true
}
```

## Audit

### `GET /api/audit`

Returns audit log entries with hash chain verification.

**Query parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `lines` | `int` | `100` | Number of entries to return |
| `offset` | `int` | `0` | Skip this many entries from the end (for pagination) |

**Response:**

```json
{
  "entries": [...],
  "total_entries": 567,
  "chain_valid": true,
  "has_more": false
}
```

If `chain_valid` is `false`, a `chain_break_at` field indicates the index where the break was detected.

## Engine Control

### `POST /api/restart`

Triggers an engine restart. The engine exits with code 75, and the process manager respawns it.

**Response:**

```json
{ "status": "restarting" }
```

## Sub-Agents

### `GET /api/sub-agents`

Lists all active sub-agents.

**Response:** Array of sub-agent status objects. Returns an empty array if no sub-agents are running.

## Metrics

### `GET /api/metrics`

Returns aggregated usage metrics for a time period.

**Query parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `period` | `string` | today | Period: `weekly`, `monthly`, `yearly`, or omit for today |

**Response:** Metrics summary object with token usage, daily metrics, and a `performance` block:

```json
{
  "performance": {
    "avg_latency_ms": 1240,
    "p50_latency_ms": 980,
    "p95_latency_ms": 2400,
    "p99_latency_ms": 3800,
    "shield_t0_p50_ms": 1,
    "shield_t0_p95_ms": 4,
    "shield_t1_p50_ms": 22,
    "shield_t1_p95_ms": 71,
    "shield_t2_p50_ms": 540,
    "shield_t2_p95_ms": 1820
  }
}
```

The top-level `*_latency_ms` fields are LLM call durations sourced from `llm_usage.duration_ms`. The `shield_t{0,1,2}_p{50,95}_ms` fields are per-tier Shield evaluation latencies sourced from the `metrics_latency` table; one sample is recorded per Shield evaluation. Zeros mean no samples for that tier in the date range.

### `GET /api/metrics/session/{id}`

Returns token usage for a specific session.

**Response:** Session token usage object.

### `GET /api/metrics/daily`

Returns daily token usage over a time range.

**Query parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `days` | `int` | `30` | Number of days to look back (max 365) |

**Response:** Array of daily token usage objects. Returns an empty array if no data exists.

## Channels

### `GET /api/channels`

Lists all active channel adapter names.

**Response:**

```json
{
  "channels": ["telegram", "discord"]
}
```

Returns an empty array if no channels are running or the channel controller is not available.

### `POST /api/channels/detach`

Gracefully detaches a running channel adapter.

**Request body:**

```json
{
  "channel": "telegram"
}
```

**Response:**

```json
{ "status": "detached", "channel": "telegram" }
```

Returns `400` if no channel name is provided, `404` if the channel is not found, or `503` if the channel controller is not available.

## WebSocket

### `GET /api/ws`

WebSocket endpoint for real-time event streaming. Events are JSON-encoded `PipelineEvent` objects. See the [Events reference](/reference/events) for payload details.

Messages are sent to the engine by writing JSON to the WebSocket:

```json
{
  "type": "message",
  "session_id": "...",
  "content": "Hello"
}
```

See the [WebSocket reference](/reference/websocket) for the full protocol.

## Error Format

All error responses use a consistent format:

```json
{
  "error": "description of what went wrong"
}
```

HTTP status codes follow standard conventions: `400` for bad requests, `404` for not found, `500` for server errors.
