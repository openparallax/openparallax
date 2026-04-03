---
outline: deep
---

# Go Library


Full API reference for the Memory Go package.

```bash
go get github.com/openparallax/openparallax/memory
```

## Core Types

### Record

A `Record` is the fundamental unit of storage. It represents a piece of text with optional metadata.

```go
type Record struct {
    ID       string            // Unique identifier (auto-generated if empty)
    Text     string            // The text content to store and search
    Metadata map[string]string // Arbitrary key-value metadata for filtering
}
```

### SearchResult

Returned from search operations.

```go
type SearchResult struct {
    ID       string            // Record ID
    Text     string            // Matched text content
    Score    float64           // Relevance score (0.0 to 1.0)
    Source   string            // "vector", "keyword", or "hybrid"
    Metadata map[string]string // Record metadata
    Path     string            // Source file path (if indexed from file)
    StartLine int             // Start line in source file
    EndLine   int             // End line in source file
}
```

### HybridSearchResult

The internal type used by the hybrid search engine, combining vector and keyword results.

```go
type HybridSearchResult struct {
    ID        string
    Path      string
    StartLine int
    EndLine   int
    Text      string
    Score     float64
    Source    string // "vector", "keyword", or "hybrid"
}
```

## Store Interface

The `Store` interface combines vector and text storage. Every backend implements this interface.

### VectorStore

Handles vector embedding storage and similarity search.

```go
type VectorStore interface {
    // Upsert inserts or updates a vector with its ID.
    Upsert(ctx context.Context, id string, embedding []float32, metadata map[string]string) error

    // Search finds the nearest vectors to the query by cosine similarity.
    Search(ctx context.Context, query []float32, limit int) ([]VectorResult, error)

    // Delete removes a vector by ID.
    Delete(ctx context.Context, id string) error
}
```

### TextStore

Handles full-text indexing and keyword search.

```go
type TextStore interface {
    // Index adds or updates text content for full-text search.
    Index(ctx context.Context, id string, text string, metadata map[string]string) error

    // Search performs full-text search and returns ranked results.
    Search(ctx context.Context, query string, limit int) ([]TextResult, error)
}
```

### Store (Combined)

```go
type Store interface {
    VectorStore
    TextStore

    // Close releases backend resources.
    Close() error
}
```

## VectorSearcher Interface

The lower-level vector search interface used internally by Memory. Each implementation provides a different search strategy.

```go
type VectorSearcher interface {
    Insert(id string, embedding []float32) error
    Search(query []float32, limit int) ([]VectorResult, error)
    Delete(id string) error
}
```

### VectorResult

```go
type VectorResult struct {
    ID    string
    Score float64 // Cosine similarity (0.0 to 1.0)
}
```

### BuiltinVectorSearcher

Pure Go in-memory cosine similarity. No external dependencies. Always available. Default for the SQLite backend.

```go
vs := memory.NewBuiltinVectorSearcher()

// Insert vectors
vs.Insert("doc-1", []float32{0.1, 0.2, 0.3, ...})
vs.Insert("doc-2", []float32{0.4, 0.5, 0.6, ...})

// Search by similarity
results, _ := vs.Search(queryVector, 10)

// Check vector count
fmt.Println(vs.Count()) // 2
```

The builtin searcher uses brute-force cosine similarity (linear scan). It works well up to ~100K vectors. See [SQLite Backend Limitations](/memory/backends/sqlite#limitations) for scaling considerations.

## Creating a Memory Instance

### With SQLite (Default)

```go
import (
    "github.com/openparallax/openparallax/memory"
    "github.com/openparallax/openparallax/memory/sqlite"
)

store, err := sqlite.NewStore("memory.db")
if err != nil {
    log.Fatal(err)
}
defer store.Close()

mem := memory.New(store)
```

### With Embeddings

```go
embedder := memory.NewEmbeddingProvider(memory.EmbeddingConfig{
    Provider: "openai",
    Model:    "text-embedding-3-small",
    APIKey:   os.Getenv("OPENAI_API_KEY"),
})

mem := memory.New(store, memory.WithEmbedder(embedder))
```

### With PostgreSQL + pgvector

```go
import "github.com/openparallax/openparallax/memory/pgvector"

store, err := pgvector.NewStore("postgres://user:pass@localhost:5432/memory", pgvector.Options{
    TableName:  "memories",     // default: "memory_records"
    Dimensions: 1536,           // must match embedding model
    IndexType:  "hnsw",         // "hnsw" or "ivfflat"
})
```

### With Qdrant

```go
import "github.com/openparallax/openparallax/memory/qdrant"

store, err := qdrant.NewStore(qdrant.Options{
    URL:            "http://localhost:6333",
    CollectionName: "memories",
    Dimensions:     1536,
    APIKey:         os.Getenv("QDRANT_API_KEY"), // for Qdrant Cloud
})
```

### With Pinecone

```go
import "github.com/openparallax/openparallax/memory/pinecone"

store, err := pinecone.NewStore(pinecone.Options{
    APIKey:    os.Getenv("PINECONE_API_KEY"),
    IndexName: "memories",
    Namespace: "default",
})
```

### With Weaviate

```go
import "github.com/openparallax/openparallax/memory/weaviate"

store, err := weaviate.NewStore(weaviate.Options{
    URL:       "http://localhost:8080",
    ClassName: "Memory",
    APIKey:    os.Getenv("WEAVIATE_API_KEY"), // for Weaviate Cloud
})
```

### With ChromaDB

```go
import "github.com/openparallax/openparallax/memory/chroma"

store, err := chroma.NewStore(chroma.Options{
    URL:            "http://localhost:8000",
    CollectionName: "memories",
})
```

### With Redis

```go
import "github.com/openparallax/openparallax/memory/redis"

store, err := redis.NewStore(redis.Options{
    URL:        "redis://localhost:6379",
    IndexName:  "memory_idx",
    Prefix:     "mem:",
    Dimensions: 1536,
})
```

## Memory Options

Configure the Memory instance with functional options:

```go
mem := memory.New(store,
    memory.WithEmbedder(embedder),
    memory.WithChunkSize(400),        // tokens per chunk (default: 400)
    memory.WithChunkOverlap(80),      // overlap tokens between chunks (default: 80)
    memory.WithVectorWeight(0.7),     // weight for vector results in hybrid search (default: 0.7)
    memory.WithKeywordWeight(0.3),    // weight for keyword results (default: 0.3)
    memory.WithLogger(logger),        // structured logger
)
```

## Storing Records

### Store a Single Record

```go
err := mem.Store(ctx, memory.Record{
    ID:   "meeting-2026-03-15",
    Text: "Decided to migrate from REST to gRPC for internal services. Timeline: Q2.",
    Metadata: map[string]string{
        "type":   "meeting-note",
        "date":   "2026-03-15",
        "author": "engineering-team",
    },
})
```

### Store Without ID (Auto-Generated)

```go
err := mem.Store(ctx, memory.Record{
    Text: "User prefers dark mode and monospace fonts in the terminal.",
})
// ID is auto-generated using crypto.NewID()
```

### Upsert (Update Existing)

If you store a record with an existing ID, it replaces the previous content and re-embeds:

```go
// First store
mem.Store(ctx, memory.Record{ID: "config-note", Text: "Using port 8080"})

// Update -- same ID, new content
mem.Store(ctx, memory.Record{ID: "config-note", Text: "Migrated to port 3100"})
```

## Searching

### Hybrid Search (Default)

Combines vector similarity and FTS5 keyword search:

```go
results, err := mem.Search(ctx, "API authentication", 10)
for _, r := range results {
    fmt.Printf("[%.3f] (%s) %s\n", r.Score, r.Source, r.Text)
}
// [0.891] (hybrid) JWT tokens with refresh rotation...
// [0.834] (vector) OAuth2 authorization code flow for...
// [0.756] (keyword) API key authentication is used for...
```

### Vector-Only Search

```go
results, err := mem.SearchVector(ctx, "deployment automation", 10)
```

### Keyword-Only Search (FTS5)

```go
results, err := mem.SearchKeyword(ctx, "CORS middleware", 10)
```

### Search with Metadata Filtering

```go
results, err := mem.Search(ctx, "database schema", 10,
    memory.WithFilter("type", "architecture-decision"),
    memory.WithFilter("date", "2026-03-*"),  // glob matching
)
```

## Deleting Records

```go
// Delete by ID
err := mem.Delete(ctx, "meeting-2026-03-15")

// Delete by metadata filter
deleted, err := mem.DeleteByFilter(ctx, map[string]string{
    "type": "temporary",
})
fmt.Printf("Deleted %d records\n", deleted)
```

## Indexer

The `Indexer` handles file-level operations: reading files from disk, chunking them, embedding the chunks, and storing everything in the backend. Used by OpenParallax to index workspace memory files.

```go
indexer := memory.NewIndexer(db, embedder, vectorSearcher, logger)

// Index a single file (skips if unchanged since last index)
err := indexer.IndexFile(ctx, "/path/to/document.md")

// Index all memory files in a workspace
indexer.IndexWorkspace(ctx, "/path/to/workspace")

// Index past session transcripts
indexer.IndexSessions(ctx)
```

The indexer:
- Computes a SHA-256 hash of file contents to detect changes
- Skips files that have not changed since the last index
- Chunks markdown into ~400-token segments with 80-token overlap
- Batches embedding API calls (20 chunks per request)
- Caches embeddings by content hash to avoid re-embedding identical text

## Chunker

The `ChunkMarkdown` function splits text into overlapping segments suitable for embedding:

```go
chunks := memory.ChunkMarkdown(text, 400, 80)
// 400 = target tokens per chunk (~1600 characters)
// 80  = overlap tokens between consecutive chunks (~320 characters)

for _, chunk := range chunks {
    fmt.Printf("Lines %d-%d: %s\n", chunk.StartLine, chunk.EndLine, chunk.Text[:50])
}
```

```go
type Chunk struct {
    Text      string
    StartLine int
    EndLine   int
}
```

Overlap ensures that information spanning a chunk boundary is captured in at least one chunk. The 80-token overlap is tuned for typical paragraph lengths in technical documentation.

## EmbeddingProvider Interface

```go
type EmbeddingProvider interface {
    // Embed converts text strings into embedding vectors.
    // Supports batching -- pass multiple texts for efficiency.
    Embed(ctx context.Context, texts []string) ([][]float32, error)

    // Dimension returns the dimensionality of the embedding vectors.
    Dimension() int

    // ModelID returns the model identifier (e.g., "text-embedding-3-small").
    ModelID() string
}
```

### Creating Providers

```go
// OpenAI
embedder := memory.NewEmbeddingProvider(memory.EmbeddingConfig{
    Provider: "openai",
    Model:    "text-embedding-3-small",  // 1536 dimensions
    APIKey:   os.Getenv("OPENAI_API_KEY"),
})

// Google
embedder := memory.NewEmbeddingProvider(memory.EmbeddingConfig{
    Provider: "google",
    Model:    "text-embedding-004",  // 768 dimensions
    APIKey:   os.Getenv("GOOGLE_AI_API_KEY"),
})

// Ollama (local, free)
embedder := memory.NewEmbeddingProvider(memory.EmbeddingConfig{
    Provider: "ollama",
    Model:    "nomic-embed-text",  // 768 dimensions
    BaseURL:  "http://localhost:11434",
})
```

### Custom Provider

Implement the `EmbeddingProvider` interface:

```go
type myEmbedder struct{}

func (e *myEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    // Call your embedding API
    return embeddings, nil
}

func (e *myEmbedder) Dimension() int  { return 384 }
func (e *myEmbedder) ModelID() string { return "my-custom-model" }

mem := memory.New(store, memory.WithEmbedder(&myEmbedder{}))
```

## File Watcher

Memory includes a filesystem watcher that monitors workspace files and triggers re-indexing on changes, with 1.5-second debounce to coalesce rapid writes:

```go
err := memory.StartWatcher(ctx, workspacePath, indexer, logger)
```

Watched paths:
- `MEMORY.md` -- accumulated knowledge
- `USER.md` -- user preferences
- `HEARTBEAT.md` -- scheduled task definitions
- `memory/` directory -- daily log files

When a watched file changes, the watcher triggers `indexer.IndexWorkspace()` after a 1.5-second debounce window.

## Cosine Similarity

The pure Go cosine similarity function is exported for direct use:

```go
score := memory.CosineSimilarity(vectorA, vectorB)
// Returns: float64 between -1.0 and 1.0
// 1.0 = identical direction
// 0.0 = orthogonal (unrelated)
// -1.0 = opposite direction
```

## Hybrid Search Function

The standalone `HybridSearch` function performs merged vector + keyword search:

```go
results, err := memory.HybridSearch(ctx, db, embedder, vectorSearcher, "query text", 10)
```

Weighting: 70% vector score + 30% normalized keyword score. If no embedding provider is configured, falls back to FTS5-only results.

## Complete Example

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

    // Set up store
    store, err := sqlite.NewStore("knowledge.db")
    if err != nil {
        log.Fatal(err)
    }
    defer store.Close()

    // Set up embeddings
    embedder := memory.NewEmbeddingProvider(memory.EmbeddingConfig{
        Provider: "openai",
        Model:    "text-embedding-3-small",
        APIKey:   os.Getenv("OPENAI_API_KEY"),
    })

    // Create memory with options
    mem := memory.New(store,
        memory.WithEmbedder(embedder),
        memory.WithChunkSize(400),
        memory.WithChunkOverlap(80),
    )

    // Index a directory of markdown files
    files := []string{
        "docs/architecture.md",
        "docs/deployment.md",
        "docs/security.md",
    }
    for _, f := range files {
        data, err := os.ReadFile(f)
        if err != nil {
            continue
        }
        if err := mem.Store(ctx, memory.Record{
            ID:       f,
            Text:     string(data),
            Metadata: map[string]string{"source": "docs", "path": f},
        }); err != nil {
            log.Printf("failed to store %s: %v", f, err)
        }
    }

    // Search
    results, _ := mem.Search(ctx, "how does the authentication flow work?", 5)
    for _, r := range results {
        fmt.Printf("[%.3f] %s (%s)\n", r.Score, r.ID, r.Source)
        fmt.Printf("  %s\n\n", r.Text[:100])
    }
}
```

## Next Steps

- [Embeddings](/memory/embeddings) -- detailed embedding provider guide
- [Choosing a Backend](/memory/backends/) -- backend comparison and migration
- [SQLite Backend](/memory/backends/sqlite) -- default backend details and limitations
