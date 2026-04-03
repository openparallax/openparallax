# gRPC API Reference

OpenParallax defines three gRPC services in `proto/openparallax/v1/pipeline.proto`. These services handle all communication between the Engine, Agent, and external clients.

## Services Overview

| Service | Purpose | Stream Type |
|---------|---------|-------------|
| `AgentService` | Engine ↔ Agent communication | Bidirectional streaming |
| `ClientService` | Engine ↔ Clients (CLI, web, channels) | Server streaming + unary |
| `SubAgentService` | Engine ↔ Sub-Agent processes | Bidirectional streaming |

## AgentService

The core bidirectional stream between the Engine and the sandboxed Agent process. This is the backbone of the system — every LLM token, tool proposal, and execution result flows through this stream.

### `RunSession`

```protobuf
rpc RunSession(stream AgentEvent) returns (stream EngineDirective);
```

**Bidirectional stream.** The Agent sends `AgentEvent` messages upstream; the Engine sends `EngineDirective` messages downstream.

#### AgentEvent (Agent → Engine)

```protobuf
message AgentEvent {
  oneof event {
    AgentReady agent_ready = 1;
    LLMTokenEmitted llm_token = 2;
    ToolCallProposed tool_call = 3;
    ToolDefsRequest tool_defs_request = 4;
    MemoryFlush memory_flush = 5;
    AgentResponseComplete response_complete = 6;
    AgentError error = 7;
  }
}
```

| Event | When Sent | Description |
|-------|-----------|-------------|
| `AgentReady` | Once, after connection | Agent has loaded config, applied sandbox, verified canary probes. Ready to process messages. Contains `canary_result` with sandbox verification status. |
| `LLMTokenEmitted` | During LLM streaming | A text token from the LLM. Contains `token` (string) and `message_id`. |
| `ToolCallProposed` | When LLM wants to use a tool | The Agent proposes a tool call. Contains `call_id`, `tool_name`, `arguments_json`. The Engine evaluates this through Shield before executing. |
| `ToolDefsRequest` | When Agent needs more tools | The Agent wants to load additional tool groups. Contains `groups` (string array of group names). The Engine responds with `ToolDefsDelivery`. |
| `MemoryFlush` | During context compaction | The Agent's compactor extracted facts that should be persisted to memory. Contains `content` (the compacted facts). |
| `AgentResponseComplete` | When response is done | All tool calls resolved, final text generated. Contains `content` (full response text), `thoughts` (reasoning trace), `token_usage`. |
| `AgentError` | On error | Something went wrong in the Agent. Contains `code` and `message`. |

#### EngineDirective (Engine → Agent)

```protobuf
message EngineDirective {
  oneof directive {
    ProcessRequest process_request = 1;
    ToolResultDelivery tool_result = 2;
    ToolDefsDelivery tool_defs = 3;
    ShutdownDirective shutdown = 4;
  }
}
```

| Directive | When Sent | Description |
|-----------|-----------|-------------|
| `ProcessRequest` | When a user sends a message | Contains `session_id`, `message_id`, `content`, `mode` (normal/otr), `history` (previous messages), `tools` (available tool definitions). The Agent starts a reasoning loop for this request. |
| `ToolResultDelivery` | After Engine executes a tool | Contains `call_id` (matching the proposal), `content` (execution result), `is_error`. The Agent feeds this back to the LLM for the next round. |
| `ToolDefsDelivery` | After Agent requests tool defs | Contains `call_id` and `content` (JSON array of tool definitions). Response to `ToolDefsRequest`. |
| `ShutdownDirective` | When Engine is shutting down | Contains `reason`. Agent should clean up and exit. |

#### Lifecycle

```
Agent connects → sends AgentReady (with canary result)
                     ↓
Engine validates canary → if sandbox failed, terminates Agent
                     ↓
Engine sends ProcessRequest (user message)
                     ↓
Agent streams LLMTokenEmitted events
Agent sends ToolCallProposed events
                     ↓
Engine evaluates proposal through Shield
Engine executes (or blocks) the action
Engine sends ToolResultDelivery
                     ↓
Agent feeds result to LLM, continues loop
                     ↓
Agent sends AgentResponseComplete
                     ↓
Wait for next ProcessRequest...
```

## ClientService

Server-streaming and unary RPCs for external clients (CLI via gRPC, web via WebSocket adapter).

### `SendMessage`

```protobuf
rpc SendMessage(ClientMessageRequest) returns (stream PipelineEvent);
```

**Server-streaming.** Client sends a message, receives a stream of pipeline events until the response is complete.

```protobuf
message ClientMessageRequest {
  string session_id = 1;
  string message_id = 2;
  string content = 3;
  string mode = 4;  // "normal" or "otr"
}
```

The response stream contains `PipelineEvent` messages — the protobuf representation of the 8 event types documented in [Event Types](/reference/events).

### `ListSessions`

```protobuf
rpc ListSessions(ListSessionsRequest) returns (ListSessionsResponse);
```

**Unary.** Returns all sessions with metadata.

```protobuf
message ListSessionsRequest {
  bool include_otr = 1;  // Whether to include OTR sessions
}

message ListSessionsResponse {
  repeated SessionInfo sessions = 1;
}

message SessionInfo {
  string id = 1;
  string title = 2;
  string mode = 3;      // "normal" or "otr"
  int64 created_at = 4;  // Unix timestamp
  int64 updated_at = 5;
  int32 message_count = 6;
}
```

### `GetHistory`

```protobuf
rpc GetHistory(GetHistoryRequest) returns (GetHistoryResponse);
```

**Unary.** Returns the message history for a session.

```protobuf
message GetHistoryRequest {
  string session_id = 1;
  int32 limit = 2;       // Max messages to return (0 = all)
  int32 offset = 3;      // Skip first N messages
}

message GetHistoryResponse {
  repeated ChatMessage messages = 1;
}

message ChatMessage {
  string id = 1;
  string role = 2;        // "user" or "assistant"
  string content = 3;
  int64 timestamp = 4;
  repeated Thought thoughts = 5;  // Only for assistant messages
  TokenUsage token_usage = 6;
}

message Thought {
  string stage = 1;    // "reasoning" or "tool_call"
  string summary = 2;  // Human-readable summary
  bytes detail = 3;    // JSON with additional data
}

message TokenUsage {
  int32 input_tokens = 1;
  int32 output_tokens = 2;
  int32 total_tokens = 3;
}
```

### `ResolveApproval`

```protobuf
rpc ResolveApproval(ApprovalRequest) returns (ApprovalResponse);
```

**Unary.** For future use — human-in-the-loop approval for high-risk actions.

## SubAgentService

Handles communication with sub-agent processes — lightweight agents spawned for parallel tasks.

### `RunSubAgent`

```protobuf
rpc RunSubAgent(stream AgentEvent) returns (stream EngineDirective);
```

Same message types as `AgentService.RunSession`, but for sub-agents. Sub-agents have:
- Reduced tool access (no sub-agent spawning — prevents recursion)
- Shared session context with the parent agent
- Independent reasoning loop via `agent.RunLoop`
- Timeout enforcement from `agents.timeout` config

## Common Types

### PipelineEvent

```protobuf
message PipelineEvent {
  string type = 1;         // Event type string (matches EventType enum)
  string session_id = 2;
  string message_id = 3;
  bytes data = 4;          // JSON-encoded event payload
  int64 timestamp = 5;     // Unix nanoseconds
}
```

### ToolDef

```protobuf
message ToolDef {
  string name = 1;
  string description = 2;
  bytes input_schema = 3;  // JSON Schema for tool parameters
}
```

## Proto File Location

```
proto/openparallax/v1/
├── pipeline.proto    # AgentService, ClientService, SubAgentService, PipelineEvent
├── shield.proto      # ShieldService (3 RPCs) — used by standalone Shield proxy
└── types.proto       # Shared enums and messages
```

Generate Go code with:

```bash
make proto
```

This runs `protoc` with `protoc-gen-go` and `protoc-gen-go-grpc` plugins. Generated code lives in `internal/types/pb/`.

## Connection Details

- **Agent → Engine**: Connects to `localhost:<dynamic-port>`. The port is passed to the Agent process via a command-line flag. TLS is not used for localhost connections.
- **CLI → Engine**: Connects to `localhost:<dynamic-port>`. Port is read from the agent registry (`~/.openparallax/registry.json`).
- **Web → Engine**: WebSocket at `ws://localhost:<web-port>/ws`. REST at `http://localhost:<web-port>/api/*`. The web server bridges WebSocket events to `ClientService` internally.
