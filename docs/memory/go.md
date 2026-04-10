---
outline: deep
---

# Go Library

The `memory` package manages workspace memory files, chunk-based indexing with embeddings, and hybrid search (FTS5 full-text + vector similarity).

```bash
go get github.com/openparallax/openparallax/memory
```

## Architecture

The memory module has two layers:

1. **Manager** — workspace memory files (SOUL.md, IDENTITY.md, USER.md, MEMORY.md, HEARTBEAT.md, AGENTS.md). Provides read, append, FTS5 search, session summarization, and relevant-context retrieval.
2. **Indexer** — chunk-based indexing pipeline. Splits files into overlapping chunks, embeds them with a configurable provider, and stores embeddings for vector search. Supports hybrid search (70% vector + 30% keyword blend).

```
                Manager                         Indexer
        ┌──────────────────┐           ┌──────────────────────┐
        │  Read / Append   │           │  IndexFile           │
        │  Search (FTS5)   │           │  IndexWorkspace      │
        │  SummarizeSession│           │  IndexSessions       │
        │  SearchRelevant  │           │                      │
        └────────┬─────────┘           └────────┬─────────────┘
                 │                               │
            Store (FTS5)                    ChunkStore + EmbeddingProvider
                 │                               │
            ┌────┴────┐                     ┌────┴────┐
            │ SQLite  │                     │ SQLite  │
            └─────────┘                     └─────────┘
```

## Manager

Manages the six workspace memory files and provides FTS5 search.

### NewManager

```go
func NewManager(workspace string, store Store, provider llm.Provider) *Manager
```

Creates a Manager and indexes all memory files on startup via `ReindexAll()`. The `provider` is used for session summarization (can be nil if summarization is not needed).

### Store Interface

The persistence interface Manager requires:

```go
type Store interface {
    IndexMemoryFile(path string, content string)
    ClearMemoryIndex()
    SearchMemory(query string, limit int) ([]SearchResult, error)
}
```

Implemented by `memory/sqlite.NewStore(db)` which wraps `internal/storage.DB`.

### Read

```go
func (m *Manager) Read(fileType FileType) (string, error)
```

Reads a workspace memory file. Returns `ErrFileNotFound` if the file does not exist.

### Append

```go
func (m *Manager) Append(fileType FileType, content string) error
```

Appends content to a memory file and reindexes it.

### Search

```go
func (m *Manager) Search(query string, limit int) ([]SearchResult, error)
```

FTS5 full-text search across all indexed memory files. Default limit is 20.

### SummarizeSession

```go
func (m *Manager) SummarizeSession(ctx context.Context, sessionTitle string, messages []llm.ChatMessage) error
```

Generates a 2-5 bullet summary of a conversation and appends it to MEMORY.md under a dated header. Skips sessions with fewer than 2 messages. Uses a 500-token cap for the summary LLM call.

### SearchRelevant

```go
func (m *Manager) SearchRelevant(query string, relevantK, recentK int) []string
```

Returns memory chunks relevant to a query plus the most recent MEMORY.md entries, deduplicated. Used by the context assembler to inject only pertinent memory into the system prompt.

### ReindexAll

```go
func (m *Manager) ReindexAll()
```

Rebuilds the FTS5 index from all workspace memory files.

## Indexer

Manages chunk-based indexing with optional embeddings for vector search.

### NewIndexer

```go
func NewIndexer(store ChunkStore, embedder EmbeddingProvider, vector VectorSearcher, log *logging.Logger) *Indexer
```

Creates an Indexer. Pass `nil` for `embedder` and `vector` to use FTS5-only mode.

### IndexFile

```go
func (idx *Indexer) IndexFile(ctx context.Context, path string) error
```

Chunks a file into overlapping segments (400 chars, 80 overlap), indexes them in the ChunkStore, and generates embeddings if a provider is configured. Skips unchanged files (tracked by content hash).

### IndexWorkspace

```go
func (idx *Indexer) IndexWorkspace(ctx context.Context, workspace string) error
```

Indexes all memory files in a workspace directory.

### IndexSessions

```go
func (idx *Indexer) IndexSessions(ctx context.Context) error
```

Indexes stored session transcripts for search.

### ChunkStore Interface

```go
type ChunkStore interface {
    GetFileHash(path string) (string, error)
    SetFileHash(path, hash string)
    InsertChunk(id, path string, startLine, endLine int, text, hash string)
    DeleteChunksByPath(path string)
    GetChunkIDsByPath(path string) ([]string, error)
    GetChunkByID(id string) (*ChunkSearchResult, error)
    UpdateChunkEmbedding(id string, embedding []float32)
    GetAllEmbeddings() ([]ChunkEmbedding, error)
    SearchChunksFTS(query string, limit int) ([]ChunkSearchResult, error)
    GetEmbeddingCache(contentHash string) ([]float32, error)
    SetEmbeddingCache(contentHash string, embedding []float32, model string)
}
```

Implemented by `internal/storage.DB`.

## Search

### HybridSearch

```go
func HybridSearch(ctx context.Context, store ChunkStore, embedder EmbeddingProvider, vector VectorSearcher, query string, limit int) ([]HybridSearchResult, error)
```

Performs vector similarity + FTS5 keyword search with a 70/30 score blend. Falls back to FTS5-only if no embedding provider is configured. Fetches `limit * 3` candidates from each source before merging.

### CosineSimilarity

```go
func CosineSimilarity(a, b []float32) float64
```

Pure Go cosine similarity between two vectors.

## Embedding

### EmbeddingProvider Interface

```go
type EmbeddingProvider interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int
    ModelID() string
}
```

### NewEmbeddingProvider

```go
func NewEmbeddingProvider(cfg EmbeddingConfig) EmbeddingProvider
```

Creates a provider from config. Returns nil if provider is `"none"` or empty.

Supported providers:

| Provider | Default Model | Dimensions |
|----------|--------------|------------|
| `openai` | `text-embedding-3-small` | 1536 |
| `google` | `text-embedding-004` | 768 |
| `ollama` | `nomic-embed-text` | 768 |

### VectorSearcher Interface

```go
type VectorSearcher interface {
    Insert(id string, embedding []float32) error
    Search(query []float32, limit int) ([]VectorResult, error)
    Delete(id string) error
}
```

The default implementation is `BuiltinVectorSearcher` — pure Go cosine similarity with an in-memory map. No external dependencies.

```go
vs := memory.NewBuiltinVectorSearcher()
```

## Types

### FileType

```go
type FileType string

const (
    MemorySoul      FileType = "SOUL.md"
    MemoryIdentity  FileType = "IDENTITY.md"
    MemoryUser      FileType = "USER.md"
    MemoryMain      FileType = "MEMORY.md"
    MemoryHeartbeat FileType = "HEARTBEAT.md"
    MemoryAgents    FileType = "AGENTS.md"
)
```

### SearchResult

FTS5 search result from memory files:

```go
type SearchResult struct {
    Path    string  `json:"path"`
    Section string  `json:"section"`
    Snippet string  `json:"snippet"`
    Score   float64 `json:"score"`
}
```

### ChunkSearchResult

Search result from the chunk index:

```go
type ChunkSearchResult struct {
    ID        string
    Path      string
    StartLine int
    EndLine   int
    Text      string
    Score     float64
    Source    string // "keyword", "vector", or "hybrid"
}
```

### HybridSearchResult

Merged result from hybrid search:

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

## Example

```go
package main

import (
    "context"
    "fmt"

    "github.com/openparallax/openparallax/memory"
)

func main() {
    // Manager: workspace memory files + FTS5 search
    mgr := memory.NewManager("/path/to/workspace", store, llmProvider)

    // Read a memory file.
    soul, _ := mgr.Read(memory.MemorySoul)
    fmt.Println(soul)

    // Append to MEMORY.md.
    mgr.Append(memory.MemoryMain, "## 2026-04-09\n- Decided to use PostgreSQL 16\n")

    // FTS5 search.
    results, _ := mgr.Search("PostgreSQL", 10)
    for _, r := range results {
        fmt.Printf("[%s] %s: %s\n", r.Path, r.Section, r.Snippet)
    }

    // Context-relevant memory for system prompt.
    chunks := mgr.SearchRelevant("database schema", 5, 5)
    for _, c := range chunks {
        fmt.Println(c)
    }

    // Indexer: chunk-based indexing with embeddings
    embedder := memory.NewEmbeddingProvider(memory.EmbeddingConfig{
        Provider:  "openai",
        Model:     "text-embedding-3-small",
        APIKeyEnv: "OPENAI_API_KEY",
    })
    vs := memory.NewBuiltinVectorSearcher()
    indexer := memory.NewIndexer(chunkStore, embedder, vs, logger)

    ctx := context.Background()
    indexer.IndexWorkspace(ctx, "/path/to/workspace")

    // Hybrid search (FTS5 + vector).
    hybrids, _ := memory.HybridSearch(ctx, chunkStore, embedder, vs, "database schema", 10)
    for _, h := range hybrids {
        fmt.Printf("[%s] %s:%d-%d (%.2f, %s)\n", h.Source, h.Path, h.StartLine, h.EndLine, h.Score, h.Source)
    }
}
```

## Key Source Files

| File | Purpose |
|------|---------|
| `memory/manager.go` | Manager: workspace file operations, FTS5 search, session summarization |
| `memory/indexer.go` | Indexer: chunk-based file indexing with embeddings |
| `memory/search.go` | HybridSearch: merged vector + keyword search |
| `memory/embedding.go` | EmbeddingProvider: OpenAI, Google, Ollama implementations |
| `memory/vector.go` | VectorSearcher: builtin pure-Go cosine similarity |
| `memory/types.go` | SearchResult, ChunkSearchResult, FileType constants |
| `memory/store.go` | ChunkStore interface |
| `memory/sqlite/store.go` | SQLite implementation wrapping internal/storage.DB |
