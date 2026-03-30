# System Audit Report

Date: 2026-03-30
Dataset: 21 LLM calls across 20 conversations, ~2100 log entries

---

## 1. Token Budget Breakdown

### Finding: All 35 tools sent on every call

Every single LLM call sends **all 35 tool definitions** regardless of context. A user asking "what time is it?" gets browser_navigate, calendar tools, git tools, email tools — everything.

Estimated tool definition tokens: ~3000-4000 tokens per call (35 tools with descriptions and JSON schemas).

| Component | Estimated % of Input |
|-----------|---------------------|
| System prompt (SOUL + IDENTITY) | ~15% |
| Tool definitions (35 tools) | **~35%** |
| Conversation history | ~45% |
| User message | ~5% |

**Tool definitions are the single largest controllable waste.** A simple Q&A uses 35% of input budget on tools the LLM will never call.

### Recommendation: Tool Pruning (Priority 1)

Implement domain-aware tool filtering. Based on conversation context and current message, send only relevant tools:

| Context | Tools to Send | Reduction |
|---------|--------------|-----------|
| General Q&A | none | -35 tools |
| File operations | read/write/list/search/delete (5) | -30 tools |
| Git work | git tools + file tools (12) | -23 tools |
| Browser task | browser + http tools (7) | -28 tools |
| Scheduling | schedule + calendar (5) | -30 tools |

Conservative estimate: **50-70% reduction in tool definition tokens**.

### Recommendation: Tool Result Summarization (Priority 2)

Currently, full tool results stay in conversation history forever. A `read_file` returning 5000 lines of code is sent back to the LLM on every subsequent turn.

After the LLM has processed a tool result, replace it in history with a compact summary: `"read_file returned 3200 bytes of Go code from internal/engine/engine.go"` instead of the full content.

---

## 2. Latency Breakdown

| Metric | Value |
|--------|-------|
| Min response time | 2.77s |
| Median (P50) | 10.44s |
| Average | 25.58s |
| P95 | 126.46s |
| Max | 126.46s |

The P95 outlier (126s) is a multi-tool conversation with browser navigation. Typical simple responses are 3-10s.

### Tool Execution Times

| Tool | Calls | Avg | Max |
|------|-------|-----|-----|
| browser_navigate | 10 | 3.9s | 27.2s |
| execute_command | 25 | 956ms | 21.7s |
| browser_click | 2 | 542ms | 565ms |
| browser_screenshot | 1 | 202ms | 202ms |
| read_file | 4 | <1ms | <1ms |
| canvas_create | 1 | <1ms | <1ms |

Browser navigation dominates execution time. Shell commands vary widely (1ms to 21s depending on what's run).

### Shield Evaluation

Only 2 shield blocks recorded (heuristic tier):
- `cron_manipulation (high)` — blocked crontab edit
- `null_byte (critical)` — blocked null byte injection

No Tier 2 (LLM evaluator) calls observed. The heuristic tier is catching threats before escalation. This is good — Tier 2 budget is preserved.

---

## 3. Conversation History Growth

| Metric | Value |
|--------|-------|
| Min history messages | 2 |
| Max history messages | 66 |
| Average | 31 |

History grows linearly and is **never compacted** in the observed data. No compaction events in the log at all. Either the compaction threshold hasn't been hit, or compaction isn't triggering properly.

### Recommendation: Verify Compaction (Priority 3)

Check the compaction threshold. With 66 messages and 35 tool definitions, the context window is likely approaching limits. If compaction never fires, long conversations will hit context limits and fail.

---

## 4. Session Summarization

**39 out of 43 summarization attempts failed** with `context canceled`. This means the user is closing sessions or switching before the background summarization completes.

### Recommendation: Decouple Summarization (Priority 4)

Run session summarization asynchronously after the user's request completes, not during it. Use a background goroutine with its own context that survives session switches.

---

## 5. Resource Usage

| Resource | Size |
|----------|------|
| SQLite database | 676 KB |
| Audit log (JSONL) | 254 KB |
| Engine log | 272 KB |
| WAL file | not present (clean) |

Resource usage is minimal. The database is compact. No WAL bloat.

---

## 6. Missing Telemetry

The audit revealed gaps in logging that prevent deeper analysis:

| Missing Metric | Impact |
|----------------|--------|
| Input/output token counts per LLM call | Can't measure actual token waste |
| Cache hit rates (Anthropic/OpenAI) | Can't verify prompt caching |
| System prompt size per call | Can't track prompt growth |
| Compaction events | Can't verify compaction works |
| Memory search latency | Can't measure FTS5 performance |
| Embedding generation time | Can't measure vector search overhead |

### Recommendation: Add Token Telemetry (Priority 5)

Log input_tokens, output_tokens, cache_read_tokens, system_prompt_tokens on every `llm_call_complete` event. This is essential for monitoring costs and optimization.

---

## Priority Recommendations Summary

| # | Action | Estimated Impact | Effort |
|---|--------|-----------------|--------|
| 1 | **Tool pruning** — domain-aware tool filtering | 50-70% fewer tool definition tokens | Medium |
| 2 | **Tool result summarization** — compact results in history | Prevents history bloat | Medium |
| 3 | **Verify compaction** — ensure context compaction fires | Prevents context limit crashes | Low |
| 4 | **Decouple summarization** — async with own context | Fixes 91% summarization failure rate | Low |
| 5 | **Add token telemetry** — log token counts per LLM call | Enables future optimization | Low |
