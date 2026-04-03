---
outline: deep
---

# Weaviate


Open-source vector database with native hybrid search (vector + BM25) and schema-aware storage.

## Overview

Weaviate is an open-source vector database that provides:

- **Native hybrid search** -- combines vector similarity and BM25 keyword scoring in a single query
- **Schema-aware storage** -- define classes with typed properties
- **HNSW index** -- fast approximate nearest neighbor search
- **Multi-tenancy** -- built-in tenant isolation
- **GraphQL API** -- flexible query interface
- **Weaviate Cloud** -- managed hosting with free sandbox tier

## When to Use

- **Native hybrid search** -- you want vector + BM25 merged at the database level, not application level
- **Schema-aware data** -- your records have structured properties you want to filter and aggregate
- **Multi-tenancy** -- you need per-tenant data isolation
- **GraphQL preference** -- your stack uses GraphQL
- **Open-source priority** -- you want to self-host with full source code access

## Setup

### Self-Hosted (Docker)

```bash
docker run -d --name weaviate \
  -p 8080:8080 \
  -p 50051:50051 \
  -e QUERY_DEFAULTS_LIMIT=25 \
  -e AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED=true \
  -e PERSISTENCE_DATA_PATH=/var/lib/weaviate \
  -v weaviate_data:/var/lib/weaviate \
  cr.weaviate.io/semitechnologies/weaviate
```

### Weaviate Cloud

Sign up at [console.weaviate.cloud](https://console.weaviate.cloud) and create a cluster. The sandbox tier is free.

### Create the Store

```go
import "github.com/openparallax/openparallax/memory/weaviate"

store, err := weaviate.NewStore(weaviate.Options{
    URL:       "http://localhost:8080",
    ClassName: "Memory",
})
if err != nil {
    log.Fatal(err)
}
defer store.Close()
```

```python
# Python
from openparallax_memory import WeaviateStore

store = WeaviateStore(
    url="http://localhost:8080",
    class_name="Memory",
)
```

```typescript
// Node.js
import { WeaviateStore } from '@openparallax/memory'

const store = new WeaviateStore({
  url: 'http://localhost:8080',
  className: 'Memory',
})
```

### Weaviate Cloud Connection

```go
store, _ := weaviate.NewStore(weaviate.Options{
    URL:       "https://your-cluster.weaviate.network",
    ClassName: "Memory",
    APIKey:    os.Getenv("WEAVIATE_API_KEY"),
})
```

## Schema Configuration

The store creates a schema automatically, but you can customize it:

```go
store, _ := weaviate.NewStore(weaviate.Options{
    URL:       "http://localhost:8080",
    ClassName: "Memory",
    Properties: []weaviate.Property{
        {Name: "text", DataType: "text"},
        {Name: "source", DataType: "text", Tokenization: "field"},
        {Name: "date", DataType: "date"},
        {Name: "tags", DataType: "text[]"},
    },
    VectorIndexType: "hnsw",
    VectorIndexConfig: map[string]any{
        "distance":       "cosine",
        "efConstruction": 128,
        "maxConnections": 64,
    },
})
```

## Hybrid Search

Weaviate's hybrid search combines vector similarity and BM25 keyword scoring at the database level. This is more efficient than application-level merging because the database can optimize the combined query.

```go
// Hybrid search is the default behavior
results, _ := store.Search(ctx, "deployment pipeline configuration", 10)
```

You can control the balance between vector and keyword scoring:

```go
results, _ := store.Search(ctx, query, 10,
    weaviate.WithAlpha(0.75),  // 0 = pure keyword, 1 = pure vector, 0.75 = 75% vector
)
```

### How It Works

1. **BM25 search** ranks objects by keyword relevance
2. **Vector search** ranks objects by cosine similarity
3. **Fusion** merges both ranked lists using reciprocal rank fusion or relative score fusion
4. A single, unified result list is returned

This happens in a single database round-trip, unlike application-level hybrid search which requires two separate queries.

## Filtering

```go
results, _ := store.Search(ctx, queryVector, 10,
    weaviate.WithWhere(map[string]any{
        "path":     []string{"source"},
        "operator": "Equal",
        "valueText": "meeting-notes",
    }),
)
```

Supported operators:
- `Equal`, `NotEqual` -- equality
- `GreaterThan`, `LessThan`, `GreaterThanEqual`, `LessThanEqual` -- comparison
- `Like` -- wildcard matching
- `ContainsAny`, `ContainsAll` -- array operations
- `And`, `Or` -- boolean combinations

## Multi-Tenancy

Weaviate supports built-in multi-tenancy for per-user or per-organization data isolation:

```go
store, _ := weaviate.NewStore(weaviate.Options{
    URL:          "http://localhost:8080",
    ClassName:    "Memory",
    MultiTenancy: true,
    TenantID:     "user-123",
})
```

Each tenant's data is stored and indexed independently. Queries against one tenant never see another tenant's data.

## Performance

Benchmarks on a 4-core machine with SSD:

| Vectors | Query (hybrid, top-10) | Query (vector-only, top-10) |
|---------|----------------------|---------------------------|
| 10,000 | < 5ms | < 2ms |
| 100,000 | < 10ms | < 5ms |
| 1,000,000 | < 25ms | < 10ms |

## Limitations

- **More complex setup** than SQLite or Pinecone
- **No ACID transactions** -- eventual consistency
- **Higher memory usage** than pgvector for equivalent workloads
- **GraphQL learning curve** for advanced queries

## Next Steps

- [Choosing a Backend](/memory/backends/) -- comparison with other backends
- [ChromaDB](/memory/backends/chroma) -- simpler alternative for prototyping
- [PostgreSQL + pgvector](/memory/backends/pgvector) -- ACID alternative
- [Embeddings](/memory/embeddings) -- configuring embedding providers
