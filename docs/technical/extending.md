# Extending OpenParallax

This guide covers how to add new channel adapters, executors, tool action types, and (with caveats) Shield tiers.

## Adding a New Channel Adapter

Channel adapters connect external messaging platforms (Telegram, WhatsApp, Discord, etc.) to the Engine. Every adapter follows the same pattern: implement `EventSender`, call `ProcessMessageForWeb`.

### Step 1: Implement EventSender

Create a sender that translates `PipelineEvent` structs into your platform's format:

```go
package myadapter

import (
    "github.com/openparallax/openparallax/internal/engine"
)

type myEventSender struct {
    // Your transport connection (e.g., Telegram bot API client, WebSocket, etc.)
    client *MyPlatformClient
    chatID string
}

func (s *myEventSender) SendEvent(event *engine.PipelineEvent) error {
    switch event.Type {
    case engine.EventLLMToken:
        // Buffer tokens and send as complete messages (most platforms
        // don't support streaming). Or batch with a timer.
        return s.bufferToken(event.LLMToken.Text)

    case engine.EventActionStarted:
        return s.client.SendTypingIndicator(s.chatID)

    case engine.EventActionCompleted:
        if !event.ActionCompleted.Success {
            return s.client.SendMessage(s.chatID,
                "Action failed: "+event.ActionCompleted.Summary)
        }
        return nil

    case engine.EventResponseComplete:
        // Flush any buffered tokens, then send the complete response.
        s.flushBuffer()
        return s.client.SendMessage(s.chatID, event.ResponseComplete.Content)

    case engine.EventError:
        return s.client.SendMessage(s.chatID,
            "Error: "+event.Error.Message)
    }
    return nil
}
```

Most messaging platforms do not support streaming, so token events should be buffered and flushed when `ResponseComplete` arrives. The complete response text is available in `event.ResponseComplete.Content`.

### Step 2: Handle Incoming Messages

When your platform receives a message, create or look up a session and call the Engine:

```go
func (a *MyAdapter) handleIncomingMessage(platformMsg *PlatformMessage) {
    // Map platform user/chat to an OpenParallax session.
    sessionID := a.resolveSession(platformMsg.ChatID)

    // Generate a unique message ID.
    messageID := crypto.NewID()

    // Create the event sender for this conversation.
    sender := &myEventSender{
        client: a.client,
        chatID: platformMsg.ChatID,
    }

    // Determine mode (normal or OTR).
    mode := types.SessionNormal

    // Call the Engine. This blocks until the response is complete.
    ctx := context.Background()
    err := a.engine.ProcessMessageForWeb(ctx, sender, sessionID, messageID,
        platformMsg.Text, mode)
    if err != nil {
        a.client.SendMessage(platformMsg.ChatID, "Processing failed: "+err.Error())
    }
}
```

`ProcessMessageForWeb` is the transport-neutral entry point. It:
1. Stores the user message.
2. Subscribes the sender for events on the session.
3. Forwards the message to the Agent.
4. Blocks until `ctx` is cancelled.
5. Unsubscribes the sender on return.

### Step 3: Register the Adapter

Add your adapter to the Engine startup in `cmd/agent/internal_engine.go`:

```go
if myConfig := cfg.Channels.MyPlatform; myConfig.Enabled {
    adapter := myadapter.New(myConfig, channelMgr, eng.Log())
    if adapter != nil {
        channelMgr.Register(adapter)
    }
}
```

The `channels.Manager` handles lifecycle (start/stop) for all registered adapters.

### Step 4: Add Config

Add your platform's config to `types.AgentConfig.Channels`:

```go
type ChannelsConfig struct {
    // existing...
    MyPlatform MyPlatformConfig `yaml:"my_platform"`
}

type MyPlatformConfig struct {
    Enabled bool   `yaml:"enabled"`
    Token   string `yaml:"token"`
    // platform-specific settings
}
```

### Reference Implementation

The cleanest reference is the web adapter:
- `internal/web/ws_sender.go` -- `wsEventSender` (EventSender implementation)
- `internal/web/websocket.go` -- `handleWSMessage` (incoming message handling)
- `internal/web/server.go` -- Server setup

## Adding a New Executor

Executors handle specific categories of tool calls. Each executor implements the `Executor` interface.

### Step 1: Implement the Interface

```go
package executors

import (
    "context"
    "github.com/openparallax/openparallax/internal/types"
)

type MyExecutor struct {
    // dependencies
}

func NewMyExecutor() *MyExecutor {
    return &MyExecutor{}
}

func (e *MyExecutor) SupportedActions() []types.ActionType {
    return []types.ActionType{
        types.ActionType("my_tool_action"),
    }
}

func (e *MyExecutor) Execute(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
    switch action.Type {
    case "my_tool_action":
        return e.handleMyAction(ctx, action)
    default:
        return &types.ActionResult{
            RequestID: action.RequestID,
            Success:   false,
            Error:     "unsupported action: " + string(action.Type),
        }
    }
}

func (e *MyExecutor) handleMyAction(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
    // Extract parameters from action.Payload
    param, _ := action.Payload["my_param"].(string)

    // Do the work
    result, err := doSomething(param)
    if err != nil {
        return &types.ActionResult{
            RequestID: action.RequestID,
            Success:   false,
            Error:     err.Error(),
            Summary:   "my_tool_action failed",
        }
    }

    return &types.ActionResult{
        RequestID: action.RequestID,
        Success:   true,
        Output:    result,
        Summary:   "my_tool_action completed",
    }
}

func (e *MyExecutor) ToolSchemas() []ToolSchema {
    return []ToolSchema{
        {
            ActionType:  "my_tool_action",
            Name:        "my_tool_action",
            Description: "Does something useful. Call this when the user asks to...",
            Parameters: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "my_param": map[string]any{
                        "type":        "string",
                        "description": "The parameter value",
                    },
                },
                "required": []string{"my_param"},
            },
        },
    }
}
```

The `ToolSchemas()` method returns JSON Schema definitions that are sent to the LLM. The `Description` is critical -- it tells the LLM when and how to use the tool.

### Step 2: Register in the Registry

Add your executor to `NewRegistry` in `internal/engine/executors/registry.go`:

```go
func NewRegistry(workspacePath string, cfg *types.AgentConfig, ...) *Registry {
    r := &Registry{executors: make(map[types.ActionType]Executor)}

    // existing registrations...
    r.register(NewMyExecutor())

    // Rebuild groups
    r.Groups = NewGroupRegistry()
    for _, g := range DefaultGroups(r.AllToolSchemas()) {
        r.Groups.Register(g)
    }
    return r
}
```

### Step 3: Add to a Tool Group

Add your tool to an existing group or create a new one in `DefaultGroups`:

```go
func DefaultGroups(schemas []ToolSchema) []*ToolGroup {
    // existing groups...

    groups = append(groups, &ToolGroup{
        Name:        "my_group",
        Description: "Tools for doing something useful",
        Schemas:     filterSchemas(schemas, "my_tool_action"),
    })

    return groups
}
```

The LLM discovers your tool by calling `load_tools({"groups": ["my_group"]})`.

## Adding a New Action Type

To add a completely new tool action type:

### Step 1: Add the ActionType Constant

In `internal/types/actions.go`:

```go
const (
    // existing...
    ActionMyNewTool ActionType = "my_new_tool"
)
```

### Step 2: Update the Proto Enum

In `proto/openparallax/v1/types.proto`:

```protobuf
enum ActionType {
    // existing...
    MY_NEW_TOOL = N;
}
```

Run `make proto` to regenerate Go code.

### Step 3: Implement the Executor

Follow the executor pattern above. Map the new `ActionType` to handler logic.

### Step 4: Consider Protection

If your action modifies sensitive files, add entries to the protection maps in `internal/engine/protection.go`. If it performs writes, ensure `isWriteAction` returns true for it.

### Step 5: Consider OTR Filtering

If your action modifies state (writes files, sends messages, etc.), it should be filtered out in OTR mode. OTR tool filtering happens at the group level -- tools that modify state are excluded from OTR sessions by the group resolver.

## Adding a New Shield Tier

This is not recommended for most use cases. The four-tier architecture is designed to balance speed, accuracy, and cost. However, the Shield pipeline is extensible.

### Shield Pipeline Architecture

The `shield.Pipeline` evaluates actions through tiers sequentially. Each tier returns a `Verdict` with a decision (ALLOW, BLOCK, ESCALATE) and the `MinTier` field on the action controls which tiers are consulted.

### Where to Hook

The most practical extension point is between Shield evaluation and execution in `handleToolProposal`. You can add additional checks after the Shield verdict:

```go
// In handleToolProposal, after Shield.Evaluate:
verdict := e.shield.Evaluate(ctx, action)

// Custom check
if myCustomCheck(action) {
    verdict = types.Verdict{Decision: types.VerdictBlock, Reasoning: "Custom check failed"}
}
```

### Tier 3: Human-in-the-Loop

OpenParallax already includes a Tier 3 mechanism for human approval. The `Tier3Manager` handles approval requests:

1. When an action requires human approval, the Engine emits an `approval_needed` event.
2. The web UI displays the approval request.
3. The user approves or denies via `tier3_decision` WebSocket message.
4. The Engine resolves the pending approval and proceeds or blocks.

### Modifying the Shield Pipeline Itself

The Shield pipeline is configured via `shield.Config`. Modifying the tier logic requires changes to `internal/shield/` and is a significant undertaking. The fail-closed design means every error path must return BLOCK -- adding a tier that can fail open would compromise the security model.

## General Guidelines

1. **Follow the interfaces.** `EventSender`, `Executor`, and the gRPC service contracts are the extension points. Work within them.
2. **Fail closed.** Any security-related extension must return BLOCK on error, not ALLOW.
3. **No CGo.** All code must be pure Go (`CGO_ENABLED=0`). Use `modernc.org/sqlite` for database, `onnxruntime-purego` for ML inference.
4. **Platform-specific code uses build tags.** If your extension has OS-specific behavior, use `//go:build linux`, `//go:build darwin`, etc.
5. **Escalate before adding dependencies.** New `go get` calls, protobuf changes, and config schema changes require review.

## Key Source Files

| File | Purpose |
|---|---|
| `internal/engine/eventsender.go` | EventSender interface to implement |
| `internal/engine/executors/executor.go` | Executor interface to implement |
| `internal/engine/executors/registry.go` | Where to register new executors |
| `internal/engine/executors/groups.go` | Tool group definitions |
| `internal/engine/engine.go` | ProcessMessageForWeb entry point |
| `internal/web/websocket.go` | Reference for channel adapter pattern |
| `internal/web/ws_sender.go` | Reference EventSender implementation |
