# Memory

OpenParallax has a persistent memory system that lets the agent remember context across sessions. Memory combines structured markdown files, full-text search (FTS5), and optional vector embeddings for semantic search.

## How Memory Works

After each normal session (non-OTR), the engine logs key information to the memory system:

1. **Session summary** — the agent's understanding of what happened in the conversation
2. **User preferences** — any preferences, facts, or context the user shares
3. **Task outcomes** — what was accomplished, what files were changed, what decisions were made

This information is indexed for future retrieval. When the agent starts a new session, it assembles context from memory to provide continuity across conversations.

## Memory Files

The workspace contains several markdown files that serve as the agent's persistent knowledge base. These files are loaded into the system prompt on startup and provide the foundational context for every conversation.

### SOUL.md

Core values, guardrails, and personality traits. This is the agent's ethical framework — it defines what the agent will and will not do, regardless of instructions.

**Contents:**

- **Core Values** — safety first, honesty, privacy, proportionality
- **Guardrails** — hard limits on file access, system modification, communication, and credential handling
- **Personality** — communication style preferences (direct, concise, adaptive)

SOUL.md is protected by Shield policy. The default policy escalates any modification to Tier 2 (LLM evaluator), and the strict policy blocks deletion entirely.

### IDENTITY.md

Agent name, role, and communication style.

```markdown
# Identity

## Name
Atlas

## Role
Personal AI Agent

## Communication Style
Direct, concise, helpful.
```

Like SOUL.md, identity modifications are escalated to Tier 2 in the default and strict policies.

### USER.md

Your personal profile. The agent uses this to personalize responses, and is nudged through its system prompt to append durable facts it learns about you (preferences, role, projects, recurring contacts, working hours) so they survive across sessions.

```markdown
# User Profile

## Name
Jane

## Timezone
America/New_York

## Language
en

## Preferences
- Preferred code style: tabs
- Communication tone: casual
```

Edit this file directly or ask the agent to update it during conversation. User data modifications are evaluated at Tier 1 in the default policy.

### MEMORY.md

Accumulated knowledge from past sessions. The agent appends session summaries and important facts here.

```markdown
# Memory

Session summaries and accumulated knowledge are recorded here.

- Helped set up a Python project with FastAPI
- User prefers pytest over unittest
- Deployed the API to fly.io on 2026-03-15
```

This file grows over time as the agent learns from conversations.

### HEARTBEAT.md

Scheduled task definitions in cron format. See [Heartbeat](/guide/heartbeat) for details.

### AGENTS.md

Multi-agent roster listing all agents, their workspaces, and channel assignments. Used when running multiple agents.

## Search Methods

### FTS5 Full-Text Search

Every memory entry is indexed using SQLite's FTS5 extension. FTS5 provides fast keyword matching with ranking.

**Capabilities:**

- Exact phrase matching: `"deploy to production"`
- Boolean operators: `python AND fastapi`
- Prefix matching: `deploy*`
- Proximity matching
- Relevance-ranked results

FTS5 search is always available, even without an embedding provider configured.

### Vector Embeddings

When an embedding provider is configured (in `memory.embedding`), memory entries are also stored as vector embeddings. This enables semantic search — finding entries that are conceptually related, not just keyword matches.

**Example:** Searching for "how to deploy" would find entries about "pushed to production", "set up CI/CD pipeline", or "configured fly.io" even if those entries do not contain the word "deploy".

Vector search uses cosine similarity to rank results by semantic relevance.

**Supported embedding providers:**

| Provider | Model | Dimensions |
|----------|-------|------------|
| OpenAI | `text-embedding-3-small` | 1536 |
| Google | `text-embedding-004` | 768 |
| Ollama | `nomic-embed-text` | 768 |

### Combined Search

When both FTS5 and vector search are available, memory queries use both methods and merge the results. FTS5 catches exact matches, while vector search catches semantic relationships. The combined approach provides the broadest recall.

## Searching Memory

### From a Conversation

Ask the agent to search its memory:

```
What do you remember about the Python project we worked on?
```

The agent uses the `memory_search` tool to query both FTS5 and vector indices, then synthesizes the results into its response.

### From the CLI

```bash
# List memory entries
openparallax memory show

# Search by query
openparallax memory search "python fastapi deployment"
```

## Memory in Context Assembly

When the agent processes a new message, it assembles context from multiple sources:

1. **Identity files** — `SOUL.md`, `IDENTITY.md`, and `USER.md` are loaded **whole** every turn. They are short and define the agent's invariants.
2. **Semantic memory retrieval** — `MEMORY.md`, `AGENTS.md`, and `HEARTBEAT.md` are **not** loaded whole. They are chunked and embedded at index time, then queried per turn. The top-k most relevant chunks (default 5) for the current user message are injected into the system prompt as a "Your Memory" section.
3. **Session history** — the current session's message history (subject to compaction below).
4. **Loaded skills** — any custom skills activated for this session.

The retrieval step is what makes memory scale. A user with thousands of memory entries pays the same per-turn token cost as a user with 10 entries — only the most similar 5 chunks reach the LLM. The full pipeline (chunking → embedding → store → retrieve → inject) is documented in [Token Economy → Semantic Memory Retrieval](/technical/design-efficiency#semantic-memory-retrieval).

Retrieved chunks are wrapped in `[MEMORY]` boundary tags so the LLM treats them as **reference data, not directives**. This prevents indirect prompt injection through poisoned memory entries — an attacker who manages to insert malicious text into MEMORY.md cannot trick the agent into obeying it.

### Compaction

When session history grows too long for the LLM context window, the engine automatically compacts older messages into summaries. This preserves the key information while staying within token limits.

## Memory and OTR

OTR sessions do not write to memory. No session summaries, no MEMORY.md updates. This is by design — OTR mode guarantees that the conversation leaves no trace in the agent's long-term memory.

The agent can still read memory during OTR sessions. It has access to all previously stored knowledge for answering questions, but it will not remember anything from the OTR conversation afterward.

## Memory and Privacy

All memory data is stored locally:

- **SQLite database** — `<workspace>/.openparallax/openparallax.db` contains session data, FTS5 indices, and vector embeddings
- **Markdown files** — SOUL.md, MEMORY.md, USER.md, etc. are plain text files in the workspace directory
- **No external storage** — memory data never leaves your machine (except when embedding requests are sent to the configured provider)

Embedding requests send text to the configured provider (OpenAI, Google, or Ollama) for vectorization. If privacy is critical, use Ollama with a local embedding model to keep everything on your machine.

## Next Steps

- [Skills](/guide/skills) — extend the agent with custom domain knowledge
- [Sessions](/guide/sessions) — how sessions feed into memory
- [Configuration](/guide/configuration) — configure embedding providers
