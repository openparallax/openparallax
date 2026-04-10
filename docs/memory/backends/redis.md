---
outline: deep
---

::: warning Planned — Not Yet Implemented
This backend is on the roadmap but not yet implemented. The interface is designed for future additions. Currently only SQLite is available.
:::

# Redis


Vector similarity search and full-text search in your existing Redis infrastructure via RediSearch.

## Overview

Redis with the RediSearch module provides:

- **Vector similarity search** -- HNSW and flat index support
- **Full-text search** -- RediSearch's inverted index with stemming and phonetic matching
- **Combined queries** -- vector search with full-text and metadata filters in a single query
- **Sub-millisecond latency** -- everything runs in-memory
- **Existing infrastructure** -- add semantic search to Redis you already run

## When to Use

- **Already running Redis** -- add vector search without deploying a new database
- **Low-latency requirements** -- sub-millisecond query times
- **Combined caching + search** -- use Redis for both application cache and semantic memory
- **Real-time workloads** -- high throughput with predictable latency
- **Full-text + vector in one query** -- RediSearch supports hybrid queries natively

## Setup

### Install Redis with RediSearch

```bash
# Docker (Redis Stack includes RediSearch)
docker run -d --name redis-stack \
  -p 6379:6379 \
  -p 8001:8001 \
  -v redis_data:/data \
  redis/redis-stack

# Or install the module separately
# https://redis.io/docs/latest/operate/oss_and_stack/install/install-stack/
```

### Verify RediSearch

```bash
redis-cli MODULE LIST
# Should include "search" module
```

### Create the Store

```go
import "github.com/openparallax/openparallax/memory/redis"

store, err := redis.NewStore(redis.Options{
    URL:        "redis://localhost:6379",
    IndexName:  "memory_idx",
    Prefix:     "mem:",
    Dimensions: 1536,
})
if err != nil {
    log.Fatal(err)
}
defer store.Close()
```

```python
# Python
from openparallax_memory import RedisStore

store = RedisStore(
    url="redis://localhost:6379",
    index_name="memory_idx",
    prefix="mem:",
    dimensions=1536,
)
```

```typescript
// Node.js
import { RedisStore } from '@openparallax/memory'

const store = new RedisStore({
  url: 'redis://localhost:6379',
  indexName: 'memory_idx',
  prefix: 'mem:',
  dimensions: 1536,
})
```

## Configuration

```go
store, _ := redis.NewStore(redis.Options{
    URL:        "redis://localhost:6379",
    Password:   os.Getenv("REDIS_PASSWORD"),
    IndexName:  "memory_idx",
    Prefix:     "mem:",           // key prefix for all records
    Dimensions: 1536,
    Distance:   "COSINE",        // "COSINE", "L2", or "IP"
    IndexType:  "HNSW",          // "HNSW" or "FLAT"
    HNSWConfig: &redis.HNSWConfig{
        M:              16,      // connections per node
        EfConstruction: 200,     // construction quality
        EfRuntime:      10,      // query quality
    },
})
```

### TLS Connection

```go
store, _ := redis.NewStore(redis.Options{
    URL: "rediss://user:pass@redis.example.com:6380",  // rediss:// for TLS
})
```

## Index Schema

The store creates a RediSearch index automatically:

```
FT.CREATE memory_idx ON HASH PREFIX 1 mem:
    SCHEMA
        text TEXT WEIGHT 1.0
        embedding VECTOR HNSW 6 TYPE FLOAT32 DIM 1536 DISTANCE_METRIC COSINE
        source TAG
        date TAG
```

Each record is stored as a Redis Hash:

```
HSET mem:doc-1
    text "The API uses JWT authentication with refresh tokens"
    embedding <binary vector data>
    source "security-docs"
    date "2026-03-15"
```

## Full-Text Search

RediSearch provides full-featured text search with:

- **Stemming** -- finds "running" when searching "run"
- **Phonetic matching** -- finds "Michael" when searching "Micheal"
- **Stop words** -- configurable stop word lists
- **Prefix matching** -- `deploy*`
- **Boolean operators** -- `CORS | middleware`, `error -deprecated`
- **Fuzzy matching** -- `%deployment%` (Levenshtein distance 1)

```go
results, _ := store.Search(ctx, "CORS middleware", 10)
```

## Combined Queries

RediSearch supports vector search combined with full-text and tag filters in a single query:

```go
results, _ := store.Search(ctx, queryVector, 10,
    redis.WithFilter("@source:{meeting-notes}"),
    redis.WithFilter("@date:{2026-03-*}"),
)
```

This executes as a single Redis command, filtering and ranking in one pass.

## Performance

Benchmarks on a single Redis instance (8 GB RAM):

| Vectors | Dimensions | Query (top-10) | Memory |
|---------|-----------|---------------|--------|
| 10,000 | 1536 | < 1ms | 150 MB |
| 100,000 | 1536 | < 2ms | 1.2 GB |
| 500,000 | 1536 | < 5ms | 5.5 GB |
| 1,000,000 | 1536 | < 8ms | 11 GB |

Redis keeps everything in memory, so query latency is consistently low. The tradeoff is memory usage -- each 1536-dimension vector uses ~6 KB of RAM.

## Persistence

Redis Stack supports two persistence modes:

- **RDB snapshots** -- periodic point-in-time snapshots to disk
- **AOF (Append-Only File)** -- logs every write operation

Configure in `redis.conf`:

```
# RDB snapshots
save 3600 1

# AOF logging
appendonly yes
appendfsync everysec
```

::: warning
Without persistence configured, Redis data is lost on restart. Always enable at least RDB snapshots for production use.
:::

## Managed Redis Providers

| Provider | RediSearch Support | Notes |
|----------|:-:|-------|
| **Redis Cloud** | Yes | Official managed service |
| **AWS ElastiCache** | Yes (Redis 7.0+) | With Redis Stack modules |
| **Azure Cache for Redis** | Yes (Enterprise) | Enterprise tier only |
| **Google Memorystore** | No | Does not support RediSearch |
| **Upstash** | No | Does not support RediSearch |

::: info
Not all managed Redis providers support the RediSearch module. Verify module support before choosing a provider.
:::

## Limitations

- **Memory-bound** -- all data must fit in RAM (no disk-only mode for vectors)
- **No ACID transactions** -- Redis transactions are limited compared to PostgreSQL
- **Module dependency** -- requires RediSearch module (not available on all managed providers)
- **No native hybrid scoring** -- combined queries use filtering, not weighted score fusion

## Next Steps

- [Choosing a Backend](/memory/backends/) -- comparison with other backends
- [PostgreSQL + pgvector](/memory/backends/pgvector) -- ACID alternative with disk-based storage
- [SQLite](/memory/backends/sqlite) -- embedded alternative
- [Embeddings](/memory/embeddings) -- configuring embedding providers
