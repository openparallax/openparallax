# Event System

The event system provides transport-neutral streaming of pipeline events from the Engine to connected clients. Events are emitted during message processing and delivered via gRPC streams, WebSocket connections, or any custom channel adapter.

## Event Types

8 core event types are defined in `internal/engine/eventsender.go`:

| Event Type | Constant | Payload Struct | Description |
|---|---|---|---|
| `llm_token` | `EventLLMToken` | `LLMTokenEvent` | Streaming text token from the LLM |
| `action_started` | `EventActionStarted` | `ActionStartedEvent` | Tool call is beginning execution |
| `shield_verdict` | `EventShieldVerdict` | `ShieldVerdictEvent` | Shield security evaluation result |
| `action_completed` | `EventActionCompleted` | `ActionCompletedEvent` | Tool call finished (success or failure) |
| `action_artifact` | `EventActionArtifact` | `ActionArtifactEvent` | Tool produced a viewable artifact |
| `response_complete` | `EventResponseComplete` | `ResponseCompleteEvent` | Full response with thoughts |
| `otr_blocked` | `EventOTRBlocked` | `OTRBlockedEvent` | Action blocked by OTR mode |
| `error` | `EventError` | `PipelineErrorEvent` | Pipeline stage error |

Additionally, 5 sub-agent event types are defined:

| Event Type | Constant | Payload Struct |
|---|---|---|
| `sub_agent_spawned` | `EventSubAgentSpawned` | `SubAgentSpawnedEvent` |
| `sub_agent_progress` | `EventSubAgentProgress` | `SubAgentProgressEvent` |
| `sub_agent_completed` | `EventSubAgentCompleted` | `SubAgentCompletedEvent` |
| `sub_agent_failed` | `EventSubAgentFailed` | `SubAgentFailedEvent` |
| `sub_agent_cancelled` | `EventSubAgentCancelled` | `SubAgentCancelledEvent` |

## PipelineEvent Structure

Every event is wrapped in a `PipelineEvent`:

```go
type PipelineEvent struct {
    SessionID string    `json:"session_id"`
    MessageID string    `json:"message_id"`
    Type      EventType `json:"type"`

    // Exactly one payload field is set per event.
    LLMToken         *LLMTokenEvent         `json:"text,omitempty"`
    ActionStarted    *ActionStartedEvent    `json:"action_started,omitempty"`
    ShieldVerdict    *ShieldVerdictEvent    `json:"shield_verdict,omitempty"`
    ActionCompleted  *ActionCompletedEvent  `json:"action_completed,omitempty"`
    ActionArtifact   *ActionArtifactEvent   `json:"action_artifact,omitempty"`
    ResponseComplete *ResponseCompleteEvent `json:"response_complete,omitempty"`
    OTRBlocked       *OTRBlockedEvent       `json:"otr_blocked,omitempty"`
    Error            *PipelineErrorEvent    `json:"error,omitempty"`

    // Sub-agent events
    SubAgentSpawned   *SubAgentSpawnedEvent   `json:"sub_agent_spawned,omitempty"`
    SubAgentProgress  *SubAgentProgressEvent  `json:"sub_agent_progress,omitempty"`
    SubAgentCompleted *SubAgentCompletedEvent `json:"sub_agent_completed,omitempty"`
    SubAgentFailed    *SubAgentFailedEvent    `json:"sub_agent_failed,omitempty"`
    SubAgentCancelled *SubAgentCancelledEvent `json:"sub_agent_cancelled,omitempty"`
}
```

## Payload Structures

### LLMTokenEvent

```go
type LLMTokenEvent struct {
    Text string `json:"text"`   // Token text (may be partial word)
}
```

### ActionStartedEvent

```go
type ActionStartedEvent struct {
    ToolName string `json:"tool_name"`
    Summary  string `json:"summary"`   // e.g. "read_file: path=/home/user/main.go"
}
```

### ShieldVerdictEvent

```go
type ShieldVerdictEvent struct {
    ToolName   string  `json:"tool_name"`
    Decision   string  `json:"decision"`    // "ALLOW", "BLOCK", "ESCALATE"
    Tier       int     `json:"tier"`        // 0, 1, or 2
    Confidence float64 `json:"confidence"`  // 0.0 - 1.0
    Reasoning  string  `json:"reasoning"`   // Human-readable explanation
}
```

### ActionCompletedEvent

```go
type ActionCompletedEvent struct {
    ToolName string `json:"tool_name"`
    Success  bool   `json:"success"`
    Summary  string `json:"summary"`   // Result summary or error message
}
```

### ActionArtifactEvent

```go
type ActionArtifactEvent struct {
    Artifact *types.Artifact `json:"artifact"`
}
```

Artifacts have a type (code, html, image, markdown, etc.), title, content, MIME type, and file path.

### ResponseCompleteEvent

```go
type ResponseCompleteEvent struct {
    Content  string          `json:"content"`
    Thoughts []types.Thought `json:"thoughts,omitempty"`
}
```

Thoughts are interleaved reasoning and tool call traces from the LLM's processing.

### OTRBlockedEvent

```go
type OTRBlockedEvent struct {
    Reason string `json:"reason"`
}
```

### PipelineErrorEvent

```go
type PipelineErrorEvent struct {
    Code        string `json:"code"`         // e.g. "AGENT_UNAVAILABLE"
    Message     string `json:"message"`
    Recoverable bool   `json:"recoverable"`
}
```

## EventSender Interface

```go
type EventSender interface {
    SendEvent(event *PipelineEvent) error
}
```

This is the transport abstraction. Any channel adapter implements this single method. Two implementations are provided:

### grpcEventSender

Located in `internal/engine/grpc_sender.go`. Adapts the `ClientService_SendMessageServer` stream to `EventSender`:

```go
type grpcEventSender struct {
    stream pb.ClientService_SendMessageServer
}

func (g *grpcEventSender) SendEvent(event *PipelineEvent) error {
    pbEvent := &pb.PipelineEvent{
        SessionId: event.SessionID,
        MessageId: event.MessageID,
    }
    switch event.Type {
    case EventLLMToken:
        pbEvent.EventType = pb.PipelineEventType_LLM_TOKEN
        pbEvent.LlmToken = &pb.LLMToken{Text: event.LLMToken.Text}
    // ... map each event type to its protobuf equivalent
    }
    return g.stream.Send(pbEvent)
}
```

### wsEventSender

Located in `internal/web/ws_sender.go`. Sends events as JSON over WebSocket:

```go
type wsEventSender struct {
    conn *websocket.Conn
    ctx  context.Context
    mu   sync.Mutex
}

func (w *wsEventSender) SendEvent(event *engine.PipelineEvent) error {
    data, err := json.Marshal(event)
    if err != nil {
        return err
    }
    w.mu.Lock()
    defer w.mu.Unlock()
    return w.conn.Write(w.ctx, websocket.MessageText, data)
}
```

The mutex serializes writes to the WebSocket connection, which is not safe for concurrent use.

## EventBroadcaster

Located in `internal/engine/broadcaster.go`. Manages fan-out of pipeline events to all subscribed clients.

### Data Structure

```go
type EventBroadcaster struct {
    mu      sync.RWMutex
    clients map[string]EventSender         // clientID -> sender
    subs    map[string]map[string]struct{} // sessionID -> set of clientIDs
    global  map[string]struct{}            // clientIDs subscribed to all events
}
```

### Methods

**Subscribe(clientID, sessionID, sender)**: Register a client for events on a specific session. Called when a user sends a message -- the client is subscribed for the duration of processing.

```go
func (b *EventBroadcaster) Subscribe(clientID, sessionID string, sender EventSender) {
    b.clients[clientID] = sender
    b.subs[sessionID][clientID] = struct{}{}
}
```

**SubscribeGlobal(clientID, sender)**: Register a client for all events regardless of session. Used for log entries and other cross-session events.

**Unsubscribe(clientID)**: Remove a client from all subscriptions -- session-specific and global.

**Broadcast(event)**: Send an event to all subscribers. First sends to session subscribers (matching `event.SessionID`), then to global subscribers (skipping those already sent via session subscription):

```go
func (b *EventBroadcaster) Broadcast(event *PipelineEvent) {
    sent := make(map[string]struct{})

    // Session subscribers
    if subs, ok := b.subs[event.SessionID]; ok {
        for clientID := range subs {
            sender.SendEvent(event)
            sent[clientID] = struct{}{}
        }
    }

    // Global subscribers (skip if already sent)
    for clientID := range b.global {
        if _, already := sent[clientID]; already {
            continue
        }
        sender.SendEvent(event)
    }
}
```

### Session Filtering

WebSocket events are filtered by session. Each client subscribes for a specific session via `Subscribe(clientID, sessionID, sender)`. Events for other sessions are not delivered.

The one exception is `log_entry` events, which are broadcast globally. The web server registers a `LogHook` on the logger that sends log entries to all connected WebSocket clients, regardless of session:

```go
log.AddHook(func(entry logging.LogEntry) {
    s.broadcastLogEntry(entry)  // Sends to ALL connected WebSocket clients
})
```

Log entries are processed before session filtering because they are not session-specific events. They use a separate broadcast path (direct WebSocket writes to all connections) rather than the `EventBroadcaster`.

### Subscription Lifecycle

For a typical web UI message:

1. User sends message via WebSocket.
2. `handleWSMessage` creates a `wsEventSender` and calls `ProcessMessageForWeb`.
3. `ProcessMessageForWeb` subscribes the sender: `broadcaster.Subscribe("ws-"+mid, sid, sender)`.
4. Message is forwarded to Agent. Events flow: Agent -> Engine `RunSession` handler -> `broadcaster.Broadcast` -> `wsEventSender.SendEvent` -> WebSocket write.
5. When `ProcessMessageForWeb` returns (context cancelled), the sender is unsubscribed.

For gRPC (TUI):

1. Client calls `SendMessage`.
2. `SendMessage` creates a `grpcEventSender` and subscribes it.
3. Events flow: Agent -> Engine -> broadcaster -> `grpcEventSender.SendEvent` -> gRPC stream `Send`.
4. When the client's stream context is done, the sender is unsubscribed.

## Key Source Files

| File | Purpose |
|---|---|
| `internal/engine/eventsender.go` | EventSender interface, PipelineEvent, all event structs |
| `internal/engine/broadcaster.go` | EventBroadcaster |
| `internal/engine/grpc_sender.go` | grpcEventSender implementation |
| `internal/web/ws_sender.go` | wsEventSender implementation |
| `internal/web/server.go` | Log entry broadcasting, BroadcastEvent |
| `internal/web/websocket.go` | WebSocket handler, session event routing |
