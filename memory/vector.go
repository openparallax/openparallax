package memory

import (
	"math"
	"sort"
	"sync"
)

// VectorResult is a single result from vector similarity search.
type VectorResult struct {
	ID    string
	Score float64
}

// VectorSearcher provides vector similarity search over chunk embeddings.
type VectorSearcher interface {
	Insert(id string, embedding []float32) error
	Search(query []float32, limit int) ([]VectorResult, error)
	Delete(id string) error
}

// BuiltinVectorSearcher uses pure Go cosine similarity.
// No external dependencies. Always available. Default mode.
type BuiltinVectorSearcher struct {
	mu      sync.RWMutex
	entries map[string][]float32
}

// NewBuiltinVectorSearcher creates a builtin vector searcher.
func NewBuiltinVectorSearcher() *BuiltinVectorSearcher {
	return &BuiltinVectorSearcher{entries: make(map[string][]float32)}
}

func (b *BuiltinVectorSearcher) Insert(id string, embedding []float32) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[id] = embedding
	return nil
}

func (b *BuiltinVectorSearcher) Delete(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.entries, id)
	return nil
}

func (b *BuiltinVectorSearcher) Search(query []float32, limit int) ([]VectorResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	type scored struct {
		ID    string
		Score float64
	}
	results := make([]scored, 0, len(b.entries))
	for id, emb := range b.entries {
		score := CosineSimilarity(query, emb)
		results = append(results, scored{id, score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	out := make([]VectorResult, len(results))
	for i, r := range results {
		out[i] = VectorResult(r)
	}
	return out, nil
}

// Count returns the number of indexed vectors.
func (b *BuiltinVectorSearcher) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.entries)
}

// CosineSimilarity computes the cosine similarity between two vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
