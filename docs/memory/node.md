---
outline: deep
---

::: warning Planned — Not Yet Implemented
The API described below is planned but not yet implemented. The documented interface shows the intended design.
:::

# Node.js


Node.js wrapper for Memory with full TypeScript support. Communicates with a pre-built Go binary over JSON-RPC (stdin/stdout).

```bash
npm install @openparallax/memory
```

The Go binary is downloaded automatically on install for your platform (linux/darwin/windows, amd64/arm64), similar to how esbuild and Prisma work.

## Quick Start

```typescript
import { Memory, SQLiteStore } from '@openparallax/memory'

const store = new SQLiteStore('memory.db')
const mem = new Memory({
  store,
  embedding: {
    provider: 'openai',
    model: 'text-embedding-3-small',
  },
})

// Store a record
await mem.store({
  id: 'note-1',
  text: 'The deployment pipeline uses blue-green deploys with Kubernetes.',
  metadata: { source: 'ops-docs' },
})

// Search by meaning
const results = await mem.search('CI/CD and deployment strategy', { limit: 5 })
for (const r of results) {
  console.log(`[${r.score.toFixed(3)}] (${r.source}) ${r.id}: ${r.text.slice(0, 80)}`)
}

await store.close()
```

## TypeScript Types

```typescript
interface Record {
  id?: string                          // Auto-generated if omitted
  text: string                         // Text content to store and search
  metadata?: Record<string, string>    // Arbitrary key-value metadata
}

interface SearchResult {
  id: string                           // Record ID
  text: string                         // Matched text content
  score: number                        // Relevance score (0.0 to 1.0)
  source: 'vector' | 'keyword' | 'hybrid'
  metadata: Record<string, string>     // Record metadata
  path?: string                        // Source file path
  startLine?: number                   // Start line in source file
  endLine?: number                     // End line in source file
}

interface SearchOptions {
  limit?: number                       // Max results (default: 10)
  filters?: Record<string, string>     // Metadata filters
}

interface EmbeddingConfig {
  provider: 'openai' | 'google' | 'ollama'
  model?: string
  apiKey?: string                      // Or reads from env var
  apiKeyEnv?: string                   // Env var name for API key
  baseUrl?: string                     // Custom API endpoint
}

interface MemoryOptions {
  store: Store
  embedding?: EmbeddingConfig
  chunkSize?: number                   // Tokens per chunk (default: 400)
  chunkOverlap?: number                // Overlap tokens (default: 80)
  vectorWeight?: number                // Vector score weight (default: 0.7)
  keywordWeight?: number               // Keyword score weight (default: 0.3)
}
```

## Configuration

### Embedding Providers

```typescript
// OpenAI
const mem = new Memory({
  store,
  embedding: {
    provider: 'openai',
    model: 'text-embedding-3-small',  // 1536 dimensions
    // Reads OPENAI_API_KEY from env by default
  },
})

// OpenAI with explicit key
const mem = new Memory({
  store,
  embedding: {
    provider: 'openai',
    model: 'text-embedding-3-large',  // 3072 dimensions
    apiKey: process.env.OPENAI_API_KEY,
  },
})

// Google
const mem = new Memory({
  store,
  embedding: {
    provider: 'google',
    model: 'text-embedding-004',  // 768 dimensions
    apiKeyEnv: 'GOOGLE_AI_API_KEY',
  },
})

// Ollama (local, free)
const mem = new Memory({
  store,
  embedding: {
    provider: 'ollama',
    model: 'nomic-embed-text',  // 768 dimensions
    baseUrl: 'http://localhost:11434',
  },
})
```

### Memory Options

```typescript
const mem = new Memory({
  store,
  embedding: { provider: 'openai', model: 'text-embedding-3-small' },
  chunkSize: 400,       // tokens per chunk
  chunkOverlap: 80,     // overlap between chunks
  vectorWeight: 0.7,    // hybrid search vector weight
  keywordWeight: 0.3,   // hybrid search keyword weight
})
```

## Store Backends

### SQLite (Default)

```typescript
import { SQLiteStore } from '@openparallax/memory'

const store = new SQLiteStore('memory.db')
```

### PostgreSQL + pgvector

```typescript
import { PgvectorStore } from '@openparallax/memory'

const store = new PgvectorStore({
  dsn: 'postgres://user:pass@localhost:5432/memory',
  tableName: 'memories',
  dimensions: 1536,
  indexType: 'hnsw',
})
```

### Qdrant

```typescript
import { QdrantStore } from '@openparallax/memory'

const store = new QdrantStore({
  url: 'http://localhost:6333',
  collectionName: 'memories',
  dimensions: 1536,
  apiKey: process.env.QDRANT_API_KEY,
})
```

### Pinecone

```typescript
import { PineconeStore } from '@openparallax/memory'

const store = new PineconeStore({
  apiKey: process.env.PINECONE_API_KEY!,
  indexName: 'memories',
  namespace: 'default',
})
```

### Weaviate

```typescript
import { WeaviateStore } from '@openparallax/memory'

const store = new WeaviateStore({
  url: 'http://localhost:8080',
  className: 'Memory',
  apiKey: process.env.WEAVIATE_API_KEY,
})
```

### ChromaDB

```typescript
import { ChromaStore } from '@openparallax/memory'

const store = new ChromaStore({
  url: 'http://localhost:8000',
  collectionName: 'memories',
})
```

### Redis

```typescript
import { RedisStore } from '@openparallax/memory'

const store = new RedisStore({
  url: 'redis://localhost:6379',
  indexName: 'memory_idx',
  prefix: 'mem:',
  dimensions: 1536,
})
```

## Storing Records

### Basic Store

```typescript
await mem.store({
  id: 'note-1',
  text: 'The frontend uses Svelte 4 with TypeScript and Vite for bundling.',
  metadata: { source: 'tech-stack', date: '2026-03-15' },
})
```

### Auto-Generated ID

```typescript
const id = await mem.store({
  text: 'User prefers GitHub Issues over Jira for project tracking.',
})
console.log(`Stored with ID: ${id}`)
```

### Batch Store

```typescript
await mem.storeBatch([
  { id: 'doc-1', text: 'Architecture uses CQRS pattern.', metadata: { type: 'adr' } },
  { id: 'doc-2', text: 'Events are stored in append-only log.', metadata: { type: 'adr' } },
  { id: 'doc-3', text: 'Read models are eventually consistent.', metadata: { type: 'adr' } },
])
```

### Upsert

Storing with an existing ID replaces the record:

```typescript
await mem.store({ id: 'config', text: 'Using port 8080' })
await mem.store({ id: 'config', text: 'Migrated to port 3100' })  // replaces
```

## Searching

### Hybrid Search (Default)

```typescript
const results = await mem.search('authentication and authorization', { limit: 10 })

for (const r of results) {
  console.log(`[${r.score.toFixed(3)}] (${r.source}) ${r.id}`)
  console.log(`  ${r.text.slice(0, 100)}`)
  console.log(`  metadata:`, r.metadata)
}
```

### Vector-Only Search

```typescript
const results = await mem.searchVector('deployment automation', { limit: 10 })
```

### Keyword-Only Search (FTS5)

```typescript
const results = await mem.searchKeyword('CORS middleware', { limit: 10 })
```

### Search with Metadata Filtering

```typescript
const results = await mem.search('database schema', {
  limit: 10,
  filters: { type: 'architecture-decision' },
})
```

## Deleting Records

```typescript
// Delete by ID
await mem.delete('note-1')

// Delete by metadata filter
const deletedCount = await mem.deleteByFilter({ type: 'temporary' })
console.log(`Deleted ${deletedCount} records`)
```

## Error Handling

```typescript
import { MemoryError, StoreError, EmbeddingError } from '@openparallax/memory'

try {
  await mem.store({ id: 'doc-1', text: 'Some content' })
} catch (err) {
  if (err instanceof EmbeddingError) {
    console.error(`Embedding failed: ${err.message}`)
    // FTS5 keyword search still works without embeddings
  } else if (err instanceof StoreError) {
    console.error(`Backend error: ${err.message}`)
  } else if (err instanceof MemoryError) {
    console.error(`Memory error: ${err.message}`)
  }
}
```

## Complete Example

```typescript
import { readdir, readFile } from 'node:fs/promises'
import { join, relative } from 'node:path'
import { Memory, SQLiteStore } from '@openparallax/memory'

async function indexDirectory(docsDir: string, dbPath: string): Promise<Memory> {
  const store = new SQLiteStore(dbPath)
  const mem = new Memory({
    store,
    embedding: {
      provider: 'openai',
      model: 'text-embedding-3-small',
    },
  })

  const files = await readdir(docsDir, { recursive: true })
  const mdFiles = files.filter(f => f.endsWith('.md'))

  for (const file of mdFiles) {
    const fullPath = join(docsDir, file)
    const content = await readFile(fullPath, 'utf-8')
    await mem.store({
      id: file,
      text: content,
      metadata: {
        source: 'docs',
        path: fullPath,
        size: String(content.length),
      },
    })
    console.log(`Indexed: ${file}`)
  }

  return mem
}

async function main() {
  const mem = await indexDirectory('./docs', 'knowledge.db')

  const queries = [
    'how does authentication work?',
    'deployment process and CI/CD',
    'error handling patterns',
  ]

  for (const query of queries) {
    console.log(`\nQuery: "${query}"`)
    const results = await mem.search(query, { limit: 3 })
    for (const r of results) {
      console.log(`  [${r.score.toFixed(3)}] ${r.id} (${r.source})`)
      console.log(`    ${r.text.slice(0, 100).replace(/\n/g, ' ')}...`)
    }
  }
}

main().catch(console.error)
```

## How It Works

The Node.js package communicates with a pre-built Go binary over JSON-RPC (stdin/stdout) -- the same protocol used by MCP. The Go binary is compiled for all major platforms and downloaded automatically during `npm install`.

This architecture means:
- **No native addons** -- works with any Node.js version, no node-gyp
- **Same performance as the Go library** -- all computation runs in Go
- **Cross-platform** -- Linux, macOS, and Windows (amd64 and arm64)
- **Full TypeScript types** -- complete type definitions included

## Next Steps

- [Embeddings](/memory/embeddings) -- choosing and configuring embedding providers
- [Choosing a Backend](/memory/backends/) -- backend comparison table
- [Go Library](/memory/go) -- full Go API reference (the source of truth)
