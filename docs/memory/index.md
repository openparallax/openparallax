---
description: OpenParallax Memory module — dual FTS5 full-text and vector similarity search for AI agents, with pluggable embedding backends and zero CGo.
outline: deep
---

# Memory

Semantic memory for AI agents. Dual search (FTS5 full-text + vector similarity), embedding provider agnostic, zero CGo.

Memory is the knowledge layer that lets an AI agent remember what happened yesterday, find relevant context from thousands of past conversations, and build up persistent understanding of the user over time. It ships as part of OpenParallax but works as a standalone library in any Go, Python, or Node.js project.

## What Is Semantic Memory?

Traditional keyword search finds documents containing exact words. Semantic memory goes further: it understands *meaning*. When you search for "database migration strategy," semantic memory returns results about "schema versioning" and "data model evolution" even if those documents never contain the word "migration."

Memory achieves this through **dual search** -- combining two complementary approaches:

### FTS5 Full-Text Search

SQLite's FTS5 engine provides fast, ranked keyword search with support for Boolean operators, prefix matching, phrase queries, and BM25 relevance scoring. FTS5 excels at finding exact terms, names, file paths, error messages, and other content where the precise wording matters.

```
query: "CORS middleware error"
→ finds: "the CORS middleware returned error 403 on /api/sessions"
```

### Vector Similarity Search

Text is converted into high-dimensional vectors (embeddings) that capture semantic meaning. Similar concepts cluster together in vector space regardless of the specific words used. Cosine similarity between vectors measures how related two pieces of text are.

```
query: "how to handle rate limiting"
→ finds: "implemented exponential backoff with jitter for API throttling"
```

### Hybrid Search: Best of Both

Memory merges results from both search modes using configurable weights (default: 70% vector, 30% keyword). This ensures you get both semantically relevant results *and* exact keyword matches, ranked together in a single result list.

```go
results, err := memory.HybridSearch(ctx, db, embedder, vectorStore, "deployment strategy", 10)
// Returns results from both FTS5 and vector search, merged and ranked
```

## Why Pluggable Backends?

Different deployment scenarios need different storage engines:

| Scenario | Backend | Why |
|----------|---------|-----|
| Personal AI agent on your laptop | **SQLite** | Zero setup. Single file. No server process. |
| Production with 500K+ vectors | **PostgreSQL + pgvector** | HNSW index. ACID transactions. Existing infra. |
| Vector-first workload, millions of records | **Qdrant** | Purpose-built. Distributed. Payload filtering. |
| Serverless, pay-per-query | **Pinecone** | Managed. Scale-to-zero. No ops. |
| Hybrid search (vector + BM25) in one engine | **Weaviate** | Native hybrid. Schema-aware. |
| ML/AI prototyping | **ChromaDB** | Simple API. Popular in the ecosystem. |
| Already running Redis | **Redis + RediSearch** | Vector + full-text in your existing cache layer. |

The key insight: **the interface is the same regardless of backend**. Your application code writes to `Store` -- swap the backend by changing one line of configuration, not hundreds of lines of business logic.

```go
// Personal laptop
store := sqlite.NewStore("memory.db")

// Production
store := pgvector.NewStore("postgres://user:pass@db:5432/memory")

// Same interface, same application code
store.Upsert(ctx, record)
results, _ := store.Search(ctx, query, limit)
```

## Embedding Provider Agnostic

Memory does not embed text itself -- it delegates to an `EmbeddingProvider` interface. This means you can use any embedding API:

| Provider | Model | Dimensions | Notes |
|----------|-------|-----------|-------|
| **OpenAI** | `text-embedding-3-small` | 1536 | Best quality/cost ratio |
| **OpenAI** | `text-embedding-3-large` | 3072 | Highest quality |
| **Google** | `text-embedding-004` | 768 | Good quality, competitive pricing |
| **Ollama** | `nomic-embed-text` | 768 | Runs locally. Free. Private. |
| **Custom** | Any | Any | Implement `EmbeddingProvider` interface |

The embedding provider is configured separately from the backend. You can use OpenAI embeddings with a Qdrant backend, or Ollama embeddings with PostgreSQL. Any combination works.

## How OpenParallax Uses Memory

Inside the OpenParallax agent, Memory serves several roles:

### Workspace Memory Files

Six markdown files form the agent's persistent knowledge:

| File | Purpose |
|------|---------|
| `SOUL.md` | Core personality, values, behavioral guidelines |
| `IDENTITY.md` | Name, role, capabilities |
| `USER.md` | Learned facts about the user (preferences, projects, context) |
| `MEMORY.md` | Session summaries, accumulated knowledge |

These files are chunked, embedded, and indexed on startup. A filesystem watcher re-indexes them when they change. The agent reads these files to assemble its system prompt and searches them for relevant context when answering questions.

### Session Summarization

When a session ends, Memory uses the LLM to generate a 2-3 bullet summary and appends it to `MEMORY.md`. Over time, this builds up a chronological knowledge base that the agent can search to recall past conversations.

### Context Assembly

When the agent receives a message, it searches Memory for relevant context -- past conversations, user preferences, tool notes -- and includes the top results in the LLM prompt. This is how the agent "remembers" things from weeks ago without keeping the full conversation history in context.

## Standalone Value

Memory is not tied to OpenParallax. It is a standalone Go package with Python and Node.js wrappers. Use it anywhere you need semantic search:

- **RAG pipelines** -- index your documents, search by meaning
- **Knowledge bases** -- build searchable knowledge from unstructured text
- **Chatbot memory** -- give any chatbot long-term recall
- **Code search** -- semantic search over codebases (chunk by function/class)
- **Note-taking apps** -- find notes by concept, not just keywords

```bash
# Go
go get github.com/openparallax/openparallax/memory

# Python
pip install openparallax-memory

# Node.js
npm install @openparallax/memory
```

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                        Memory                                │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │   Indexer     │  │   Chunker    │  │  Hybrid Search   │  │
│  │ (file→chunk→  │  │ (markdown    │  │ (vector 70% +    │  │
│  │  embed→store) │  │  splitting)  │  │  keyword 30%)    │  │
│  └──────┬───────┘  └──────────────┘  └────────┬─────────┘  │
│         │                                      │            │
│  ┌──────┴──────────────────────────────────────┴─────────┐  │
│  │                    Store Interface                     │  │
│  │         VectorStore + TextStore combined               │  │
│  └──────────────────────┬────────────────────────────────┘  │
│                         │                                    │
│  ┌──────┬───────┬───────┼───────┬──────┬───────┬─────────┐  │
│  │SQLite│pgvec  │Qdrant │Pinec  │Weav  │Chroma │ Redis   │  │
│  └──────┴───────┴───────┴───────┴──────┴───────┴─────────┘  │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │              EmbeddingProvider Interface              │    │
│  │          OpenAI │ Google │ Ollama │ Custom            │    │
│  └──────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────┘
```

## Next Steps

- [Quick Start](/memory/quickstart) -- get Memory running in 5 minutes
- [Go Library](/memory/go) -- full API reference
- [Python](/memory/python) -- Python wrapper
- [Node.js](/memory/node) -- Node.js wrapper
- [Embeddings](/memory/embeddings) -- embedding provider configuration
- [Choosing a Backend](/memory/backends/) -- feature comparison and migration guide
