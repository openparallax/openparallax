# Engine Internals

The Engine is the privileged, unsandboxed process that orchestrates all system operations. It runs the gRPC server, HTTP/WebSocket server, Shield security pipeline, executor registry, audit logger, chronicle, and memory manager. The Agent proposes actions; the Engine evaluates and executes them.

## Initialization

`engine.New(configPath, verbose)` creates the Engine. The constructor:

1. Loads config from YAML.
2. Creates a structured JSON logger (to `.openparallax/engine.log`).
3. Initializes the LLM provider.
4. Opens the SQLite database (WAL mode, pure Go via modernc.org/sqlite).
5. Reads the canary token from the workspace.
6. Creates the executor registry with all available executors.
7. Initializes the Shield pipeline (policy file, ONNX threshold, evaluator config).
8. Creates the audit logger (append-only JSONL with hash chain).
9. Creates the chronicle (copy-on-write snapshots).
10. Creates the memory manager (FTS5 + vector search).
11. Initializes the MCP manager (if external servers are configured).
12. Creates the EventBroadcaster for fan-out.

```go
type Engine struct {
    cfg        *types.AgentConfig
    llm        llm.Provider
    log        *logging.Logger
    agent      *agent.Agent
    executors  *executors.Registry
    shield     *shield.Pipeline
    ifcPolicy  *ifc.Policy
    chronicle  *chronicle.Chronicle
    memory     *memory.Manager
    audit      *audit.Logger
    verifier   *Verifier
    db         *storage.DB
    mcpManager *mcp.Manager
    tier3Manager    *Tier3Manager
    subAgentManager *SubAgentManager
    broadcaster     *EventBroadcaster
    server   *grpc.Server
    listener net.Listener
    agentStream pb.AgentService_RunSessionServer
    // ...
}
```

## gRPC Server

### Start

`Engine.Start(listenPort ...int)` binds a TCP listener and registers three gRPC services:

```go
e.server = grpc.NewServer()
pb.RegisterAgentServiceServer(e.server, e)
pb.RegisterClientServiceServer(e.server, e)
pb.RegisterSubAgentServiceServer(e.server, e)
```

If a specific port is requested but unavailable, the server falls back to a dynamic port (`:0`). The actual port is read from the listener's address and returned to the caller, which writes it to stdout as `PORT:<port>`.

### RunSession (AgentService)

`RunSession` is the bidirectional stream between the Engine and the Agent. This is the core communication channel. The Engine acts as the server; the Agent connects as a client.

The handler:

1. Waits for `AgentReady` as the first event.
2. Stores the stream reference so other methods can forward messages to the Agent.
3. Enters a read loop processing `AgentEvent` messages.

```go
func (e *Engine) RunSession(stream pb.AgentService_RunSessionServer) error {
    // Wait for AgentReady
    firstEvent, _ := stream.Recv()
    ready := firstEvent.GetReady()

    // Store stream for message forwarding
    e.agentStream = stream

    // Process agent events
    for {
        event, _ := stream.Recv()
        switch ev := event.Event.(type) {
        case *pb.AgentEvent_LlmTokenEmitted:
            e.broadcaster.Broadcast(...)
        case *pb.AgentEvent_ToolProposal:
            result := e.handleToolProposal(ctx, ev.ToolProposal)
            stream.Send(&pb.EngineDirective{
                Directive: &pb.EngineDirective_ToolResult{ToolResult: result},
            })
        case *pb.AgentEvent_ToolDefsRequest:
            // Resolve tool groups, send definitions back
        case *pb.AgentEvent_MemoryFlush:
            // Persist compaction facts to MEMORY.md
        case *pb.AgentEvent_ResponseComplete:
            // Store message, broadcast, generate title
        case *pb.AgentEvent_AgentError:
            e.broadcaster.Broadcast(...)
        }
    }
}
```

Event routing:

| AgentEvent | Engine action |
|---|---|
| `LlmTokenEmitted` | Broadcast as `llm_token` to session subscribers |
| `ToolCallProposed` | Run `handleToolProposal`, send `ToolResultDelivery` back |
| `ToolDefsRequest` | Resolve tool groups, send `ToolDefsDelivery` back |
| `MemoryFlush` | Append content to MEMORY.md |
| `AgentResponseComplete` | Store message, broadcast `response_complete`, generate title |
| `AgentError` | Broadcast as `error` event |

### SendMessage (ClientService)

`SendMessage` is the entry point for gRPC clients (the TUI). It is a server-streaming RPC: the client sends one `ClientMessageRequest` and receives a stream of `PipelineEvent` messages.

```go
func (e *Engine) SendMessage(req *pb.ClientMessageRequest, stream pb.ClientService_SendMessageServer) error {
    // Store user message
    e.storeMessage(sid, mid, "user", req.Content)

    // Subscribe this client for events
    sender := newGRPCEventSender(stream)
    e.broadcaster.Subscribe(clientID, sid, sender)
    defer e.broadcaster.Unsubscribe(clientID)

    // Forward to Agent
    e.forwardToAgent(sid, mid, req.Content, mode, req.Source)

    // Block until client disconnects
    <-stream.Context().Done()
    return nil
}
```

The method blocks on `stream.Context().Done()`. Events reach the client through the broadcaster: as the Agent processes the message and emits events, the Engine broadcasts them to all subscribers for that session, including this gRPC client.

### forwardToAgent

`forwardToAgent` sends a `ProcessRequest` directive to the Agent via the stored `agentStream`:

```go
func (e *Engine) forwardToAgent(sid, mid, content string, mode types.SessionMode, source string) error {
    return e.agentStream.Send(&pb.EngineDirective{
        Directive: &pb.EngineDirective_Process{
            Process: &pb.ProcessRequest{
                SessionId: sid,
                MessageId: mid,
                Content:   content,
                Mode:      pbMode,
                Source:     source,
            },
        },
    })
}
```

## handleToolProposal

This is the security-critical path. Every tool call proposed by the Agent passes through this method before execution. The full flow is documented in [pipeline.md](pipeline.md). Key steps:

1. **Parse arguments** from JSON.
2. **Build ActionRequest** with a unique ID, type, payload, and timestamp.
3. **Compute action hash** (SHA-256 of canonicalized action).
4. **IFC classification** via `ifc.Policy.Classify()`.
5. **CheckProtection** -- hardcoded file protection (see [protection.md](protection.md)).
6. **Emit ActionStarted** event.
7. **Audit: PROPOSED** entry.
8. **Shield.Evaluate** -- 4-tier security pipeline.
9. **Emit ShieldVerdict** event.
10. **Audit: EVALUATED** entry.
11. **Verify hash** -- recompute and compare (TOCTOU defense).
12. **Chronicle snapshot** -- workspace backup before mutation.
13. **IFC check** -- information flow control labels.
14. **Execute** -- via MCP client or built-in executor.
15. **Audit: EXECUTED/FAILED** entry.
16. **Broadcast ActionCompleted**.
17. **Return ToolResultDelivery** to Agent.

## ProcessMessageForWeb

`ProcessMessageForWeb` is the public entry point for the web server and any channel adapter:

```go
func (e *Engine) ProcessMessageForWeb(ctx context.Context, sender EventSender,
    sid, mid, content string, mode types.SessionMode) error
```

It:
1. Stores the user message.
2. Subscribes the sender for events on the session.
3. Forwards to the Agent via `forwardToAgent`.
4. Blocks on `ctx.Done()` (the WebSocket connection or channel adapter manages the context lifetime).
5. Unsubscribes the sender on return.

All message processing routes through the sandboxed Agent via the gRPC bidirectional stream (RunSession).

## EventBroadcaster

The `EventBroadcaster` manages fan-out of pipeline events to subscribed clients. See [event-system.md](event-system.md) for details.

## Executor Registry

The `executors.Registry` maps `ActionType` values to `Executor` implementations. It is populated during Engine initialization:

| Executor | Actions |
|---|---|
| `FileExecutor` | read_file, write_file, delete_file, list_directory, search_files, etc. |
| `ShellExecutor` | execute_command |
| `GitExecutor` | git_status, git_diff, git_log, git_commit, etc. |
| `HTTPExecutor` | http_request |
| `ScheduleExecutor` | create_schedule, list_schedules, delete_schedule |
| `CanvasExecutor` | canvas_create, canvas_update, canvas_project, canvas_preview |
| `BrowserExecutor` | browser_navigate, browser_extract, browser_screenshot |
| `EmailExecutor` | send_email, search_email (requires config) |
| `CalendarExecutor` | read_calendar, create_event, update_event (requires config) |
| `MemoryExecutor` | memory_search, memory_write, memory_read |
| `SystemExecutor` | system_info, clipboard, open, notify, screenshot (per-tool availability) |
| `FileFormatExecutor` | archive, pdf_read, spreadsheet operations |
| `SubAgentExecutor` | spawn_agent, list_agents, cancel_agent |

The `Execute` method dispatches to the appropriate executor:

```go
func (r *Registry) Execute(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
    executor, ok := r.executors[action.Type]
    if !ok {
        return &types.ActionResult{Success: false, Error: "unknown action type"}
    }
    return executor.Execute(ctx, action)
}
```

### Tool Groups

Tools are organized into groups for lazy loading. The `GroupRegistry` manages groups and provides the `load_tools` meta-tool definition. Groups include: `files`, `shell`, `git`, `web`, `email`, `calendar`, `memory`, `agents`, and others. The Agent calls `load_tools({"groups": ["files", "shell"]})` to load specific groups.

### Sub-Agent Tool Filtering

Sub-agents receive their full tool set at spawn time (no `load_tools` meta-tool). Three categories are excluded:

**Excluded groups:** `agents`, `schedule`, `memory` — sub-agents cannot spawn other agents, create schedules, or write to memory.

**Excluded tools:** `load_tools` (would deadlock — the sub-agent callback ignores `EventToolDefsRequest`), `create_agent`, `agent_status`, `agent_result`, `agent_message`, `delete_agent`, `list_agents`.

## Shutdown

`Engine.Stop()` performs graceful shutdown:

1. Sets the shutdown flag.
2. Cancels background tasks (session title generation) with a 5-second grace period.
3. Shuts down the SubAgentManager.
4. Shuts down MCP server connections.
5. Calls `grpc.Server.GracefulStop()`.
6. Closes the audit logger.
7. Closes the database.

## Engine Accessors

The Engine exposes read-only accessors used by the web server, CLI, and channel adapters:

| Method | Return Type | Description |
|--------|-------------|-------------|
| `ShieldStatus()` | `map[string]any` | Shield state: active, tier2_used, tier2_budget, tier2_enabled |
| `SandboxStatus()` | `map[string]any` | Kernel sandbox state: active, mode, version, filesystem, network |
| `OAuthManager()` | `*oauth.Manager` | OAuth2 token manager (nil if not configured) |
| `SubAgentManager()` | `*SubAgentManager` | Sub-agent lifecycle manager |
| `MCPServerStatus()` | `[]map[string]any` | Status of all configured MCP servers |
| `Broadcaster()` | `*EventBroadcaster` | Event fan-out for subscribing clients |
| `Tier3()` | `*Tier3Manager` | Human-in-the-loop approval manager |
| `Audit()` | `*audit.Logger` | Audit logger for security events |
| `ConfigPath()` | `string` | Path to config.yaml |
| `AuditPath()` | `string` | Path to audit.jsonl |
| `LogPath()` | `string` | Path to engine.log |

Mutation methods (called by slash commands and web handlers):

| Method | Description |
|--------|-------------|
| `UpdateIdentity(name, avatar)` | Update agent name/avatar in-memory |
| `UpdateShieldBudget(budget)` | Change daily Tier 2 budget in-memory |
| `SetAgentAuthToken(token)` | Set the ephemeral agent gRPC auth token |
| `SetSandboxStatus(...)` | Store sandbox probe results for API reporting |
| `SetupSubAgents(grpcAddr)` | Create sub-agent manager and register executor |

## Storage Schema

The Engine uses a single SQLite database (`<workspace>/.openparallax/openparallax.db`) in WAL mode. Tables:

| Table | Purpose |
|-------|---------|
| `sessions` | Session metadata (id, mode, title, timestamps) |
| `messages` | Chat messages with thoughts JSON |
| `memory_fts` | FTS5 virtual table for memory file search |
| `snapshots` | Chronicle snapshot metadata with hash chain |
| `transactions` | Chronicle transaction grouping |
| `audit_index` | Indexed audit entries for fast queries by session/type |
| `chunks` | Chunk-based memory index (path, line range, text, embedding) |
| `chunks_fts` | FTS5 virtual table over chunks |
| `embedding_cache` | Embedding cache by content hash (avoids re-embedding) |
| `file_hashes` | File content hashes for incremental indexing |
| `llm_usage` | Per-message LLM token usage (input, output, cache, rounds, duration) |
| `metrics_daily` | Daily aggregated metrics |
| `metrics_latency` | Per-observation latency samples for percentile queries |
| `oauth_tokens` | OAuth2 tokens encrypted at rest (AES-256-GCM) |

## Logging

The Engine uses structured JSON logging (`internal/logging/`). Every log entry is a JSON object with `timestamp`, `level`, `event`, and a `data` map.

**LogHook**: Any component can register a `LogHook` callback via `logger.AddHook(fn)`. The hook is called for every log entry after it is written to disk. The web server uses this to broadcast `log_entry` events to all connected WebSocket clients for the developer console.

## Key Source Files

| File | Purpose |
|---|---|
| `internal/engine/engine.go` | Engine struct, New, Start, Stop, accessors |
| `internal/engine/engine_pipeline.go` | RunSession, handleToolProposal, SendMessage |
| `internal/engine/engine_grpc.go` | GetStatus, Shutdown, memory/session RPCs |
| `internal/engine/engine_session.go` | Message storage, session management, title generation |
| `internal/engine/engine_tools.go` | ProcessMessageForWeb, processToolCall |
| `internal/engine/eventsender.go` | EventSender interface, PipelineEvent struct, event type constants |
| `internal/engine/grpc_sender.go` | grpcEventSender implementation |
| `internal/engine/broadcaster.go` | EventBroadcaster fan-out |
| `internal/engine/protection.go` | CheckProtection, file protection levels |
| `internal/engine/redactor.go` | StreamingRedactor for secret masking |
| `internal/engine/verifier.go` | Verifier for TOCTOU hash checks |
| `internal/engine/executors/registry.go` | Executor registry |
| `internal/engine/executors/groups.go` | Tool group management |
| `internal/engine/executors/executor.go` | Executor interface |
