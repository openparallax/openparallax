package memory

import (
	"context"
	"sort"

	"github.com/openparallax/openparallax/internal/storage"
)

// HybridSearchResult is a merged search result from vector + keyword search.
type HybridSearchResult struct {
	ID        string
	Path      string
	StartLine int
	EndLine   int
	Text      string
	Score     float64
	Source    string // "vector", "keyword", or "hybrid"
}

// HybridSearch performs vector similarity + FTS5 keyword search, merged with
// configurable weights. Falls back to FTS5-only if no embedding provider.
func HybridSearch(ctx context.Context, db *storage.DB, embedder EmbeddingProvider, vector VectorSearcher, query string, limit int) ([]HybridSearchResult, error) {
	candidates := limit * 3

	// FTS5 keyword search (always available).
	keywordResults, _ := db.SearchChunksFTS(query, candidates)

	// Vector search (if embedding provider and searcher available).
	var vectorResults []VectorResult
	if embedder != nil && vector != nil {
		embeddings, err := embedder.Embed(ctx, []string{query})
		if err == nil && len(embeddings) > 0 {
			vectorResults, _ = vector.Search(embeddings[0], candidates)
		}
	}

	if len(vectorResults) == 0 {
		return convertKeywordResults(keywordResults, limit), nil
	}

	// Build vector result map for merging.
	vectorMap := make(map[string]float64)
	for _, vr := range vectorResults {
		vectorMap[vr.ID] = vr.Score
	}

	// Build keyword result map.
	keywordMap := make(map[string]float64)
	maxKeywordScore := 0.0
	for _, kr := range keywordResults {
		if kr.Score > maxKeywordScore {
			maxKeywordScore = kr.Score
		}
	}
	if maxKeywordScore > 0 {
		for _, kr := range keywordResults {
			keywordMap[kr.ID] = kr.Score / maxKeywordScore // normalize to 0-1
		}
	}

	// Merge all unique IDs with weighted scoring.
	allIDs := make(map[string]bool)
	for _, vr := range vectorResults {
		allIDs[vr.ID] = true
	}
	for _, kr := range keywordResults {
		allIDs[kr.ID] = true
	}

	type scored struct {
		ID    string
		Score float64
	}
	var merged []scored
	for id := range allIDs {
		vs := vectorMap[id]
		ks := keywordMap[id]
		combined := vs*0.7 + ks*0.3
		merged = append(merged, scored{id, combined})
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	if len(merged) > limit {
		merged = merged[:limit]
	}

	// Look up chunk details.
	var results []HybridSearchResult
	for _, m := range merged {
		chunk, err := db.GetChunkByID(m.ID)
		if err != nil {
			continue
		}
		source := "hybrid"
		if vectorMap[m.ID] > 0 && keywordMap[m.ID] == 0 {
			source = "vector"
		} else if keywordMap[m.ID] > 0 && vectorMap[m.ID] == 0 {
			source = "keyword"
		}
		results = append(results, HybridSearchResult{
			ID: m.ID, Path: chunk.Path, StartLine: chunk.StartLine,
			EndLine: chunk.EndLine, Text: chunk.Text, Score: m.Score, Source: source,
		})
	}

	return results, nil
}

func convertKeywordResults(results []storage.ChunkSearchResult, limit int) []HybridSearchResult {
	if len(results) > limit {
		results = results[:limit]
	}
	out := make([]HybridSearchResult, len(results))
	for i, r := range results {
		out[i] = HybridSearchResult{
			ID: r.ID, Path: r.Path, StartLine: r.StartLine,
			EndLine: r.EndLine, Text: r.Text, Score: r.Score, Source: "keyword",
		}
	}
	return out
}
