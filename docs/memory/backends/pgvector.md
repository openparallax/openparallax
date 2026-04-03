---
outline: deep
---

# PostgreSQL + pgvector

<style>
:root {
  --vp-c-brand-1: #a855f7;
  --vp-c-brand-2: #9333ea;
  --vp-c-brand-3: #7e22ce;
  --vp-c-brand-soft: rgba(168, 85, 247, 0.14);
}
</style>

Production-grade vector search with ACID transactions, HNSW indexing, and native full-text search.

## Overview

PostgreSQL with the `pgvector` extension provides:

- **HNSW index** -- Approximate Nearest Neighbor search with sub-10ms queries at million-vector scale
- **IVFFlat index** -- Alternative ANN index with lower memory usage, slightly lower recall
- **ACID transactions** -- Full transactional guarantees for data integrity
- **tsvector/tsquery** -- PostgreSQL's built-in full-text search with ranking
- **Mature ecosystem** -- Managed hosting (Supabase, Neon, RDS), monitoring, backups, replication

## When to Use

- **Production deployments** with > 100K vectors
- **Multiple agents** or services sharing the same memory store
- **Existing PostgreSQL infrastructure** -- add vector search to what you already run
- **ACID requirements** -- when data integrity is critical
- **Read replicas** needed for horizontal read scaling

## Setup

### Install pgvector

```bash
# Ubuntu/Debian
sudo apt install postgresql-16-pgvector

# macOS (Homebrew)
brew install pgvector

# Docker
docker run -d --name pgvector \
  -e POSTGRES_PASSWORD=secret \
  -p 5432:5432 \
  pgvector/pgvector:pg16
```

### Enable the Extension

```sql
CREATE EXTENSION IF NOT EXISTS vector;
```

### Create the Store

```go
import "github.com/openparallax/openparallax/memory/pgvector"

store, err := pgvector.NewStore("postgres://user:pass@localhost:5432/memory", pgvector.Options{
    TableName:  "memories",     // default: "memory_records"
    Dimensions: 1536,           // must match your embedding model
    IndexType:  "hnsw",         // "hnsw" or "ivfflat"
})
if err != nil {
    log.Fatal(err)
}
defer store.Close()
```

```python
# Python
from openparallax_memory import PgvectorStore

store = PgvectorStore(
    dsn="postgres://user:pass@localhost:5432/memory",
    table_name="memories",
    dimensions=1536,
    index_type="hnsw",
)
```

```typescript
// Node.js
import { PgvectorStore } from '@openparallax/memory'

const store = new PgvectorStore({
  dsn: 'postgres://user:pass@localhost:5432/memory',
  tableName: 'memories',
  dimensions: 1536,
  indexType: 'hnsw',
})
```

## Schema

The pgvector backend creates this schema automatically:

```sql
CREATE TABLE memories (
    id         TEXT PRIMARY KEY,
    text       TEXT NOT NULL,
    embedding  vector(1536),
    metadata   JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- HNSW index for fast vector search
CREATE INDEX memories_embedding_idx ON memories
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

-- GIN index for metadata filtering
CREATE INDEX memories_metadata_idx ON memories USING gin (metadata);

-- Full-text search
ALTER TABLE memories ADD COLUMN tsv tsvector
    GENERATED ALWAYS AS (to_tsvector('english', text)) STORED;
CREATE INDEX memories_tsv_idx ON memories USING gin (tsv);
```

## Index Types

### HNSW (Recommended)

Hierarchical Navigable Small Worlds. Builds a multi-layer graph for fast approximate nearest neighbor search.

```go
store, _ := pgvector.NewStore(dsn, pgvector.Options{
    IndexType: "hnsw",
    HNSWOptions: pgvector.HNSWOptions{
        M:              16,    // connections per node (default: 16)
        EfConstruction: 64,    // construction quality (default: 64)
    },
})
```

| Property | Value |
|----------|-------|
| Build time | Moderate (parallel) |
| Query time | O(log n) |
| Memory | Higher (graph in memory) |
| Recall | 95-99% (tunable) |
| Insert speed | Fast (no rebuild needed) |

**Tuning `ef_search` for query time vs. recall:**

```sql
-- Higher = better recall, slower queries
SET hnsw.ef_search = 100;  -- default: 40
```

### IVFFlat

Inverted File Index with flat quantization. Partitions vectors into lists and searches the closest lists.

```go
store, _ := pgvector.NewStore(dsn, pgvector.Options{
    IndexType: "ivfflat",
    IVFFlatOptions: pgvector.IVFFlatOptions{
        Lists: 100,  // number of partitions (default: sqrt(n))
    },
})
```

| Property | Value |
|----------|-------|
| Build time | Fast |
| Query time | O(n / lists) |
| Memory | Lower than HNSW |
| Recall | 90-95% (tunable) |
| Insert speed | Requires periodic reindex |

::: info
IVFFlat requires periodic reindexing after bulk inserts to maintain quality. HNSW does not. For most use cases, HNSW is the better choice.
:::

## Full-Text Search

PostgreSQL's built-in full-text search uses `tsvector` for indexing and `tsquery` for queries. The pgvector backend creates a generated column with the text vector and a GIN index for fast lookup.

```go
results, _ := store.Search(ctx, "CORS middleware configuration", 10)
```

Supports PostgreSQL's full query syntax:
- `web & server` -- AND
- `error | exception` -- OR
- `!deprecated` -- NOT
- `deploy:*` -- prefix match
- `'rate limiting'` -- phrase (with `phraseto_tsquery`)

## Connection Configuration

### Connection String

```
postgres://user:password@host:port/database?sslmode=require
```

### Connection Pool

```go
store, _ := pgvector.NewStore(dsn, pgvector.Options{
    MaxConnections:     20,    // max pool size (default: 10)
    MinConnections:     2,     // min idle connections (default: 2)
    ConnectionTimeout:  5 * time.Second,
})
```

### SSL/TLS

```go
store, _ := pgvector.NewStore(
    "postgres://user:pass@db.example.com:5432/memory?sslmode=verify-full&sslrootcert=/path/to/ca.crt",
    pgvector.Options{Dimensions: 1536},
)
```

## Performance

Benchmarks with HNSW index, 1536-dimension vectors:

| Vectors | Insert (batch) | Query (top-10) | Recall |
|---------|---------------|----------------|--------|
| 10,000 | 12s | < 2ms | 99.1% |
| 100,000 | 2 min | < 3ms | 98.5% |
| 1,000,000 | 20 min | < 8ms | 97.2% |
| 10,000,000 | 3.5 hrs | < 15ms | 96.1% |

Memory usage for HNSW index:
- 100K vectors (1536d): ~1.2 GB
- 1M vectors (1536d): ~12 GB

## Managed PostgreSQL Providers

Any PostgreSQL provider that supports the `pgvector` extension works:

| Provider | pgvector Support | Notes |
|----------|:---:|-------|
| **Supabase** | Yes | Built-in, one-click enable |
| **Neon** | Yes | Serverless, scale-to-zero |
| **AWS RDS** | Yes | Supported on PostgreSQL 15+ |
| **Google Cloud SQL** | Yes | Supported on PostgreSQL 15+ |
| **Azure Database** | Yes | Supported via extensions |
| **DigitalOcean** | Yes | Managed databases |
| **Railway** | Yes | Simple deployment |

## Next Steps

- [Choosing a Backend](/memory/backends/) -- comparison with other backends
- [SQLite](/memory/backends/sqlite) -- simpler alternative for personal use
- [Qdrant](/memory/backends/qdrant) -- alternative for vector-first workloads
- [Embeddings](/memory/embeddings) -- configuring embedding providers
