---
outline: deep
---

# Qdrant


Purpose-built vector database with HNSW indexing, payload filtering, and distributed deployment.

## Overview

Qdrant is a vector similarity search engine built from the ground up for vector workloads. It provides:

- **HNSW index** -- fast approximate nearest neighbor search at any scale
- **Payload filtering** -- filter vectors by metadata before or during search
- **Distributed mode** -- horizontal scaling with sharding and replication
- **gRPC and REST API** -- high-performance binary protocol and HTTP interface
- **Qdrant Cloud** -- fully managed service with free tier

## When to Use

- **Vector-first workloads** where vector search is the primary access pattern
- **Large-scale deployments** with millions or billions of vectors
- **Payload filtering** -- complex metadata filters combined with vector search
- **Managed cloud** -- you want a hosted solution without managing infrastructure
- **Multi-tenant** -- collection-level isolation for different users or projects

## Setup

### Self-Hosted (Docker)

```bash
docker run -d --name qdrant \
  -p 6333:6333 \
  -p 6334:6334 \
  -v qdrant_storage:/qdrant/storage \
  qdrant/qdrant
```

### Qdrant Cloud

Sign up at [cloud.qdrant.io](https://cloud.qdrant.io) and create a cluster. The free tier includes 1GB of storage.

### Create the Store

```go
import "github.com/openparallax/openparallax/memory/qdrant"

store, err := qdrant.NewStore(qdrant.Options{
    URL:            "http://localhost:6333",
    CollectionName: "memories",
    Dimensions:     1536,
})
if err != nil {
    log.Fatal(err)
}
defer store.Close()
```

```python
# Python
from openparallax_memory import QdrantStore

store = QdrantStore(
    url="http://localhost:6333",
    collection_name="memories",
    dimensions=1536,
)
```

```typescript
// Node.js
import { QdrantStore } from '@openparallax/memory'

const store = new QdrantStore({
  url: 'http://localhost:6333',
  collectionName: 'memories',
  dimensions: 1536,
})
```

### Qdrant Cloud Connection

```go
store, _ := qdrant.NewStore(qdrant.Options{
    URL:            "https://your-cluster.qdrant.io:6333",
    CollectionName: "memories",
    Dimensions:     1536,
    APIKey:         os.Getenv("QDRANT_API_KEY"),
})
```

## Collection Configuration

The store creates the collection automatically with sensible defaults:

```go
store, _ := qdrant.NewStore(qdrant.Options{
    URL:            "http://localhost:6333",
    CollectionName: "memories",
    Dimensions:     1536,
    Distance:       "cosine",        // "cosine", "euclid", or "dot"
    OnDiskPayload:  false,           // store payloads on disk for large metadata
    HNSWConfig: &qdrant.HNSWConfig{
        M:              16,          // connections per node
        EfConstruct:    100,         // construction quality
        FullScanThreshold: 10000,    // switch to brute-force below this
    },
})
```

## Payload Filtering

Qdrant supports filtering vectors by metadata (called "payloads") during search. Filters are applied before or during the vector search, not after -- this is efficient even with selective filters.

```go
results, _ := store.Search(ctx, queryVector, 10,
    qdrant.WithFilter("source", "meeting-notes"),
    qdrant.WithFilter("date", qdrant.Range{GTE: "2026-01-01"}),
)
```

Supported filter types:
- **Match** -- exact value match
- **Range** -- numeric or date ranges
- **Geo** -- geographic bounding box or radius
- **Has ID** -- filter by point IDs
- **Nested** -- filter on nested payload fields
- **Boolean** -- AND, OR, NOT combinations

## Full-Text Search

Qdrant supports full-text search on payload fields via text indexes:

```go
store, _ := qdrant.NewStore(qdrant.Options{
    URL:            "http://localhost:6333",
    CollectionName: "memories",
    Dimensions:     1536,
    TextIndexFields: []string{"text"},  // create text index on "text" field
})

// Full-text search on payload
results, _ := store.SearchText(ctx, "CORS middleware", 10)
```

::: info
Qdrant's full-text search is basic compared to FTS5 or Elasticsearch. For advanced full-text needs, consider using Memory's hybrid search with application-level merging, or choose Weaviate for native hybrid search.
:::

## Performance

Benchmarks on a 4-core machine with SSD:

| Vectors | Dimensions | Index Build | Query (top-10) | Memory |
|---------|-----------|------------|----------------|--------|
| 100,000 | 1536 | 45s | < 2ms | 800 MB |
| 1,000,000 | 1536 | 8 min | < 5ms | 8 GB |
| 10,000,000 | 1536 | 1.5 hrs | < 10ms | ~80 GB |

Qdrant supports on-disk storage with memory-mapped files for collections that exceed available RAM. Query latency increases slightly but remains practical.

## Distributed Deployment

For large-scale deployments, Qdrant supports sharding and replication:

```yaml
# qdrant-config.yaml
storage:
  performance:
    max_search_threads: 0  # auto-detect
  optimizers:
    default_segment_number: 4

cluster:
  enabled: true
  p2p:
    port: 6335
```

- **Sharding** -- distribute vectors across multiple nodes
- **Replication** -- replicate shards for fault tolerance
- **Consensus** -- Raft-based consensus for cluster coordination

## Next Steps

- [Choosing a Backend](/memory/backends/) -- comparison with other backends
- [Pinecone](/memory/backends/pinecone) -- managed alternative
- [PostgreSQL + pgvector](/memory/backends/pgvector) -- ACID alternative
- [Embeddings](/memory/embeddings) -- configuring embedding providers
