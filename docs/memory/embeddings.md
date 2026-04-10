---
outline: deep
---

# Embeddings


How embeddings work, which providers are supported, and how to choose the right model for your use case.

## What Are Embeddings?

An embedding is a vector -- a list of floating-point numbers -- that represents the *meaning* of a piece of text. Similar texts produce similar vectors. Dissimilar texts produce distant vectors. This is what enables semantic search: instead of matching keywords, you measure the mathematical distance between meaning.

```
"The server is overloaded"      → [0.12, -0.34, 0.89, 0.02, ...]
"High CPU usage on production"  → [0.11, -0.31, 0.87, 0.05, ...]  ← similar!
"My favorite color is blue"     → [-0.67, 0.42, -0.11, 0.93, ...] ← different
```

The distance between the first two vectors is small (cosine similarity ~0.95), because they express related concepts. The third vector is far away because it is about something unrelated.

### Dimensions

Each embedding model produces vectors of a fixed size, called the *dimension*. Common dimensions:

| Dimensions | Model | Notes |
|-----------|-------|-------|
| 384 | `all-MiniLM-L6-v2` | Small, fast, lower quality |
| 768 | `nomic-embed-text`, `text-embedding-004` | Good balance |
| 1536 | `text-embedding-3-small`, `text-embedding-ada-002` | High quality |
| 3072 | `text-embedding-3-large` | Highest quality, more storage |

Higher dimensions capture more nuance but use more storage and memory. For most use cases, 768 or 1536 dimensions provide excellent results.

::: warning Dimension Consistency
All vectors in a store must have the same dimension. If you switch embedding models, you must re-embed all existing records. Mixing dimensions causes search failures.
:::

### Cosine Similarity

Memory uses cosine similarity to compare vectors. It measures the angle between two vectors, ignoring magnitude:

```
cosine_similarity(A, B) = (A . B) / (|A| * |B|)
```

- **1.0** = identical direction (same meaning)
- **0.0** = orthogonal (unrelated)
- **-1.0** = opposite direction (opposite meaning)

In practice, most embedding models produce values between 0.0 and 1.0. Results above 0.7 are typically relevant; above 0.85 is a strong match.

## Supported Providers

### OpenAI

The most widely used embedding API. Excellent quality across all languages and domains.

```go
embedder := memory.NewEmbeddingProvider(memory.EmbeddingConfig{
    Provider: "openai",
    Model:    "text-embedding-3-small",
    APIKey:   os.Getenv("OPENAI_API_KEY"),
})
```

| Model | Dimensions | Max Tokens | Price (per 1M tokens) | Notes |
|-------|-----------|-----------|----------------------|-------|
| `text-embedding-3-small` | 1536 | 8191 | $0.02 | Best value. Recommended default. |
| `text-embedding-3-large` | 3072 | 8191 | $0.13 | Highest quality. Use for critical search. |
| `text-embedding-ada-002` | 1536 | 8191 | $0.10 | Legacy. Use 3-small instead. |

**When to use:** Default choice. Best quality/cost ratio. Works well for all text types (code, prose, technical docs, conversations).

**Configuration:**

```python
# Python
EmbeddingConfig(provider="openai", model="text-embedding-3-small")
```

```typescript
// Node.js
{ provider: 'openai', model: 'text-embedding-3-small' }
```

**API key resolution order:**
1. Explicit `APIKey` / `api_key` field
2. Environment variable from `APIKeyEnv` / `api_key_env` field
3. `OPENAI_API_KEY` environment variable

**Custom base URL** (for OpenAI-compatible APIs like Azure, Together AI, or local proxies):

```go
embedder := memory.NewEmbeddingProvider(memory.EmbeddingConfig{
    Provider: "openai",
    Model:    "text-embedding-3-small",
    BaseURL:  "https://your-azure-endpoint.openai.azure.com/v1",
    APIKey:   os.Getenv("AZURE_OPENAI_KEY"),
})
```

### Google

Google's embedding models via the Generative Language API.

```go
embedder := memory.NewEmbeddingProvider(memory.EmbeddingConfig{
    Provider: "google",
    Model:    "text-embedding-004",
    APIKey:   os.Getenv("GOOGLE_AI_API_KEY"),
})
```

| Model | Dimensions | Notes |
|-------|-----------|-------|
| `text-embedding-004` | 768 | Current recommended model. Good quality. |

**When to use:** If you already have Google Cloud infrastructure or prefer Google's pricing. Quality is competitive with OpenAI `text-embedding-3-small`.

**API key resolution order:**
1. Explicit `APIKey` / `api_key` field
2. Environment variable from `APIKeyEnv` / `api_key_env` field
3. `GOOGLE_API_KEY` environment variable

::: info Note
Google's embedding API processes texts one at a time (no batch endpoint). Memory handles this internally, but embedding large numbers of records is slower than with OpenAI's batch API.
:::

### Ollama (Local)

Run embedding models locally on your machine. Free, private, no API key required.

```go
embedder := memory.NewEmbeddingProvider(memory.EmbeddingConfig{
    Provider: "ollama",
    Model:    "nomic-embed-text",
    BaseURL:  "http://localhost:11434", // default
})
```

**Setup:**

```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Pull an embedding model
ollama pull nomic-embed-text

# Ollama serves on localhost:11434 by default
```

| Model | Dimensions | Size | Notes |
|-------|-----------|------|-------|
| `nomic-embed-text` | 768 | 274 MB | Best local quality. Recommended. |
| `all-minilm` | 384 | 45 MB | Smallest. Fastest. Lower quality. |
| `mxbai-embed-large` | 1024 | 670 MB | Higher quality, more resources. |

**When to use:** Privacy-sensitive workloads. Offline operation. Development without API costs. Air-gapped environments.

**Considerations:**
- Quality is good but not as high as OpenAI `text-embedding-3-small`
- Speed depends on your hardware (GPU significantly faster than CPU)
- No API costs, but uses local compute resources
- Processes texts one at a time (no batch endpoint)

## Custom Embedding Providers

Implement the `EmbeddingProvider` interface to use any embedding API:

### Go

```go
type EmbeddingProvider interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int
    ModelID() string
}
```

Example -- using Cohere's embedding API:

```go
type cohereEmbedder struct {
    apiKey string
    model  string
}

func (e *cohereEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    body := map[string]any{
        "texts":      texts,
        "model":      e.model,
        "input_type": "search_document",
    }
    jsonBody, _ := json.Marshal(body)

    req, _ := http.NewRequestWithContext(ctx, "POST",
        "https://api.cohere.ai/v1/embed", bytes.NewReader(jsonBody))
    req.Header.Set("Authorization", "Bearer "+e.apiKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result struct {
        Embeddings [][]float32 `json:"embeddings"`
    }
    json.NewDecoder(resp.Body).Decode(&result)
    return result.Embeddings, nil
}

func (e *cohereEmbedder) Dimension() int  { return 1024 }
func (e *cohereEmbedder) ModelID() string { return e.model }

// Use it
mem := memory.New(store, memory.WithEmbedder(&cohereEmbedder{
    apiKey: os.Getenv("COHERE_API_KEY"),
    model:  "embed-english-v3.0",
}))
```

### Python

```python
from openparallax_memory import Memory, SQLiteStore, CustomEmbedder

class CohereEmbedder(CustomEmbedder):
    def embed(self, texts: list[str]) -> list[list[float]]:
        import cohere
        co = cohere.Client(api_key=os.environ["COHERE_API_KEY"])
        response = co.embed(texts=texts, model="embed-english-v3.0", input_type="search_document")
        return response.embeddings

    @property
    def dimension(self) -> int:
        return 1024

    @property
    def model_id(self) -> str:
        return "embed-english-v3.0"

mem = Memory(store=SQLiteStore("memory.db"), embedding=CohereEmbedder())
```

## Choosing a Model

### Decision Tree

```
Need highest quality?
  YES → OpenAI text-embedding-3-large (3072d)
  NO ↓

Need privacy / offline?
  YES → Ollama nomic-embed-text (768d)
  NO ↓

Cost-sensitive with moderate volume?
  YES → OpenAI text-embedding-3-small (1536d)
  NO ↓

Already on Google Cloud?
  YES → Google text-embedding-004 (768d)
  NO → OpenAI text-embedding-3-small (1536d)
```

### Comparison Table

| | OpenAI 3-small | OpenAI 3-large | Google 004 | Ollama nomic |
|--|:-:|:-:|:-:|:-:|
| **Quality** | High | Highest | Good | Good |
| **Dimensions** | 1536 | 3072 | 768 | 768 |
| **Speed** | Fast (batch) | Fast (batch) | Moderate | Hardware-dependent |
| **Cost** | $0.02/1M tok | $0.13/1M tok | Competitive | Free |
| **Privacy** | Cloud | Cloud | Cloud | Local |
| **Offline** | No | No | No | Yes |
| **Batch API** | Yes | Yes | No | No |

### Storage Cost per Vector

| Dimensions | Bytes per Vector | 100K Vectors | 1M Vectors |
|-----------|-----------------|-------------|-----------|
| 384 | 1,536 B | 146 MB | 1.4 GB |
| 768 | 3,072 B | 293 MB | 2.9 GB |
| 1536 | 6,144 B | 586 MB | 5.9 GB |
| 3072 | 12,288 B | 1.2 GB | 11.7 GB |

## Embedding Cache

Memory caches embeddings by content hash (SHA-256). If you store the same text content again -- even with a different record ID -- the cached embedding is reused without calling the embedding API.

This means:
- Re-indexing unchanged files costs zero API calls
- Updating a record with the same text does not re-embed
- The cache is stored in the database alongside the records

The cache key is `SHA256(text_content)` and the cache stores the embedding vector plus the model ID that produced it. If you change embedding models, the cache miss triggers fresh embedding.

## Batching

The embedding pipeline batches texts for efficiency. When indexing files, chunks are collected and sent in batches of 20 to the embedding API. OpenAI and compatible providers support batch embedding natively; Google and Ollama process one text at a time but Memory handles the iteration internally.

For manual batching in Go:

```go
texts := []string{"first document", "second document", "third document"}
embeddings, err := embedder.Embed(ctx, texts)
// embeddings[0] corresponds to texts[0], etc.
```

## Migration Between Models

Switching embedding models requires re-embedding all stored records because different models produce incompatible vector spaces.

1. Create a new store (or clear the existing one)
2. Configure the new embedding provider
3. Re-store all records

```go
// Re-embed everything with a new model
newEmbedder := memory.NewEmbeddingProvider(memory.EmbeddingConfig{
    Provider: "openai",
    Model:    "text-embedding-3-large", // upgrading from 3-small
})

newStore, _ := sqlite.NewStore("memory-v2.db")
newMem := memory.New(newStore, memory.WithEmbedder(newEmbedder))

// Re-store all records from the old store
for _, record := range allRecords {
    newMem.Store(ctx, record)
}
```

## Next Steps

- [Quick Start](/memory/quickstart) -- get Memory running in 5 minutes
- [Go Library](/memory/go) -- full API reference
- [Choosing a Backend](/memory/backends/) -- backend comparison and feature matrix
