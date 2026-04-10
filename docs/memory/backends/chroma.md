---
outline: deep
---

::: warning Planned — Not Yet Implemented
This backend is on the roadmap but not yet implemented. The interface is designed for future additions. Currently only SQLite is available.
:::

# ChromaDB


Simple, developer-friendly vector database popular in the AI/ML ecosystem.

## Overview

ChromaDB is an open-source embedding database designed for simplicity:

- **Simple API** -- minimal configuration, intuitive interface
- **HNSW index** -- fast approximate nearest neighbor search
- **Metadata filtering** -- filter by key-value metadata during search
- **Persistent storage** -- SQLite-backed persistence
- **In-memory mode** -- for testing and prototyping
- **Popular in AI/ML** -- widely used with LangChain, LlamaIndex, and other AI frameworks

## When to Use

- **AI/ML prototyping** -- simple API gets you started fast
- **LangChain/LlamaIndex integration** -- ChromaDB is a first-class citizen in these frameworks
- **Small to medium workloads** -- up to ~1M vectors
- **Development and testing** -- in-memory mode for unit tests
- **Python-heavy teams** -- ChromaDB's ecosystem is Python-native

## Setup

### Self-Hosted

```bash
# Docker
docker run -d --name chroma \
  -p 8000:8000 \
  -v chroma_data:/chroma/chroma \
  chromadb/chroma

# Or install the Python package for local mode
pip install chromadb
```

### Create the Store

```go
import "github.com/openparallax/openparallax/memory/chroma"

store, err := chroma.NewStore(chroma.Options{
    URL:            "http://localhost:8000",
    CollectionName: "memories",
})
if err != nil {
    log.Fatal(err)
}
defer store.Close()
```

```python
# Python
from openparallax_memory import ChromaStore

store = ChromaStore(
    url="http://localhost:8000",
    collection_name="memories",
)
```

```typescript
// Node.js
import { ChromaStore } from '@openparallax/memory'

const store = new ChromaStore({
  url: 'http://localhost:8000',
  collectionName: 'memories',
})
```

## Configuration

```go
store, _ := chroma.NewStore(chroma.Options{
    URL:            "http://localhost:8000",
    CollectionName: "memories",
    DistanceFunction: "cosine",  // "cosine", "l2", or "ip" (inner product)
    Tenant:         "default",   // multi-tenancy support
    Database:       "default",   // database isolation
})
```

## Metadata Filtering

ChromaDB supports metadata filtering during search:

```go
// Store with metadata
store.Upsert(ctx, "doc-1", embedding, map[string]string{
    "source": "meeting-notes",
    "date":   "2026-03-15",
})

// Search with filter
results, _ := store.Search(ctx, queryVector, 10,
    chroma.WithWhere(map[string]any{
        "source": "meeting-notes",
    }),
)

// Complex filters
results, _ := store.Search(ctx, queryVector, 10,
    chroma.WithWhere(map[string]any{
        "$and": []map[string]any{
            {"source": "meeting-notes"},
            {"date": map[string]any{"$gte": "2026-01-01"}},
        },
    }),
)
```

Supported operators:
- `$eq`, `$ne` -- equality
- `$gt`, `$gte`, `$lt`, `$lte` -- comparison
- `$in`, `$nin` -- set membership
- `$and`, `$or` -- boolean combinations

## Full-Text Search

ChromaDB supports basic document search through its `where_document` filter:

```go
results, _ := store.Search(ctx, queryVector, 10,
    chroma.WithWhereDocument(map[string]any{
        "$contains": "CORS middleware",
    }),
)
```

::: info
ChromaDB's document search is basic substring matching, not ranked full-text search. For BM25-style keyword search, Memory provides application-level hybrid search using a local FTS5 index.
:::

## Performance

| Vectors | Query (top-10) | Memory |
|---------|---------------|--------|
| 10,000 | < 5ms | ~200 MB |
| 100,000 | < 10ms | ~1.5 GB |
| 500,000 | < 30ms | ~7 GB |
| 1,000,000 | < 50ms | ~14 GB |

## Limitations

- **No native hybrid search** -- vector search only; keyword search via substring matching
- **No managed cloud** -- self-hosted only (as of 2026)
- **No ACID transactions** -- eventual consistency
- **Scaling ceiling** -- practical limit around 1M vectors per collection
- **Python-centric ecosystem** -- Go and Node.js support is via REST API

## Next Steps

- [Choosing a Backend](/memory/backends/) -- comparison with other backends
- [Weaviate](/memory/backends/weaviate) -- alternative with native hybrid search
- [SQLite](/memory/backends/sqlite) -- simpler alternative for embedded use
- [Embeddings](/memory/embeddings) -- configuring embedding providers
