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
    enricher   *shield.MetadataEnricher
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
4. **Metadata enrichment** via `shield.MetadataEnricher`.
5. **CheckProtection** -- hardcoded file protection (see [protection.md](protection.md)).
6. **Emit ActionStarted** event.
7. **Audit: PROPOSED** entry.
8. **Shield.Evaluate** -- 3-tier security pipeline.
9. **Emit ShieldVerdict** event.
10. **Audit: EVALUATED** entry.
11. **Verify hash** -- recompute and compare (TOCTOU defense).
12. **Chronicle snapshot** -- workspace backup before mutation.
13. **IFC check** -- information flow control labels.
14. **Execute** -- via MCP client or built-in executor.
15. **Audit: EXECUTED/FAILED** entry.
16. **Broadcast ActionCompleted** (and ActionArtifact if applicable).
17. **Log to daily action log**.
18. **Return ToolResultDelivery** to Agent.

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

If no Agent is connected (e.g., it crashed), a fallback in-process pipeline (`processMessageCore`) can handle the request directly.

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
| `SystemExecutor` | system_info |
| `CalculateExecutor` | calculate |
| `FileFormatExecutor` | file_format operations |
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

## Shutdown

`Engine.Stop()` performs graceful shutdown:

1. Sets the shutdown flag.
2. Cancels background tasks (session title generation) with a 5-second grace period.
3. Shuts down the SubAgentManager.
4. Shuts down MCP server connections.
5. Calls `grpc.Server.GracefulStop()`.
6. Closes the audit logger.
7. Closes the database.

## Key Source Files

| File | Purpose |
|---|---|
| `internal/engine/engine.go` | Engine struct, New, Start, Stop, RunSession, SendMessage, handleToolProposal, ProcessMessageForWeb |
| `internal/engine/eventsender.go` | EventSender interface, PipelineEvent struct, event type constants |
| `internal/engine/grpc_sender.go` | grpcEventSender implementation |
| `internal/engine/broadcaster.go` | EventBroadcaster fan-out |
| `internal/engine/protection.go` | CheckProtection, file protection levels |
| `internal/engine/redactor.go` | StreamingRedactor for secret masking |
| `internal/engine/verifier.go` | Verifier for TOCTOU hash checks |
| `internal/engine/executors/registry.go` | Executor registry |
| `internal/engine/executors/groups.go` | Tool group management |
| `internal/engine/executors/executor.go` | Executor interface |
