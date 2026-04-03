---
outline: deep
---

# Python

<style>
:root {
  --vp-c-brand-1: #a855f7;
  --vp-c-brand-2: #9333ea;
  --vp-c-brand-3: #7e22ce;
  --vp-c-brand-soft: rgba(168, 85, 247, 0.14);
}
</style>

Python wrapper for Memory. Communicates with a pre-built Go binary over JSON-RPC (stdin/stdout).

```bash
pip install openparallax-memory
```

The Go binary is downloaded automatically on install for your platform (linux/darwin/windows, amd64/arm64).

## Quick Start

```python
from openparallax_memory import Memory, SQLiteStore, EmbeddingConfig

store = SQLiteStore("memory.db")
mem = Memory(
    store=store,
    embedding=EmbeddingConfig(
        provider="openai",
        model="text-embedding-3-small",
    ),
)

# Store
mem.store(id="doc-1", text="The API gateway handles rate limiting at 1000 req/min.")

# Search
results = mem.search("rate limiting configuration", limit=5)
for r in results:
    print(f"[{r.score:.3f}] {r.id}: {r.text}")
```

## Configuration

### EmbeddingConfig

```python
from openparallax_memory import EmbeddingConfig

# OpenAI
embedding = EmbeddingConfig(
    provider="openai",
    model="text-embedding-3-small",  # or "text-embedding-3-large"
    api_key="sk-...",                # or reads OPENAI_API_KEY env var
)

# Google
embedding = EmbeddingConfig(
    provider="google",
    model="text-embedding-004",
    api_key_env="GOOGLE_AI_API_KEY",  # env var name
)

# Ollama (local)
embedding = EmbeddingConfig(
    provider="ollama",
    model="nomic-embed-text",
    base_url="http://localhost:11434",
)
```

### Memory Options

```python
mem = Memory(
    store=store,
    embedding=embedding,
    chunk_size=400,        # tokens per chunk (default: 400)
    chunk_overlap=80,      # overlap tokens (default: 80)
    vector_weight=0.7,     # weight for vector results (default: 0.7)
    keyword_weight=0.3,    # weight for keyword results (default: 0.3)
)
```

## Store Backends

### SQLite (Default)

```python
from openparallax_memory import SQLiteStore

store = SQLiteStore("memory.db")
```

### PostgreSQL + pgvector

```python
from openparallax_memory import PgvectorStore

store = PgvectorStore(
    dsn="postgres://user:pass@localhost:5432/memory",
    table_name="memories",
    dimensions=1536,
    index_type="hnsw",  # or "ivfflat"
)
```

### Qdrant

```python
from openparallax_memory import QdrantStore

store = QdrantStore(
    url="http://localhost:6333",
    collection_name="memories",
    dimensions=1536,
    api_key=None,  # for Qdrant Cloud
)
```

### Pinecone

```python
from openparallax_memory import PineconeStore

store = PineconeStore(
    api_key="pc-...",
    index_name="memories",
    namespace="default",
)
```

### Weaviate

```python
from openparallax_memory import WeaviateStore

store = WeaviateStore(
    url="http://localhost:8080",
    class_name="Memory",
    api_key=None,
)
```

### ChromaDB

```python
from openparallax_memory import ChromaStore

store = ChromaStore(
    url="http://localhost:8000",
    collection_name="memories",
)
```

### Redis

```python
from openparallax_memory import RedisStore

store = RedisStore(
    url="redis://localhost:6379",
    index_name="memory_idx",
    prefix="mem:",
    dimensions=1536,
)
```

## Storing Records

### Basic Store

```python
mem.store(
    id="note-1",
    text="User prefers vim keybindings and dark themes.",
    metadata={"source": "preferences", "date": "2026-03-15"},
)
```

### Auto-Generated ID

```python
record_id = mem.store(
    text="The staging environment uses port 8080.",
)
print(f"Stored with ID: {record_id}")
```

### Batch Store

```python
records = [
    {"id": "doc-1", "text": "Architecture uses event sourcing.", "metadata": {"type": "adr"}},
    {"id": "doc-2", "text": "Deployments go through staging first.", "metadata": {"type": "runbook"}},
    {"id": "doc-3", "text": "All API endpoints require JWT auth.", "metadata": {"type": "security"}},
]

mem.store_batch(records)
```

### Upsert

Storing with an existing ID replaces the record:

```python
mem.store(id="config", text="Using port 8080")
mem.store(id="config", text="Migrated to port 3100")  # replaces
```

## Searching

### Hybrid Search (Default)

```python
results = mem.search("database migration strategy", limit=10)

for r in results:
    print(f"[{r.score:.3f}] ({r.source}) {r.id}")
    print(f"  {r.text[:80]}")
    print(f"  metadata: {r.metadata}")
```

### Vector-Only Search

```python
results = mem.search_vector("deployment automation", limit=10)
```

### Keyword-Only Search (FTS5)

```python
results = mem.search_keyword("CORS middleware", limit=10)
```

### Search with Metadata Filtering

```python
results = mem.search(
    "authentication",
    limit=10,
    filters={"type": "security"},
)
```

## SearchResult

```python
@dataclass
class SearchResult:
    id: str                    # Record ID
    text: str                  # Matched text content
    score: float               # Relevance score (0.0 to 1.0)
    source: str                # "vector", "keyword", or "hybrid"
    metadata: dict[str, str]   # Record metadata
    path: str | None           # Source file path (if from file indexing)
    start_line: int | None     # Start line in source file
    end_line: int | None       # End line in source file
```

## Deleting Records

```python
# Delete by ID
mem.delete("note-1")

# Delete by metadata filter
deleted_count = mem.delete_by_filter({"type": "temporary"})
print(f"Deleted {deleted_count} records")
```

## Context Manager

```python
from openparallax_memory import Memory, SQLiteStore, EmbeddingConfig

with Memory(
    store=SQLiteStore("memory.db"),
    embedding=EmbeddingConfig(provider="openai", model="text-embedding-3-small"),
) as mem:
    mem.store(id="note-1", text="Important context for later.")
    results = mem.search("context", limit=5)
# Store is closed automatically
```

## Error Handling

```python
from openparallax_memory import MemoryError, StoreError, EmbeddingError

try:
    mem.store(id="doc-1", text="Some content")
except EmbeddingError as e:
    print(f"Embedding failed: {e}")
    # Memory still works -- FTS5 search is available without embeddings
except StoreError as e:
    print(f"Backend error: {e}")
except MemoryError as e:
    print(f"Memory error: {e}")
```

## Complete Example

```python
import os
from pathlib import Path
from openparallax_memory import Memory, SQLiteStore, EmbeddingConfig

def build_knowledge_base(docs_dir: str, db_path: str) -> Memory:
    """Index a directory of markdown files into a searchable knowledge base."""
    store = SQLiteStore(db_path)
    mem = Memory(
        store=store,
        embedding=EmbeddingConfig(
            provider="openai",
            model="text-embedding-3-small",
        ),
    )

    docs = Path(docs_dir)
    for md_file in docs.glob("**/*.md"):
        content = md_file.read_text()
        mem.store(
            id=str(md_file.relative_to(docs)),
            text=content,
            metadata={
                "source": "docs",
                "path": str(md_file),
                "size": str(len(content)),
            },
        )
        print(f"Indexed: {md_file.name}")

    return mem


def main():
    mem = build_knowledge_base("./docs", "knowledge.db")

    while True:
        query = input("\nSearch: ").strip()
        if not query:
            break

        results = mem.search(query, limit=5)
        if not results:
            print("  No results found.")
            continue

        for r in results:
            print(f"  [{r.score:.3f}] {r.id} ({r.source})")
            preview = r.text[:120].replace("\n", " ")
            print(f"    {preview}...")


if __name__ == "__main__":
    main()
```

## How It Works

The Python package communicates with a pre-built Go binary over JSON-RPC (stdin/stdout) -- the same protocol used by MCP. The Go binary is compiled for all major platforms and downloaded automatically during `pip install`.

This architecture means:
- **No CGo, no native extensions** -- pure Python package with a pre-built binary
- **Same performance as the Go library** -- all heavy lifting runs in Go
- **Cross-platform** -- works on Linux, macOS, and Windows (amd64 and arm64)

## Next Steps

- [Embeddings](/memory/embeddings) -- choosing and configuring embedding providers
- [Choosing a Backend](/memory/backends/) -- backend comparison table
- [Go Library](/memory/go) -- full Go API reference (the source of truth)
