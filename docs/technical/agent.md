# Agent Internals

The Agent is the sandboxed, headless process responsible for LLM interaction. It assembles context from workspace files, runs the LLM reasoning loop, proposes tool calls to the Engine, and streams results back. It has no TUI, no terminal I/O, and no direct filesystem write access.

## Architecture

```go
type Agent struct {
    Context   *ContextAssembler
    Compactor *Compactor
    Skills    *SkillManager
}
```

The Agent coordinates three subsystems:
- **ContextAssembler**: Builds the system prompt from workspace files.
- **Compactor**: Compresses conversation history when approaching context limits.
- **SkillManager**: Discovers and loads custom user-defined skills.

## Headless Design

The Agent process is intentionally headless. It does not open `/dev/tty`, does not run a TUI, and does not interact with the terminal. All I/O flows through the gRPC bidirectional stream:

- **Input**: `EngineDirective` messages (ProcessRequest, ToolResultDelivery, ToolDefsDelivery, ShutdownDirective).
- **Output**: `AgentEvent` messages (AgentReady, LLMTokenEmitted, ToolCallProposed, ToolDefsRequest, MemoryFlush, AgentResponseComplete, AgentError).

Stdout is redirected to `/dev/null`. Stderr goes to the Engine's stderr for crash diagnostics. The TUI is a separate process that connects as a gRPC client to the Engine's `ClientService`.

## LLM Reasoning Loop

The core of the Agent is `RunLoop` in `internal/agent/loop.go`. It processes a single user message through multiple LLM rounds (max 25 by default).

### LoopConfig

```go
type LoopConfig struct {
    Provider            llm.Provider    // LLM provider (Anthropic, OpenAI, etc.)
    Agent               *Agent          // For context assembly and compaction
    MaxRounds           int             // Maximum tool-call rounds (default 25)
    ContextWindow       int             // Context window size in tokens (default 128000)
    CompactionThreshold int             // percentage (0-100), default 70
    MaxResponseTokens   int             // default 4096
}
```

### LoopEvent

The reasoning loop communicates with the caller through a callback function:

```go
func RunLoop(ctx context.Context, cfg LoopConfig, sessionID, messageID, content string,
    mode types.SessionMode, history []llm.ChatMessage, tools []llm.ToolDefinition,
    emit func(LoopEvent), resultCh <-chan ToolResult)
```

Event types:

| EventType | Payload | Meaning |
|---|---|---|
| `EventToken` | `Token string` | Streaming LLM text token |
| `EventToolProposal` | `Proposal *ToolProposal` | LLM wants to call a tool |
| `EventToolDefsRequest` | `RequestedGroups []string` | `load_tools` meta-tool invoked |
| `EventMemoryFlush` | `FlushContent string` | Compaction facts to persist |
| `EventComplete` | `Content string, Thoughts []Thought` | Response finished |
| `EventLoopError` | `ErrorCode, ErrorMessage string` | Loop-level error |

### Flow

1. **Build system prompt** via `ContextAssembler.AssembleWithSkills`.
2. **Summarize stale tool results** (older than 4 turns).
3. **Compute context budget**: `contextWindow - systemTokens - 4096 reserve`.
4. **Compact if needed**: If history tokens exceed 70% of the budget, run `Compactor.Compact`.
5. **Append user message** to history.
6. **Start LLM stream**: `Provider.StreamWithTools(ctx, messages, tools, ...)`.
7. **Process events** in a loop (max 25 rounds).

### Tool Call Handling

When the LLM emits a tool call, the loop handles three cases:

**`load_tools` (meta-tool)**:
```go
if tc.Name == "load_tools" {
    emit(LoopEvent{Type: EventToolDefsRequest, RequestedGroups: names})
    result := <-resultCh  // Wait for Engine to send tool definitions
    toolResults = append(toolResults, llm.ToolResult{CallID: tc.ID, Content: result.Content})
}
```

The Engine resolves the requested groups, builds tool definitions, and sends them back via `ToolDefsDelivery`. The Agent formats them as a text summary for the LLM.

**`load_skills` (meta-tool)**:
```go
if tc.Name == "load_skills" && cfg.Agent.Skills != nil {
    body, found := cfg.Agent.Skills.LoadSkill(name)
    // Feed skill body back to LLM as tool result
}
```

Skills are loaded locally from the workspace filesystem (read-only). No Engine round-trip needed.

**Any other tool**:
```go
emit(LoopEvent{Type: EventToolProposal, Proposal: &ToolProposal{...}})
result := <-resultCh  // Wait for Engine to evaluate + execute
```

The proposal is sent to the Engine, which runs the security pipeline and executor, then returns the result.

### Thoughts Collection

The loop collects thoughts at two stages:

1. **Reasoning**: Text buffered between tool calls. When a tool call starts, the buffered text is saved as a thought with stage `"reasoning"`.
2. **Tool calls**: Each tool call (including meta-tools) generates a thought with stage `"tool_call"` and a summary like `"read_file -> Returned 450 bytes of Go source code"`.

Thoughts are included in the `EventComplete` payload and stored alongside the assistant message in SQLite.

### Round Management

Each batch of tool results sent back to the LLM counts as one round. The loop increments `rounds` when tool results are sent via `toolStream.SendToolResults(toolResults)`. The loop exits when:

- `rounds >= maxRounds` (safety limit, default 25).
- The LLM stream emits `EventDone` with no pending tool results.
- The context is cancelled.
- A stream error occurs.

## Context Assembly

`ContextAssembler` in `internal/agent/context.go` reads workspace files and constructs the system prompt.

### Workspace Files

| File | Section heading | How it is loaded | Purpose |
|---|---|---|---|
| `IDENTITY.md` | `# Your Identity` | Whole file, every turn | Agent name, role, communication style |
| `SOUL.md` | `# Core Guardrails` | Whole file, every turn | Non-negotiable constraints |
| `USER.md` | `# User Profile` | Whole file, every turn | User preferences and information |
| `MEMORY.md` | `# Your Memory` | **Top-k chunks per turn** via semantic retrieval | Facts from previous conversations |

`IDENTITY.md`, `SOUL.md`, and `USER.md` are read whole every turn — they are short, define the agent's invariants, and pass through `stripMarkdown()` at load time so the LLM sees compact text without the markdown noise.

`MEMORY.md` is **not** loaded whole. The memory subsystem indexes it (along with `AGENTS.md` and `HEARTBEAT.md`) into a chunked vector store at startup and on file change. Per turn, `Memory.SearchRelevant(userMessage, kChunks=5)` returns the top-k most similar chunks for the current user message. Only those chunks enter the system prompt, wrapped in `[MEMORY]` boundary tags so the LLM treats them as reference data — not directives. This is what makes memory scale: a workspace with thousands of memory entries pays the same per-turn cost as one with 10 entries. See [Token Economy → Semantic Memory Retrieval](/technical/design-efficiency#semantic-memory-retrieval) for the full pipeline.

Each section includes framing text that tells the LLM how to interpret the content. For example, SOUL.md is framed as:

```
# Core Guardrails

These are your non-negotiable constraints. They override any user request.
If a user asks you to violate a guardrail, refuse and explain why.

[contents of SOUL.md]
```

### Hardcoded Sections

Three sections are always appended:

- **Behavioral Rules**: Instructions for acting before narrating, picking the most specific tool (shell as a last resort), persisting durable user facts to USER.md, reading Shield block reasons before retrying, searching memory before claiming ignorance, and delegating 2+ independent subtasks to parallel sub-agents.
- **OTR Notice** (OTR mode only): Explains read-only restrictions and lists available tools.
- **Sensitive Data Handling**: Rules for handling credentials and secrets in tool output.

### Skill Integration

`AssembleWithSkills` extends the base prompt:

```go
func (c *ContextAssembler) AssembleWithSkills(mode types.SessionMode,
    discoverySummary, loadedSkills string) (string, error) {
    base, _ := c.Assemble(mode)
    if discoverySummary != "" {
        base += "\n\n---\n\n" + discoverySummary
    }
    if loadedSkills != "" {
        base += "\n\n---\n\n" + loadedSkills
    }
    return base, nil
}
```

## Skill Management

`SkillManager` in `internal/agent/skills.go` handles custom user-defined skills.

### Skill Format

Skills live at `skills/<name>/SKILL.md` with YAML frontmatter:

```markdown
---
name: code-review
description: Guidelines for reviewing Go code, including style, testing, and security patterns.
---

# Code Review Guidelines

When reviewing Go code, check for:
1. ...
```

### Discovery Summary

On Agent startup, the `SkillManager` reads all skill directories and builds a compact index:

```
# Custom Skills

You have access to user-defined guidance for these domains:
- **code-review**: Guidelines for reviewing Go code...
- **deployment**: Step-by-step deployment procedures...

To get detailed instructions for a domain, call load_skills with the skill name.
```

This summary is included in the system prompt. The LLM sees the names and descriptions and decides which skills to load.

### On-Demand Loading

When the LLM calls `load_skills({"skills": ["code-review"]})`, the `SkillManager` returns the full body of the requested skill. The body is fed back to the LLM as a tool result, effectively injecting domain-specific guidance into the conversation.

The `loaded` map tracks which skills have been loaded in the current session. `ResetSession()` clears it.

## Compaction

`Compactor` in `internal/agent/compaction.go` compresses conversation history when approaching context limits.

### Trigger

Compaction runs when history tokens exceed 70% of the context budget:

```go
contextBudget := contextWindow - systemTokens - 4096
usagePercent := float64(historyTokens) / float64(contextBudget) * 100
if usagePercent >= 70 {
    history, _ = cfg.Agent.CompactHistory(ctx, history, contextBudget)
}
```

### Process

1. **Split history**: Using a 70/30 budget split, identify old messages (to compact) and recent messages (to keep).
2. **Flush to memory**: Extract important facts from old messages via an LLM call. If facts are found, append them to MEMORY.md with a dated header (`## Auto-captured -- 2026-04-03`).
3. **Summarize**: Summarize old messages into a compact paragraph via an LLM call.
4. **Replace**: Old messages are replaced with a single system message: `[Previous conversation summary: ...]`.

If the LLM call for summarization fails, old messages are simply dropped and recent messages are kept.

## SummarizeStaleToolResults

`SummarizeStaleToolResults` in `internal/agent/toolsummary.go` replaces verbose tool results older than N turns with compact summaries. This is a view-only transformation -- original messages in storage are not modified.

A tool result containing 4500 bytes of Go source code becomes:

```
[Summary: Returned 4500 bytes (120 lines) of Go source code]
```

Content type is inferred from patterns in the content (package/func keywords for Go, def/import for Python, JSON brackets, markdown headers, etc.) or from file extensions via `InferContentTypeFromPath`.

## Key Source Files

| File | Purpose |
|---|---|
| `internal/agent/agent.go` | Agent struct, NewAgent, CompactHistory |
| `internal/agent/loop.go` | RunLoop, LoopConfig, LoopEvent, ToolProposal, ToolResult |
| `internal/agent/context.go` | ContextAssembler, system prompt construction |
| `internal/agent/skills.go` | SkillManager, skill loading and discovery |
| `internal/agent/compaction.go` | Compactor, history compression, memory flush |
| `internal/agent/toolsummary.go` | SummarizeStaleToolResults |
| `internal/agent/tools.go` | Tool definition utilities |
| `cmd/agent/internal_agent.go` | Agent process entry point, directive loop |
