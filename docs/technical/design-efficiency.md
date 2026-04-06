# Token Economy: Doing More with Less

Every token in an LLM context window costs money and attention. OpenParallax treats tokens as a scarce resource — every system prompt, tool definition, and historical message is evaluated for efficiency. This page covers the techniques that reduce token consumption without reducing capability.

## Dynamic Tool Loading

This implements *Dynamic Tool Surface Reduction* — the principle of least privilege applied temporally to the agent's capability surface. Tools that are not loaded cannot be called, regardless of what the agent attempts. This is a security property, not just a performance optimization.

A common approach is to send all tool definitions with every LLM call. With 73 action types across 10 executor groups, that adds thousands of tokens to every request — tokens the LLM must process even when the user asks a simple question that requires no tools at all.

OpenParallax starts each turn with a single meta-tool:

```json
{
  "name": "load_tools",
  "description": "Load tool groups for the current task.",
  "parameters": {
    "groups": {
      "type": "array",
      "items": { "type": "string" },
      "description": "Group names: files, shell, git, browser, email, calendar, canvas, memory, http, schedule"
    }
  }
}
```

The LLM calls `load_tools(["files", "shell"])` to load only what it needs. Tool definitions enter the context only when requested. The loaded tools persist for the rest of the conversation — subsequent turns do not need to reload them.

The savings depend on the task:

| Scenario | Traditional | Dynamic | Savings |
|----------|------------|---------|---------|
| "What time is it?" | ~4,000 tokens (all tools) | ~200 tokens (load_tools only) | ~3,800 |
| File editing task | ~4,000 tokens (all tools) | ~900 tokens (files group) | ~3,100 |
| Complex multi-tool task | ~4,000 tokens (all tools) | ~4,000 tokens (all groups loaded) | 0 |

The worst case — loading every group — matches traditional behavior. The common case saves 2,000-4,000 tokens per message. At scale, across thousands of messages per day, this is significant cost reduction with zero capability loss.

The `GroupRegistry` manages tool groups and handles the `load_tools` call:

```go
type GroupRegistry struct {
    groups   map[string]*ToolGroup
    disabled map[string]bool
}
```

OTR mode uses `DisableGroups` to remove write-capable groups entirely — the tools are not filtered from responses, they are never loaded in the first place. See [Shield Pipeline](/technical/design-security) for how OTR interacts with security evaluation.

## Stripping Markdown from System Prompts

The workspace files that compose the system prompt — IDENTITY.md, TOOLS.md, and others — are written in markdown for human readability. But markdown formatting characters are tokens the LLM does not need:

```markdown
# Identity                          ← 2 tokens for "# "
**Name:** Atlas                     ← 4 tokens for "**" "**"
- Speaks in a direct, clear style   ← 1 token for "- "
---                                 ← 1 token for "---"
```

The `stripMarkdown` function removes these at load time:

```go
func stripMarkdown(s string) string {
    lines := strings.Split(s, "\n")
    var out []string
    for _, line := range lines {
        line = mdHeadingRe.ReplaceAllString(line, "$1")
        if mdHrRe.MatchString(line) {
            continue
        }
        line = strings.ReplaceAll(line, "**", "")
        line = strings.ReplaceAll(line, "__", "")
        if strings.HasPrefix(line, "- ") {
            line = line[2:]
        }
        out = append(out, line)
    }
    return strings.Join(out, "\n")
}
```

Heading markers, bold/italic markers, bullet prefixes, and horizontal rules are removed. The semantic content is preserved. On a typical system prompt, this saves 50-100 tokens — a small number per message, but these tokens are paid on every single LLM call for the entire lifetime of a conversation.

Files stay as markdown on disk. The stripping is a view transformation at context assembly time, not a destructive edit.

## Stale Tool Result Summarization

In a 20-turn conversation, the LLM is paying attention tokens to process tool results from turn 3 that are no longer relevant. A `file_read` that returned 45KB of source code is still sitting in the context, consuming thousands of tokens and diluting the LLM's attention on the current task.

`SummarizeStaleToolResults` replaces old tool outputs with compact one-line summaries:

```go
func SummarizeStaleToolResults(messages []llm.ChatMessage, currentTurn int,
    stalenessTurns int) []llm.ChatMessage {
    // For each tool result older than stalenessTurns:
    // Replace full content with a summary like:
    // "[Summary: Returned 45,231 bytes (890 lines) of Go source code]"
}
```

The summary preserves three facts: the tool was called, approximately what it returned, and the content type. The full output is gone from the context but remains in storage — if the LLM needs it again, it can re-read the file.

The staleness threshold defaults to 4 turns. Tool results from the last 4 user-assistant exchanges remain intact; older results are summarized. The transformation is view-only — original messages in storage are never modified.

Content type inference is deterministic, based on file extension and content patterns:

```go
func inferContentType(content string) string {
    if strings.Contains(lower, "package ") && strings.Contains(lower, "func ") {
        return "Go source code"
    }
    if strings.HasPrefix(content, "{") || strings.HasPrefix(content, "[") {
        return "JSON data"
    }
    // ...
}
```

This gives the LLM enough context to know what happened without paying for the full payload.

## The 70% Compaction Threshold

When conversation history exceeds 70% of the context budget, the compactor summarizes older messages into a compact paragraph. This is a more aggressive optimization than tool result summarization — it replaces entire conversation turns, not just tool outputs.

The compaction process:

1. Calculate token usage as a percentage of the model's context window
2. If below 70%, do nothing
3. If above 70%, ask the LLM to summarize the oldest N messages into a paragraph
4. Extract important facts and flush them to MEMORY.md for long-term recall
5. Replace the old messages with the summary

The 70% threshold is not arbitrary. The remaining 30% must accommodate:

- System prompt (~5-10% of context)
- Loaded tool definitions (~5-15% depending on groups)
- Current turn input and response (~10-15%)
- Safety margin for tool results in the current turn

Going higher (e.g., 90%) risks truncation when the current turn generates large tool results. Going lower (e.g., 50%) wastes half the context window — the user is paying for a 200K context model but only using 100K of it.

The threshold is configurable via `agents.compaction_threshold` in `config.yaml`. Users with predictable workloads can tune it. The default of 70 works well across a range of conversation patterns.

## Embedding Cache and Content Hashing

The memory indexer generates vector embeddings for workspace files to enable semantic search. Embedding API calls are the bottleneck — each one requires an HTTP round-trip to OpenAI or another provider.

Two optimizations eliminate redundant calls:

**Content hashing.** Before indexing a file, compute its SHA-256 hash and compare it to the stored hash from the last indexing run. If the file has not changed, skip it entirely — no chunking, no embedding, no API call. For a workspace with 500 files where 3 changed since last index, this skips 497 files.

**Embedding cache.** Two chunks with identical content produce identical embeddings. This happens more often than expected — boilerplate headers, license blocks, common import patterns. The embedding cache is keyed by content hash. If the hash matches a cached embedding, reuse it without calling the API.

Together, these make re-indexing near-instant for unchanged workspaces. A full re-index of 500 files might take 30 seconds on first run (limited by API rate limits). A subsequent re-index with 3 changed files takes under a second.

## Memoized Markdown Rendering in the Frontend

Every message in the web UI chat panel runs through `marked` (Markdown parsing) and `DOMPurify` (HTML sanitization) on every Svelte render cycle. For conversations with 100+ messages, this becomes a measurable performance bottleneck.

The `renderMarkdown` function caches parsed and sanitized HTML keyed by the raw input string:

```typescript
const markdownCache = new Map<string, string>();
const MAX_CACHE_SIZE = 500;

export function renderMarkdown(text: string): string {
  const cached = markdownCache.get(text);
  if (cached !== undefined) return cached;

  const html = DOMPurify.sanitize(marked.parse(text) as string, {
    ADD_ATTR: ['target'],
  });

  if (markdownCache.size >= MAX_CACHE_SIZE) {
    const firstKey = markdownCache.keys().next().value;
    if (firstKey !== undefined) markdownCache.delete(firstKey);
  }
  markdownCache.set(text, html);
  return html;
}
```

Messages do not change after they are sent. The cache hit rate for historical messages is 100%. The 500-entry FIFO eviction cap prevents unbounded memory growth while covering the typical visible conversation window.

This is a micro-optimization, but it compounds: 150 messages rendered 60 times per second during scrolling means 9,000 `marked.parse` + `DOMPurify.sanitize` calls per second without caching, versus 150 calls total with caching.

## The Principle

Token efficiency is not about being cheap. It is about respecting the constraints of the system. Context windows are finite. API calls cost money and time. Rendering cycles have budgets. Every optimization here follows the same pattern: eliminate redundant work, preserve semantic content, make the trade-off configurable where reasonable people might disagree.
