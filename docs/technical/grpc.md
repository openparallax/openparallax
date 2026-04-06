# gRPC Services

OpenParallax defines three gRPC services in `proto/openparallax/v1/pipeline.proto`. Generated Go code lives in `internal/types/pb/`. All three services are registered on a single gRPC server in the Engine process.

## AgentService

The AgentService handles the bidirectional stream between the Engine and the sandboxed Agent process. The Agent calls the LLM and proposes tool calls; the Engine evaluates them through Shield and executes allowed tools.

### RunSession

```protobuf
rpc RunSession(stream AgentEvent) returns (stream EngineDirective);
```

A persistent bidirectional stream for the Agent's lifetime. The Engine sends directives; the Agent sends events.

#### EngineDirective

The Engine sends one of four directive types:

```protobuf
message EngineDirective {
  oneof directive {
    ProcessRequest process = 1;
    ToolResultDelivery tool_result = 2;
    ToolDefsDelivery tool_defs = 3;
    ShutdownDirective shutdown = 4;
  }
}
```

**ProcessRequest**: Tells the Agent to process a user message.

```protobuf
message ProcessRequest {
  string session_id = 1;
  string message_id = 2;
  string content = 3;
  SessionMode mode = 4;    // NORMAL or OTR
  string source = 5;       // "cli", "web", "telegram", etc.
}
```

**ToolResultDelivery**: Returns the result of a tool the Agent proposed.

```protobuf
message ToolResultDelivery {
  string call_id = 1;      // Matches the tool call ID
  string content = 2;      // Tool output or error message
  bool is_error = 3;       // True if the tool call was blocked or failed
}
```

**ToolDefsDelivery**: Sends available tool definitions when the Agent requests them via `load_tools`.

```protobuf
message ToolDefsDelivery {
  repeated ToolDef tools = 1;
}

message ToolDef {
  string name = 1;
  string description = 2;
  string parameters_json = 3;  // JSON Schema for input parameters
  string group = 4;            // Group this tool belongs to
}
```

**ShutdownDirective**: Tells the Agent to shut down gracefully.

```protobuf
message ShutdownDirective {
  string reason = 1;
}
```

#### AgentEvent

The Agent sends one of seven event types:

```protobuf
message AgentEvent {
  oneof event {
    AgentReady ready = 1;
    LLMTokenEmitted llm_token_emitted = 2;
    ToolCallProposed tool_proposal = 3;
    ToolDefsRequest tool_defs_request = 4;
    MemoryFlush memory_flush = 5;
    AgentResponseComplete response_complete = 6;
    AgentError agent_error = 7;
  }
}
```

**AgentReady**: Sent once on stream open. Signals the Agent is initialized and waiting for directives.

```protobuf
message AgentReady {
  string agent_id = 1;   // Agent display name (e.g. "Atlas")
}
```

**LLMTokenEmitted**: A single streaming token from the LLM. Emitted as the LLM generates text.

```protobuf
message LLMTokenEmitted {
  string session_id = 1;
  string message_id = 2;
  string text = 3;          // Token text (may be partial word)
}
```

**ToolCallProposed**: The LLM wants to call a tool. The Engine must evaluate and execute it, then send back a `ToolResultDelivery`.

```protobuf
message ToolCallProposed {
  string session_id = 1;
  string message_id = 2;
  string call_id = 3;        // Unique call ID (matches ToolResultDelivery.call_id)
  string tool_name = 4;      // Tool name (e.g. "read_file", "execute_command")
  string arguments_json = 5; // JSON-encoded arguments
}
```

**ToolDefsRequest**: The Agent's `load_tools` meta-tool was invoked. The Engine resolves the requested groups and sends back `ToolDefsDelivery`.

```protobuf
message ToolDefsRequest {
  repeated string groups = 1;  // e.g. ["files", "shell", "git"]
}
```

**MemoryFlush**: Compaction extracted facts that should be persisted to MEMORY.md.

```protobuf
message MemoryFlush {
  string content = 1;   // Facts to append to MEMORY.md
}
```

**AgentResponseComplete**: The Agent finished processing a message.

```protobuf
message AgentResponseComplete {
  string session_id = 1;
  string message_id = 2;
  string content = 3;           // Full response text
  repeated Thought thoughts = 4; // Reasoning and tool call traces
  TokenUsage usage = 5;         // Token consumption stats
}

message Thought {
  string stage = 1;        // "reasoning" or "tool_call"
  string summary = 2;      // Human-readable summary
  string detail_json = 3;  // Optional structured detail
}

message TokenUsage {
  int32 input_tokens = 1;
  int32 output_tokens = 2;
  int32 cache_read_tokens = 3;
  int32 cache_write_tokens = 4;
}
```

**AgentError**: An error occurred during processing.

```protobuf
message AgentError {
  string session_id = 1;
  string message_id = 2;
  string code = 3;           // e.g. "CONTEXT_FAILED", "LLM_CALL_FAILED", "STREAM_ERROR"
  string message = 4;
  bool recoverable = 5;
}
```

### Stream Lifecycle

```
Agent connects
    |
    v
Agent sends AgentReady
    |
    v
Engine stores stream reference
    |
    v
Engine sends ProcessRequest (when user message arrives)
    |
    v
Agent streams LLMTokenEmitted events
    |
    v
Agent sends ToolCallProposed
    |
    v
Engine evaluates, executes, sends ToolResultDelivery
    |
    v
Agent feeds result to LLM, continues loop
    |
    v
Agent sends AgentResponseComplete
    |
    (repeats for next message)
    |
    v
Engine sends ShutdownDirective (or Agent stream closes on crash/exit)
```

## ClientService

The ClientService handles external client connections. The TUI, web UI (via gRPC), and channel adapters use this service.

### SendMessage

```protobuf
rpc SendMessage(ClientMessageRequest) returns (stream PipelineEvent);
```

Server-streaming RPC. The client sends one message and receives a stream of pipeline events. Events are delivered via the `EventBroadcaster` -- the Engine subscribes the client's stream as an `EventSender` for the session.

```protobuf
message ClientMessageRequest {
  string content = 1;
  string session_id = 2;
  SessionMode mode = 3;
  string source = 4;
}
```

### PipelineEvent

The `PipelineEvent` message carries one of many event types:

```protobuf
message PipelineEvent {
  string session_id = 1;
  string message_id = 2;
  PipelineEventType event_type = 3;

  // One payload per event type
  LLMToken llm_token = 17;
  ShieldVerdict shield_verdict = 13;
  ActionStarted action_started = 14;
  ActionCompleted action_completed = 15;
  ResponseComplete response_complete = 18;
  ApprovalNeeded approval_needed = 19;
  OTRBlocked otr_blocked = 20;
  PipelineError pipeline_error = 21;
  // ... and more
}
```

### ResolveApproval

```protobuf
rpc ResolveApproval(ApprovalResponse) returns (ApprovalAck);
```

Responds to a Tier 3 human-in-the-loop approval request.

### Other RPCs

```protobuf
rpc GetStatus(StatusRequest) returns (StatusResponse);
rpc ListSessions(ListSessionsRequest) returns (ListSessionsResponse);
rpc GetHistory(GetHistoryRequest) returns (GetHistoryResponse);
rpc Shutdown(ShutdownRequest) returns (ShutdownResponse);
```

These support the TUI for displaying status, listing sessions, and loading conversation history.

## SubAgentService

The SubAgentService handles sub-agent process communication. Sub-agents are spawned by the main agent (via `spawn_agent` tool) and follow the same pattern: sandboxed, call LLM, propose tools.

### RegisterSubAgent

```protobuf
rpc RegisterSubAgent(SubAgentRegisterRequest) returns (SubAgentRegisterResponse);
```

Authenticates a sub-agent process and returns its task assignment, available tools, and LLM configuration.

```protobuf
message SubAgentRegisterRequest {
  string token = 1;    // Authentication token
}

message SubAgentRegisterResponse {
  string name = 1;
  string task = 2;
  repeated SubAgentToolDef tools = 3;
  string system_prompt = 4;
  string model = 5;
  string provider = 6;
  string api_key_env = 7;
  string base_url = 8;
  int32 max_llm_calls = 9;
}
```

### SubAgentExecuteTool

```protobuf
rpc SubAgentExecuteTool(SubAgentToolRequest) returns (SubAgentToolResponse);
```

Forwards a tool call from a sub-agent to the Engine for evaluation and execution through the same Shield pipeline as the main agent.

### SubAgentComplete / SubAgentFailed

```protobuf
rpc SubAgentComplete(SubAgentCompleteRequest) returns (SubAgentCompleteResponse);
rpc SubAgentFailed(SubAgentFailedRequest) returns (SubAgentFailedResponse);
rpc SubAgentPollMessage(SubAgentPollRequest) returns (SubAgentPollResponse);
```

Report task completion or failure. The Engine broadcasts corresponding events to connected clients.

## Shared Types

Defined in `proto/openparallax/v1/types.proto`:

- `SessionMode`: NORMAL = 0, OTR = 1, HEARTBEAT = 3
- `ActionType`: Enumeration of all action types (50+)
- `GoalType`: Intent categories
- `VerdictDecision`: ALLOW, BLOCK, ESCALATE
- `ActionRequest`: Full action description with payload
- `Verdict`: Shield evaluation result
- `ActionResult`: Tool execution outcome with output, error, and summary

## Proto File Locations

| File | Contents |
|---|---|
| `proto/openparallax/v1/pipeline.proto` | AgentService, ClientService, SubAgentService, all request/response messages |
| `proto/openparallax/v1/shield.proto` | ShieldService (standalone Shield gRPC service) |
| `proto/openparallax/v1/types.proto` | Shared enums, ActionRequest, Verdict, ActionResult |

Generated Go code: `internal/types/pb/`

Generate with: `make proto`
