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
    "context"
    "fmt"
    "log"
    "os"

    "github.com/openparallax/openparallax/memory"
    "github.com/openparallax/openparallax/memory/sqlite"
)

func main() {
    ctx := context.Background()

    // 1. Create a SQLite store (zero dependencies, single file)
    store, err := sqlite.NewStore("memory.db")
    if err != nil {
        log.Fatal(err)
    }
    defer store.Close()

    // 2. Configure an embedding provider
    embedder := memory.NewEmbeddingProvider(memory.EmbeddingConfig{
        Provider: "openai",
        Model:    "text-embedding-3-small",
        APIKey:   os.Getenv("OPENAI_API_KEY"),
    })

    // 3. Create the memory instance
    mem := memory.New(store, memory.WithEmbedder(embedder))

    // 4. Store some records
    records := []memory.Record{
        {
            ID:   "note-1",
            Text: "The deployment pipeline uses GitHub Actions with a staging environment on Railway.",
            Metadata: map[string]string{
                "source": "meeting-notes",
                "date":   "2026-03-15",
            },
        },
        {
            ID:   "note-2",
            Text: "User prefers Go for backend services and Svelte for frontend. Dislikes React.",
            Metadata: map[string]string{
                "source": "preferences",
            },
        },
        {
            ID:   "note-3",
            Text: "The database schema uses UUID primary keys and created_at/updated_at timestamps on every table.",
            Metadata: map[string]string{
                "source": "architecture-decision",
            },
        },
    }

    for _, r := range records {
        if err := mem.Store(ctx, r); err != nil {
            log.Fatal(err)
        }
    }
    fmt.Println("Stored 3 records")

    // 5. Search by meaning
    results, err := mem.Search(ctx, "CI/CD setup", 5)
    if err != nil {
        log.Fatal(err)
    }

    for _, r := range results {
        fmt.Printf("  [%.3f] %s: %s\n", r.Score, r.ID, r.Text[:60])
    }
    // Output:
    //   [0.847] note-1: The deployment pipeline uses GitHub Actions with a stagi...
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
