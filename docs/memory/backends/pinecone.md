---
outline: deep
---

# Pinecone

<style>
:root {
  --vp-c-brand-1: #a855f7;
  --vp-c-brand-2: #9333ea;
  --vp-c-brand-3: #7e22ce;
  --vp-c-brand-soft: rgba(168, 85, 247, 0.14);
}
</style>

Fully managed vector database with serverless pricing and zero infrastructure.

## Overview

Pinecone is a managed vector database service that provides:

- **Serverless deployment** -- no infrastructure to manage
- **Scale-to-zero** -- pay only for what you use
- **Low-latency queries** -- sub-10ms at any scale
- **Metadata filtering** -- filter by key-value metadata during search
- **Namespaces** -- logical partitions within an index

## When to Use

- **Serverless applications** where you want zero ops burden
- **Variable traffic patterns** -- scale-to-zero pricing means you pay nothing when idle
- **Pay-per-query pricing** -- cost scales with actual usage, not provisioned capacity
- **Rapid prototyping** -- create an index via API, start storing vectors immediately
- **No self-hosting capability** -- you need a fully managed solution

## Setup

### Create an Account

Sign up at [pinecone.io](https://www.pinecone.io) and create an API key.

### Create an Index

```bash
# Via the Pinecone console, or via API:
curl -X POST https://api.pinecone.io/indexes \
  -H "Api-Key: $PINECONE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "memories",
    "dimension": 1536,
    "metric": "cosine",
    "spec": {
      "serverless": {
        "cloud": "aws",
        "region": "us-east-1"
      }
    }
  }'
```

### Create the Store

```go
import "github.com/openparallax/openparallax/memory/pinecone"

store, err := pinecone.NewStore(pinecone.Options{
    APIKey:    os.Getenv("PINECONE_API_KEY"),
    IndexName: "memories",
    Namespace: "default",   // optional logical partition
})
if err != nil {
    log.Fatal(err)
}
defer store.Close()
```

```python
# Python
from openparallax_memory import PineconeStore

store = PineconeStore(
    api_key=os.environ["PINECONE_API_KEY"],
    index_name="memories",
    namespace="default",
)
```

```typescript
// Node.js
import { PineconeStore } from '@openparallax/memory'

const store = new PineconeStore({
  apiKey: process.env.PINECONE_API_KEY!,
  indexName: 'memories',
  namespace: 'default',
})
```

## Configuration

### Index Settings

Index configuration is set at creation time via the Pinecone console or API:

| Setting | Options | Notes |
|---------|---------|-------|
| **Metric** | `cosine`, `euclidean`, `dotproduct` | Use `cosine` for text embeddings |
| **Dimensions** | Any positive integer | Must match your embedding model |
| **Cloud** | `aws`, `gcp`, `azure` | Region selection |
| **Plan** | Serverless, Pod-based | Serverless recommended for most cases |

### Namespaces

Namespaces provide logical partitions within a single index. Each namespace is searched independently.

```go
// Different namespaces for different contexts
agentStore, _ := pinecone.NewStore(pinecone.Options{
    APIKey: apiKey, IndexName: "memories", Namespace: "agent-atlas",
})
userStore, _ := pinecone.NewStore(pinecone.Options{
    APIKey: apiKey, IndexName: "memories", Namespace: "user-docs",
})
```

Use cases for namespaces:
- **Multi-tenant** -- one namespace per user or organization
- **Environment separation** -- `dev`, `staging`, `production`
- **Content types** -- `conversations`, `documents`, `code`

## Metadata Filtering

Pinecone supports metadata filters on upsert and query:

```go
// Store with metadata
store.Upsert(ctx, "doc-1", embedding, map[string]string{
    "source": "meeting-notes",
    "date":   "2026-03-15",
    "team":   "engineering",
})

// Search with metadata filter
results, _ := store.Search(ctx, queryVector, 10,
    pinecone.WithFilter(map[string]any{
        "source": "meeting-notes",
        "date":   map[string]string{"$gte": "2026-01-01"},
    }),
)
```

Supported filter operators:
- `$eq` -- equal
- `$ne` -- not equal
- `$gt`, `$gte`, `$lt`, `$lte` -- comparison
- `$in`, `$nin` -- set membership

## Full-Text Search

Pinecone is a vector-only database. It does not provide native full-text search. Memory handles this by:

1. Running vector search through Pinecone
2. Running FTS5 keyword search through a local SQLite index
3. Merging results at the application level

This hybrid approach is automatic when you use Memory's `Search()` method.

## Pricing

Pinecone's serverless pricing is based on:

| Metric | Cost |
|--------|------|
| **Storage** | ~$0.33/GB/month |
| **Read units** | ~$8.25/1M read units |
| **Write units** | ~$2.00/1M write units |

A single vector search query with top-10 results typically consumes 5-10 read units. At 1536 dimensions, 100K vectors use approximately 0.6 GB of storage.

Scale-to-zero means you pay nothing when the index is not being queried.

## Limitations

- **No full-text search** -- vector search only; keyword search requires a separate system
- **No self-hosting** -- cloud-only service
- **No ACID transactions** -- eventual consistency model
- **Vendor lock-in** -- proprietary API and infrastructure
- **Cold start latency** -- serverless indexes may have higher latency after periods of inactivity

## Next Steps

- [Choosing a Backend](/memory/backends/) -- comparison with other backends
- [Qdrant](/memory/backends/qdrant) -- self-hosted alternative with similar capabilities
- [PostgreSQL + pgvector](/memory/backends/pgvector) -- ACID alternative
- [Embeddings](/memory/embeddings) -- configuring embedding providers
