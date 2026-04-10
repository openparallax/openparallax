---
outline: deep
---

# Quick Start


Get Memory running in 5 minutes. Store text, embed it, search by meaning.

## Go

### Install

```bash
go get github.com/openparallax/openparallax/memory
```

### Basic Usage

```go
package main

import (
    "fmt"
    "log"

    "github.com/openparallax/openparallax/memory"
    "github.com/openparallax/openparallax/memory/sqlite"
)

func main() {
    // 1. Create a SQLite store (wraps an existing *storage.DB instance).
    store := sqlite.NewStore(db)

    // 2. Create the memory Manager.
    mgr := memory.NewManager("/path/to/workspace", store, llmProvider)

    // 3. Read a memory file.
    soul, err := mgr.Read(memory.MemorySoul)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(soul)

    // 4. Append to MEMORY.md.
    err = mgr.Append(memory.MemoryMain, "## 2026-04-09\n- Decided to use PostgreSQL 16\n")
    if err != nil {
        log.Fatal(err)
    }

    // 5. FTS5 search across all memory files.
    results, err := mgr.Search("PostgreSQL", 10)
    if err != nil {
        log.Fatal(err)
    }
    for _, r := range results {
        fmt.Printf("  [%.3f] %s — %s\n", r.Score, r.Section, r.Snippet)
    }

    // 6. Get context-relevant chunks for the system prompt.
    chunks := mgr.SearchRelevant("database schema", 5, 5)
    for _, c := range chunks {
        fmt.Println(c)
    }
}
```

### Without Embeddings (FTS5 Only)

If you do not have an embedding API key, Memory still works -- it falls back to FTS5 keyword search:

```go
store, _ := sqlite.NewStore("memory.db")
defer store.Close()

// No embedder -- FTS5 only
mem := memory.New(store)

_ = mem.Store(ctx, memory.Record{
    ID:   "doc-1",
    Text: "Configure CORS middleware in the API gateway",
})

// Keyword search works without embeddings
results, _ := mem.Search(ctx, "CORS", 5)
```

### Local Embeddings with Ollama

For fully private, offline operation:

```go
embedder := memory.NewEmbeddingProvider(memory.EmbeddingConfig{
    Provider: "ollama",
    Model:    "nomic-embed-text",
    BaseURL:  "http://localhost:11434", // default
})

mem := memory.New(store, memory.WithEmbedder(embedder))
```

Make sure Ollama is running and the model is pulled:

```bash
ollama pull nomic-embed-text
```

## Python

### Install

```bash
pip install openparallax-memory
```

### Basic Usage

```python
from openparallax_memory import Memory, SQLiteStore, EmbeddingConfig

# Create store and memory instance
store = SQLiteStore("memory.db")
mem = Memory(
    store=store,
    embedding=EmbeddingConfig(
        provider="openai",
        model="text-embedding-3-small",
    ),
)

# Store records
mem.store(
    id="note-1",
    text="The API uses JWT tokens with 24-hour expiry and refresh token rotation.",
    metadata={"source": "security-review"},
)

mem.store(
    id="note-2",
    text="Database backups run nightly at 3 AM UTC via pg_dump to S3.",
    metadata={"source": "ops-runbook"},
)

# Search by meaning
results = mem.search("authentication mechanism", limit=5)
for r in results:
    print(f"  [{r.score:.3f}] {r.id}: {r.text[:60]}")

# Output:
#   [0.831] note-1: The API uses JWT tokens with 24-hour expiry and refresh...
```

### With Ollama (Local Embeddings)

```python
mem = Memory(
    store=SQLiteStore("memory.db"),
    embedding=EmbeddingConfig(
        provider="ollama",
        model="nomic-embed-text",
    ),
)
```

## Node.js

### Install

```bash
npm install @openparallax/memory
```

### Basic Usage

```typescript
import { Memory, SQLiteStore } from '@openparallax/memory'

// Create store and memory instance
const store = new SQLiteStore('memory.db')
const mem = new Memory({
  store,
  embedding: {
    provider: 'openai',
    model: 'text-embedding-3-small',
  },
})

// Store records
await mem.store({
  id: 'note-1',
  text: 'Error monitoring uses Sentry with source maps uploaded during CI.',
  metadata: { source: 'ops-docs' },
})

await mem.store({
  id: 'note-2',
  text: 'The frontend build generates a service worker for offline support.',
  metadata: { source: 'architecture' },
})

// Search by meaning
const results = await mem.search('crash reporting and error tracking', { limit: 5 })
for (const r of results) {
  console.log(`  [${r.score.toFixed(3)}] ${r.id}: ${r.text.slice(0, 60)}`)
}

// Output:
//   [0.862] note-1: Error monitoring uses Sentry with source maps uploaded du...

// Clean up
await store.close()
```

## What Happens Under the Hood

When you call `mem.Store()`:

1. The text is chunked into segments (default: ~400 tokens with 80-token overlap)
2. Each chunk is sent to the embedding provider (OpenAI, Google, Ollama, etc.)
3. The embedding vector and the text are stored in the backend (SQLite, PostgreSQL, etc.)
4. The text is also indexed in FTS5 for keyword search

When you call `mem.Search()`:

1. The query is embedded using the same provider
2. Vector similarity search finds semantically similar chunks (cosine similarity)
3. FTS5 keyword search finds exact term matches
4. Results are merged with configurable weights (default: 70% vector, 30% keyword)
5. Deduplicated, ranked results are returned

## Next Steps

- [Go Library](/memory/go) -- full API reference with all options
- [Python](/memory/python) -- complete Python API
- [Node.js](/memory/node) -- complete Node.js API
- [Embeddings](/memory/embeddings) -- choosing and configuring embedding providers
- [Choosing a Backend](/memory/backends/) -- when to use SQLite vs. PostgreSQL vs. Qdrant
