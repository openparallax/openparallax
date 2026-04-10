---
outline: deep
---

# Choosing a Backend

::: info Current Status
Currently only the **SQLite** backend is implemented. The remaining backends (pgvector, Qdrant, Pinecone, Weaviate, ChromaDB, Redis) are planned. The `ChunkStore` interface is designed for future backend additions.
:::

Memory's `ChunkStore` interface is backend-agnostic. The SQLite implementation is the default and only current backend.

## Feature Matrix

| Feature | SQLite | pgvector | Qdrant | Pinecone | Weaviate | ChromaDB | Redis |
|---------|:------:|:--------:|:------:|:--------:|:--------:|:--------:|:-----:|
| **Vector search** | Brute-force | HNSW / IVFFlat | HNSW | Proprietary | HNSW | HNSW | HNSW |
| **Full-text search** | FTS5 | tsvector | Payload filter | Metadata filter | BM25 | Metadata filter | RediSearch |
| **Hybrid search** | App-level merge | App-level merge | Native | No | Native | No | App-level merge |
| **Zero dependencies** | Yes | No (PostgreSQL) | No (server) | No (cloud) | No (server) | No (server) | No (server) |
| **Self-hosted** | Yes | Yes | Yes | No | Yes | Yes | Yes |
| **Managed cloud** | No | Yes (many) | Yes | Yes | Yes | No | Yes |
| **ANN index** | No | Yes | Yes | Yes | Yes | Yes | Yes |
| **Max vectors (practical)** | ~100K | Millions | Billions | Billions | Millions | ~1M | Millions |
| **Transactions** | ACID | ACID | No | No | No | No | No |
| **Zero CGo** | Yes | Yes | Yes | Yes | Yes | Yes | Yes |

## Decision Tree

```
Just getting started or personal agent?
  → SQLite (zero setup, single file)

Need > 100K vectors?
  → Already have PostgreSQL? → pgvector
  → Want managed cloud?     → Pinecone or Qdrant Cloud
  → Want self-hosted?       → Qdrant or Weaviate

Need native hybrid search (vector + BM25)?
  → Weaviate

Already have Redis in your stack?
  → Redis + RediSearch

ML/AI prototyping, want simplest API?
  → ChromaDB

Production with ACID transactions?
  → PostgreSQL + pgvector
```

## When to Use Each Backend

### SQLite

**Best for:** Personal AI agents, development, single-user applications, embedded use.

The default backend. Zero dependencies. Single file database. Pure Go via `modernc.org/sqlite`. Uses FTS5 for full-text search and brute-force cosine similarity for vector search.

The brute-force approach scans every vector on each query. This is fast enough for up to ~100K vectors (sub-100ms on modern hardware). Beyond that, latency grows linearly.

[Full SQLite documentation](/memory/backends/sqlite)

### PostgreSQL + pgvector

**Best for:** Production deployments, existing PostgreSQL infrastructure, multi-agent systems, ACID-critical workloads.

Adds true ANN (Approximate Nearest Neighbor) indexing via HNSW or IVFFlat. Scales to millions of vectors with sub-10ms query times. Full-text search via PostgreSQL's built-in `tsvector`/`tsquery`. ACID transactions for data integrity.

[Full pgvector documentation](/memory/backends/pgvector)

### Qdrant

**Best for:** Vector-first workloads, large-scale deployments, payload filtering, managed cloud option.

Purpose-built vector database with HNSW indexing, payload filtering, and distributed deployment. Available self-hosted or as Qdrant Cloud. Handles billions of vectors.

[Full Qdrant documentation](/memory/backends/qdrant)

### Pinecone

**Best for:** Serverless vector search, pay-per-query pricing, minimal ops.

Fully managed vector database. No infrastructure to maintain. Scale-to-zero pricing. Excellent for applications with variable traffic patterns.

[Full Pinecone documentation](/memory/backends/pinecone)

### Weaviate

**Best for:** Native hybrid search (vector + BM25), schema-aware storage, complex filtering.

Open-source vector database with built-in hybrid search that combines vector similarity and BM25 keyword scoring in a single query, without application-level merging.

[Full Weaviate documentation](/memory/backends/weaviate)

### ChromaDB

**Best for:** AI/ML prototyping, simple API, Python-ecosystem integration.

Popular in the AI/ML community for its simplicity. Good for prototyping and smaller-scale deployments. Straightforward API without configuration overhead.

[Full ChromaDB documentation](/memory/backends/chroma)

### Redis

**Best for:** Existing Redis infrastructure, low-latency requirements, combined caching + search.

Redis with the RediSearch module provides vector similarity search and full-text search in your existing Redis deployment. Ideal when you already use Redis for caching and want to add semantic search without another database.

[Full Redis documentation](/memory/backends/redis)

## Backend Comparison by Use Case

### Personal Agent (< 50K vectors)

| | SQLite | pgvector | Qdrant |
|--|:--:|:--:|:--:|
| Setup time | 0 min | 15 min | 10 min |
| Dependencies | None | PostgreSQL | Docker |
| Query latency | < 50ms | < 5ms | < 5ms |
| Storage | Single file | Server process | Server process |
| **Recommendation** | **Use this** | Overkill | Overkill |

### Production Service (100K - 1M vectors)

| | SQLite | pgvector | Qdrant | Pinecone |
|--|:--:|:--:|:--:|:--:|
| Query latency | 200ms+ | < 5ms | < 5ms | < 10ms |
| Ops burden | None | Medium | Medium | None |
| Cost | Free | Self-host | Self-host or paid | Pay-per-query |
| Transactions | ACID | ACID | No | No |
| **Recommendation** | Too slow | **Use this** | Good option | Good option |

### Large Scale (> 1M vectors)

| | pgvector | Qdrant | Pinecone |
|--|:--:|:--:|:--:|
| Max vectors | Millions | Billions | Billions |
| Horizontal scaling | Read replicas | Native sharding | Managed |
| Query latency | < 10ms | < 5ms | < 10ms |
| Ops burden | Medium | Medium-High | None |
| **Recommendation** | Good | **Good** | **Good** |

## Migrating Between Backends

All backends implement the same `Store` interface. Migration is a data copy operation:

```go
// Read from old store
oldStore, _ := sqlite.NewStore("old-memory.db")
records, _ := oldStore.Export(ctx) // export all records

// Write to new store
newStore, _ := pgvector.NewStore("postgres://...", pgvector.Options{
    Dimensions: 1536,
    IndexType:  "hnsw",
})

for _, record := range records {
    newStore.Upsert(ctx, record.ID, record.Embedding, record.Metadata)
    newStore.Index(ctx, record.ID, record.Text, record.Metadata)
}
```

::: warning Re-embedding
If you change embedding models during migration, all vectors must be re-generated. Use the same embedding model to preserve vector compatibility.
:::

### Migration Checklist

1. **Verify embedding dimensions match.** The new backend must be configured for the same dimension as your embedding model.
2. **Export records from the old store.** Use `Export()` or iterate through records.
3. **Create the new store** with matching configuration.
4. **Import records** into the new store.
5. **Verify search results** by running the same queries against both stores.
6. **Update configuration** to point to the new store.
7. **Decommission the old store** after verifying everything works.

## Next Steps

- [SQLite](/memory/backends/sqlite) -- default backend details
- [PostgreSQL + pgvector](/memory/backends/pgvector) -- production backend
- [Qdrant](/memory/backends/qdrant) -- vector-first backend
- [Quick Start](/memory/quickstart) -- get Memory running in 5 minutes
