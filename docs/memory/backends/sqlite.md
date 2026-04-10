---
outline: deep
---

# SQLite Backend


The default Memory backend. Zero dependencies. Single file. Pure Go.

## Overview

SQLite is the default backend for Memory. It uses:

- **SQLite via `modernc.org/sqlite`** -- a pure Go transpilation of SQLite. No CGo. No C compiler. No shared libraries.
- **FTS5** -- SQLite's built-in full-text search engine with BM25 ranking.
- **Brute-force cosine similarity** -- vectors are stored as BLOBs and compared using pure Go math.
- **WAL mode** -- Write-Ahead Logging for concurrent read/write performance.

This combination delivers a fully functional semantic memory system in a single file, with zero external dependencies.

## Setup

```go
import "github.com/openparallax/openparallax/memory/sqlite"

store, err := sqlite.NewStore("memory.db")
if err != nil {
    log.Fatal(err)
}
defer store.Close()
```

That is the entire setup. No server to install, no credentials to configure, no ports to open.

```python
# Python
from openparallax_memory import SQLiteStore
store = SQLiteStore("memory.db")
```

```typescript
// Node.js
import { SQLiteStore } from '@openparallax/memory'
const store = new SQLiteStore('memory.db')
```

## How It Works

### Schema

The SQLite backend creates these tables automatically:

```sql
-- Text chunks with optional embeddings
CREATE TABLE chunks (
    id         TEXT PRIMARY KEY,
    path       TEXT NOT NULL,
    start_line INTEGER NOT NULL,
    end_line   INTEGER NOT NULL,
    text       TEXT NOT NULL,
    hash       TEXT NOT NULL,
    embedding  BLOB
);

-- FTS5 full-text search index
CREATE VIRTUAL TABLE chunks_fts USING fts5(
    id, text, content='chunks', content_rowid='rowid'
);

-- File change detection
CREATE TABLE file_hashes (
    path TEXT PRIMARY KEY,
    hash TEXT NOT NULL
);

-- Embedding cache (avoid re-embedding identical content)
CREATE TABLE embedding_cache (
    content_hash TEXT PRIMARY KEY,
    embedding    BLOB NOT NULL,
    model        TEXT NOT NULL
);
```

### FTS5 Full-Text Search

Full-text search uses SQLite's FTS5 extension with BM25 ranking. FTS5 supports:

- **Boolean operators:** `CORS AND middleware`, `error OR exception`
- **Phrase queries:** `"rate limiting"`
- **Prefix matching:** `deploy*`
- **Column filtering:** specific fields
- **BM25 ranking:** relevance-scored results

```go
results, err := store.Search(ctx, "CORS middleware", 10)
```

The FTS5 index is synchronized with the `chunks` table using triggers. When chunks are inserted or deleted, the FTS5 index updates automatically.

### Vector Storage

Embedding vectors are stored as BLOBs in the `chunks` table. Each float32 value is serialized as 4 bytes in little-endian format:

```go
// 1536-dimension vector = 6,144 bytes per record
embedding := []float32{0.12, -0.34, 0.89, ...}
blob := vecSerialize(embedding)  // → []byte (len: 6144)
```

### Vector Search

The default vector search mode is `BuiltinVectorSearcher` -- a pure Go in-memory implementation that loads all vectors from the database at startup and performs brute-force cosine similarity on every query.

```go
// On startup: load all embeddings into memory
for _, e := range db.GetAllEmbeddings() {
    vectorSearcher.Insert(e.ID, e.Embedding)
}

// On search: linear scan with cosine similarity
results := vectorSearcher.Search(queryVector, 10)
```

### Optional: sqlite-vec Extension

If the `sqlite-vec` extension is installed, Memory uses it for native in-database vector queries instead of the builtin searcher:

```bash
# Download the extension
openparallax get-vector-ext
# Installed to ~/.openparallax/extensions/sqlite-vec.so
```

When detected, Memory creates a `vec0` virtual table and uses SQL-level vector queries:

```sql
SELECT id, distance
FROM chunks_vec
WHERE embedding MATCH ?
ORDER BY distance ASC
LIMIT ?
```

The extension is optional. Memory automatically falls back to the builtin searcher if the extension is not found.

::: info
The `sqlite-vec` extension is a CGo-free loadable extension (it is loaded at runtime, not linked at compile time). The OpenParallax binary itself remains zero-CGo.
:::

### Hybrid Search

Hybrid search merges results from both FTS5 and vector search using weighted scoring:

```
combined_score = (vector_score * 0.7) + (normalized_keyword_score * 0.3)
```

Keyword scores are normalized to a 0-1 range by dividing by the maximum keyword score in the result set. This prevents raw BM25 scores from dominating the merged results.

If no embedding provider is configured, hybrid search degrades gracefully to FTS5-only results.

## Performance Characteristics

Benchmarks on a typical developer laptop (M2 MacBook Air, NVMe SSD):

### FTS5 Search

| Records | Query Latency |
|---------|--------------|
| 1,000 | < 1ms |
| 10,000 | < 2ms |
| 100,000 | < 5ms |
| 1,000,000 | < 15ms |

FTS5 uses an inverted index. Performance scales logarithmically. Even at 1M records, keyword search is fast.

### Vector Search (Builtin)

| Vectors | Dimensions | Memory | Query Latency |
|---------|-----------|--------|--------------|
| 1,000 | 1536 | 6 MB | < 2ms |
| 10,000 | 1536 | 59 MB | < 10ms |
| 50,000 | 1536 | 293 MB | < 40ms |
| 100,000 | 1536 | 586 MB | < 80ms |
| 500,000 | 1536 | 2.9 GB | ~400ms |

The builtin searcher loads all vectors into memory and performs a linear scan on every query. Latency scales linearly with vector count.

### Database Size on Disk

| Records | Text Only | With 1536d Embeddings |
|---------|----------|----------------------|
| 1,000 | ~2 MB | ~8 MB |
| 10,000 | ~20 MB | ~80 MB |
| 100,000 | ~200 MB | ~800 MB |

## Limitations

::: danger Linear Scan
The SQLite backend uses brute-force cosine similarity for vector search. Every query compares against every stored vector. This is O(n) per query.
:::

### Scaling Limits

- **Query latency grows linearly** with vector count. At 100K vectors, expect ~80ms per query. At 500K, expect ~400ms.
- **All vectors must fit in memory.** The `BuiltinVectorSearcher` loads every embedding into RAM at startup. At 1536 dimensions, each vector consumes ~6 KB. 100K vectors need ~586 MB of RAM.
- **No approximate nearest neighbor (ANN) index.** There is no HNSW, IVFFlat, or other sub-linear search structure. Every query is exact.
- **Single-process only.** SQLite does not support concurrent writes from multiple processes. Use file locking or a single writer process.

### When SQLite Is Not Enough

Switch to a dedicated vector database when:

- You have **more than 100K vectors** and need sub-100ms latency
- You need **multiple processes writing concurrently**
- You want **horizontal scaling** (read replicas, sharding)
- You need **ANN search** for sub-linear query times at scale

See [Choosing a Backend](/memory/backends/) for alternatives.

## Roadmap

The SQLite backend's scaling limitation is an active area of research. The goal is to maintain the zero-CGo, single-file, zero-dependency experience while supporting larger vector collections.

### Under Exploration

**Pure-Go HNSW index.** Hierarchical Navigable Small Worlds is the algorithm used by pgvector, Qdrant, and most modern vector databases. A pure Go implementation could run as an in-process index alongside SQLite, providing O(log n) query times while keeping the zero-CGo constraint.

**Custom vector index on SQLite.** Store the HNSW graph structure in SQLite tables, combining the portability of a single-file database with sub-linear search. The index would be built incrementally as vectors are inserted and persisted across restarts.

**Quantization.** Reduce vector dimensionality or precision (e.g., int8 quantization) to lower memory usage and improve scan speed. OpenAI's `text-embedding-3-small` at 1536 dimensions could be quantized to ~384 bytes per vector without significant quality loss.

### Design Constraints

Any improvement must satisfy:

1. **Zero CGo.** No C compiler, no shared libraries at build time.
2. **Single binary.** No external processes or services required.
3. **Single file.** The database remains a single SQLite file.
4. **Backward compatible.** Existing databases continue to work without migration.

## Configuration

### WAL Mode

SQLite is configured in WAL mode for better concurrent read/write performance:

```go
store, err := sqlite.NewStore("memory.db")
// WAL mode is enabled automatically
```

### Custom Path

```go
// Relative path (relative to working directory)
store, _ := sqlite.NewStore("data/memory.db")

// Absolute path
store, _ := sqlite.NewStore("/var/lib/openparallax/memory.db")

// In-memory (for testing)
store, _ := sqlite.NewStore(":memory:")
```

## Next Steps

- [PostgreSQL + pgvector](/memory/backends/pgvector) -- production backend with ANN indexing
- [Qdrant](/memory/backends/qdrant) -- vector-first backend for large scale
- [Choosing a Backend](/memory/backends/) -- comparison table and decision tree
