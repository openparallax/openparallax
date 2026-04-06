# Event Types

OpenParallax uses an event-driven architecture for real-time communication between the Engine and its clients (CLI, web UI, channel adapters). Every significant action in the message pipeline emits a `PipelineEvent` that clients receive through their transport (gRPC stream or WebSocket).

Source: [`internal/engine/eventsender.go`](https://github.com/openparallax/openparallax/blob/main/internal/engine/eventsender.go)

## EventSender Interface

All transports implement the same interface:

```go
type EventSender interface {
    SendEvent(event *PipelineEvent) error
}
```

Implementations: `grpcEventSender` (protobuf over gRPC for CLI), `wsEventSender` (JSON over WebSocket for web UI), and any custom channel adapter.

## PipelineEvent Envelope

Every event is wrapped in a `PipelineEvent` with routing metadata:

| Field | Type | Description |
|-------|------|-------------|
| `session_id` | `string` | Session this event belongs to |
| `message_id` | `string` | Message that triggered this event |
| `type` | `string` | Event type identifier (see below) |

Exactly one payload field is set per event, determined by the `type` field.

## Core Events

These 8 events cover the standard message pipeline.

### `llm_token`

A single streamed token from the LLM response. Emitted continuously during text generation.

| Field | Type | Description |
|-------|------|-------------|
| `text` | `string` | The token text fragment |

JSON key: `text`

### `action_started`

Signals that a tool call is beginning execution.

| Field | Type | Description |
|-------|------|-------------|
| `tool_name` | `string` | Name of the tool being executed |
| `summary` | `string` | Human-readable description of the action |

### `shield_verdict`

Carries the Shield security evaluation result for a proposed action.

| Field | Type | Description |
|-------|------|-------------|
| `tool_name` | `string` | Tool that was evaluated |
| `decision` | `string` | Verdict: `"ALLOW"`, `"BLOCK"`, or `"ESCALATE"` |
| `tier` | `int` | Shield tier that made the decision (0, 1, or 2) |
| `confidence` | `float64` | Confidence score (0.0 - 1.0) |
| `reasoning` | `string` | Explanation of the verdict |

### `action_completed`

Signals that a tool call finished execution.

| Field | Type | Description |
|-------|------|-------------|
| `tool_name` | `string` | Tool that completed |
| `success` | `bool` | Whether the action succeeded |
| `summary` | `string` | Human-readable result summary |

### `response_complete`

Emitted once at the end of a message pipeline cycle. Carries the full assistant response and any extended thinking content.

| Field | Type | Description |
|-------|------|-------------|
| `content` | `string` | Full assistant response text |
| `thoughts` | `[]Thought` | Extended thinking content (optional) |

### `otr_blocked`

Emitted when an action is blocked because the session is in OTR mode.

| Field | Type | Description |
|-------|------|-------------|
| `reason` | `string` | User-facing explanation of why the action was blocked |

### `error`

Carries a pipeline error.

| Field | Type | Description |
|-------|------|-------------|
| `code` | `string` | Machine-readable error code |
| `message` | `string` | Human-readable error description |
| `recoverable` | `bool` | Whether the pipeline can continue after this error |

### `tier3_approval_required`

Requests human-in-the-loop approval for an action that Shield escalated to Tier 3.

| Field | Type | Description |
|-------|------|-------------|
| `action_id` | `string` | Unique ID for the pending action |
| `tool_name` | `string` | Tool awaiting approval |
| `reasoning` | `string` | Explanation of why approval is needed |
| `timeout_secs` | `int` | Seconds before auto-deny if no response |

JSON key: `tier3_approval`

## Sub-Agent Events

These 5 events track the lifecycle of sub-agents spawned via the `create_agent` action.

### `sub_agent_spawned`

A new sub-agent has been created and is starting work.

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Sub-agent name |
| `task` | `string` | Task description assigned to the sub-agent |
| `tool_groups` | `[]string` | Tool groups available to the sub-agent (optional) |

### `sub_agent_progress`

Periodic progress update from a running sub-agent.

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Sub-agent name |
| `llm_calls` | `int` | Number of LLM calls made so far |
| `tool_calls` | `int` | Number of tool calls made so far |
| `elapsed_ms` | `int64` | Milliseconds since the sub-agent started |

### `sub_agent_completed`

A sub-agent finished its task successfully.

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Sub-agent name |
| `result` | `string` | Task result or summary |
| `duration_ms` | `int64` | Total execution time in milliseconds |

### `sub_agent_failed`

A sub-agent encountered an error and could not complete.

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Sub-agent name |
| `error` | `string` | Error description |

### `sub_agent_cancelled`

A sub-agent was terminated by the user or system.

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Sub-agent name |

## Event Flow

Events are emitted in the following order during a typical message pipeline cycle:

```
User message arrives
  -> llm_token (streamed, many)
  -> action_started
  -> shield_verdict
  -> action_completed
  -> ... (repeat for each tool call, up to 25 rounds)
  -> llm_token (streamed, many)
  -> response_complete
```

For sub-agent actions:

```
action_started (create_agent)
  -> shield_verdict
  -> action_completed
  -> sub_agent_spawned
  -> sub_agent_progress (periodic)
  -> sub_agent_completed | sub_agent_failed | sub_agent_cancelled
```

## WebSocket Transport

Over WebSocket (`ws://localhost:3100/ws`), events are JSON-encoded `PipelineEvent` objects. Clients filter events by `session_id` to avoid cross-session contamination. The `log_entry` event type (used for live log broadcast) is global and processed before the session filter.

## gRPC Transport

Over gRPC, events are protobuf-encoded via the `PipelineService.StreamEvents` RPC. See the [gRPC API reference](/reference/grpc-api) for protobuf definitions.
