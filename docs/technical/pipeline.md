# Per-Message Pipeline

Every user message -- whether it arrives via CLI, web UI, Telegram, or any other channel adapter -- follows the same pipeline. This document traces the complete flow from input to response.

## Entry Points

All channels converge on a single transport-neutral method:

```go
engine.ProcessMessageForWeb(ctx, sender, sessionID, messageID, content, mode)
```

The `sender` implements `EventSender`, which has one method:

```go
type EventSender interface {
    SendEvent(event *PipelineEvent) error
}
```

For the CLI (gRPC), the entry point is `Engine.SendMessage`, which wraps a `grpcEventSender`. For the web UI, `handleWSMessage` creates a `wsEventSender`. Both subscribe the sender to the `EventBroadcaster` for the session and forward the message to the Agent.

## Pipeline Flow

### Phase 1: Message Ingestion

```
User input arrives
    |
    v
Store user message in SQLite (session_id, role="user", content)
    |
    v
Subscribe sender to EventBroadcaster for this session
    |
    v
Forward ProcessRequest to Agent via gRPC stream
```

The Engine stores the message before forwarding to ensure durability. The message is stored with `role: "user"` and a timestamp.

### Phase 2: Agent Processing (in the Agent process)

The Agent receives a `ProcessRequest` directive and executes the reasoning loop:

#### 2a. Load History

```go
if db != nil {
    msgs, _ := db.GetMessages(sessionID)
    for _, m := range msgs {
        history = append(history, llm.ChatMessage{Role: m.Role, Content: m.Content})
    }
}
```

History is loaded from the Agent's read-only database connection.

#### 2b. Build System Prompt

The `ContextAssembler` reads workspace files and constructs a structured system prompt:

```
# Your Identity          <- IDENTITY.md
# Core Guardrails        <- SOUL.md
# User Profile           <- USER.md
# Your Memory            <- MEMORY.md
# Your Capabilities      <- TOOLS.md
# Session Context         <- BOOT.md
# Behavioral Rules       <- hardcoded rules
# OTR Notice             <- (only in OTR mode)
# Sensitive Data Handling <- hardcoded rules
# Custom Skills           <- SkillManager.DiscoverySummary()
```

Each section includes framing text that tells the LLM how to interpret the content. The `AssembleWithSkills` method appends the skill discovery summary (names and descriptions of available skills) and any already-loaded skill bodies.

#### 2c. Summarize Stale Tool Results

Before compaction, old tool results (more than 4 turns old) are replaced with compact summaries to reduce token usage:

```go
history = SummarizeStaleToolResults(history, turnCount, 4)
```

A tool result like "Returned 4500 bytes of Go source code" replaces the full file contents.

#### 2d. Compact if Needed

If history tokens exceed 70% of the context budget (context window minus system prompt minus 4096 reserve):

```go
usagePercent := float64(historyTokens) / float64(contextBudget) * 100
if usagePercent >= 70 {
    history, _ = cfg.Agent.CompactHistory(ctx, history, contextBudget)
}
```

Compaction:
1. Splits history into "old" and "recent" portions using a 70/30 budget.
2. Extracts durable facts from old messages via an LLM call and flushes them to MEMORY.md (emitted via `EventMemoryFlush`).
3. Summarizes old messages into a compact summary via another LLM call.
4. Prepends the summary as a system message: `[Previous conversation summary: ...]`.

#### 2e. Load Tools

The Agent starts each turn with only the `load_tools` meta-tool:

```go
tools := []llm.ToolDefinition{{
    Name:        "load_tools",
    Description: "Request additional tool groups...",
}}
```

The LLM must call `load_tools` to gain access to file, shell, git, and other tool groups. This lazy-loading pattern keeps the initial tool set small and lets the LLM request only what it needs.

#### 2f. Stream LLM with Tools

```go
toolStream, _ := cfg.Provider.StreamWithTools(ctx, messages, tools,
    llm.WithSystem(systemPrompt), llm.WithMaxTokens(4096))
```

The reasoning loop processes streaming events in up to 25 rounds:

### Phase 3: Reasoning Loop (max 25 rounds)

```
for rounds < maxRounds:
    event = toolStream.Next()
    |
    +-- EventTextDelta:
    |       Emit EventToken (text goes to client via EventSender)
    |       Buffer text for reasoning thoughts
    |
    +-- EventToolCallComplete:
    |       Collect reasoning thought
    |       |
    |       +-- load_tools meta-tool:
    |       |       Emit EventToolDefsRequest to Engine
    |       |       Wait for tool definitions on resultCh
    |       |       Feed definitions back to LLM
    |       |
    |       +-- load_skills meta-tool:
    |       |       Load skill body from SkillManager
    |       |       Feed skill content back to LLM
    |       |
    |       +-- Any other tool:
    |               Emit EventToolProposal to Engine
    |               Wait for result on resultCh
    |               Feed result back to LLM
    |
    +-- EventDone / EOF:
            If pending tool results: send them to LLM, increment round
            Otherwise: break
```

#### Text Handling

Each `EventTextDelta` is immediately emitted as an `EventToken` to the client. The text is also buffered in a `reasoningBuf` for thought collection. When a tool call starts, the buffered reasoning text is saved as a thought with stage `"reasoning"`.

#### Tool Proposal Flow

For non-meta tools, the Agent emits an `EventToolProposal` via the gRPC stream to the Engine. The Engine processes it through the security pipeline (see Phase 4 below) and returns a `ToolResultDelivery`. The Agent's result-reading goroutine delivers this to `resultCh`, and the reasoning loop feeds it back to the LLM.

Each tool call generates a thought with stage `"tool_call"`:

```go
thoughts = append(thoughts, types.Thought{
    Stage:   "tool_call",
    Summary: fmt.Sprintf("%s -> %s", tc.Name, truncateResult(result.Content)),
    Detail:  map[string]any{"tool_name": tc.Name, "success": !result.IsError},
})
```

### Phase 4: Engine-Side Tool Processing

When the Engine receives a `ToolCallProposed` event from the Agent via the `RunSession` stream, it runs the full security pipeline in `handleToolProposal`:

```
ToolCallProposed received
    |
    v
Parse arguments JSON
    |
    v
Build ActionRequest (with ULID, type, payload, timestamp)
    |
    v
Compute action hash (SHA-256 of canonicalized type + payload)
    |
    v
Metadata enrichment (e.g. resolve paths, detect categories)
    |
    v
CheckProtection (hardcoded file protection)
    |--- BLOCKED --> Return error result, broadcast ActionCompleted(false)
    |--- EscalateTier2 --> Set action.MinTier = 2
    |--- WriteTier1Min --> Set action.MinTier = 1
    v
Emit ActionStarted event
    |
    v
Audit: log PROPOSED entry
    |
    v
Shield.Evaluate (3-tier security pipeline)
    |
    v
Emit ShieldVerdict event
    |
    v
Audit: log EVALUATED entry
    |
    +--- BLOCK --> Return error result, broadcast ActionCompleted(false)
    +--- ESCALATE --> Return "requires human approval"
    |
    v
Verify action hash (recompute and compare -- TOCTOU defense)
    |
    v
Chronicle snapshot (copy-on-write workspace backup)
    |
    v
IFC check (information flow control)
    |--- violation --> Return "Blocked: IFC violation"
    |
    v
Execute action
    |
    +--- MCP route? --> client.CallTool via MCP protocol
    +--- Built-in? --> executors.Execute(ctx, action)
    |
    v
Audit: log EXECUTED or FAILED entry
    |
    v
Broadcast ActionCompleted event
    |
    v
Log to daily action log (memory)
    |
    v
Return ToolResultDelivery to Agent
```

### Phase 5: Response Completion

When the LLM finishes (no more tool calls), the Agent emits `EventComplete` with the full response text and all collected thoughts.

The Engine receives `AgentResponseComplete` and:

1. Converts thoughts from protobuf to internal types.
2. Stores the assistant message in SQLite with thoughts attached.
3. Broadcasts `ResponseComplete` event to all subscribed clients.
4. Checks if the session needs a title. If the session has no title and the conversation has 3+ exchanges (6+ messages), generates a title asynchronously via an LLM call.

```
AgentResponseComplete received
    |
    v
Store assistant message (with thoughts) in SQLite
    |
    v
Broadcast ResponseComplete to all session subscribers
    |
    v
If no title AND >= 6 messages:
    Generate session title asynchronously
```

## Security Pipeline Detail

The Shield evaluation within `handleToolProposal` follows a 3-tier escalation:

```
Tier 0 (YAML policy match)
    |--- DENY --> BLOCK immediately
    |--- ALLOW (minTier <= 0) --> ALLOW
    |--- ESCALATE or no match --> continue
    v
Tier 1 (ONNX DeBERTa classifier + heuristic)
    |--- BLOCK --> BLOCK
    |--- ALLOW (minTier <= 1) --> ALLOW
    |--- otherwise --> continue
    v
Tier 2 (LLM evaluator with canary verification)
    |--- final decision (rate-limited, daily budget)
```

The `minTier` field on the action request enforces minimum evaluation depth. Actions on `MEMORY.md` or `USER.md` have `minTier=1`, ensuring they always go through at least the heuristic classifier. Actions on `AGENTS.md` or `HEARTBEAT.md` have `minTier=2`, requiring full LLM evaluation.

## Event Flow Summary

Events emitted during a typical message with one tool call:

```
1. llm_token (streaming text from LLM)
2. llm_token (more streaming text)
3. action_started (tool call beginning)
4. shield_verdict (security evaluation result)
5. action_completed (tool execution result)
6. llm_token (LLM response after tool result)
7. response_complete (final response with thoughts)
```

## Key Source Files

| File | Purpose |
|---|---|
| `internal/agent/loop.go` | `RunLoop` -- LLM reasoning loop |
| `internal/agent/context.go` | `ContextAssembler` -- system prompt construction |
| `internal/agent/compaction.go` | `Compactor` -- history compression |
| `internal/agent/toolsummary.go` | `SummarizeStaleToolResults` |
| `internal/engine/engine.go` | `RunSession`, `handleToolProposal`, `ProcessMessageForWeb` |
| `internal/engine/protection.go` | `CheckProtection` -- file protection enforcement |
| `internal/engine/redactor.go` | `StreamingRedactor` -- secret redaction |
| `internal/engine/verifier.go` | `Verifier` -- TOCTOU hash verification |
| `cmd/agent/internal_agent.go` | Agent-side `processMessage` |
