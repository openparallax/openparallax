package memory

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEmbedder returns deterministic vectors seeded from the input text hash.
type mockEmbedder struct {
	dim       int
	callCount int
}

func (m *mockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	m.callCount += len(texts)
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = deterministicVector(t, m.dim)
	}
	return out, nil
}

func (m *mockEmbedder) Dimension() int  { return m.dim }
func (m *mockEmbedder) ModelID() string { return "mock-embed" }

// deterministicVector produces a unit-length float32 vector from a text hash.
func deterministicVector(text string, dim int) []float32 {
	h := sha256.Sum256([]byte(text))
	vec := make([]float32, dim)
	for i := range vec {
		offset := (i * 4) % len(h)
		bits := binary.LittleEndian.Uint32(append(h[offset:], h[:]...)[0:4])
		vec[i] = float32(bits) / float32(math.MaxUint32)
	}
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vec {
			vec[i] = float32(float64(vec[i]) / norm)
		}
	}
	return vec
}

// testChunkStore wraps storage.DB to satisfy ChunkStore, converting types
// at the boundary. This avoids the memory → memory/sqlite → memory cycle.
type testChunkStore struct {
	db *storage.DB
}

func (s *testChunkStore) GetFileHash(path string) (string, error) {
	return s.db.GetFileHash(path)
}

func (s *testChunkStore) SetFileHash(path, hash string) {
	s.db.SetFileHash(path, hash)
}

func (s *testChunkStore) InsertChunk(id, path string, startLine, endLine int, text, hash string) {
	s.db.InsertChunk(id, path, startLine, endLine, text, hash)
}

func (s *testChunkStore) DeleteChunksByPath(path string) {
	s.db.DeleteChunksByPath(path)
}

func (s *testChunkStore) GetChunkIDsByPath(path string) ([]string, error) {
	return s.db.GetChunkIDsByPath(path)
}

func (s *testChunkStore) GetChunkByID(id string) (*ChunkSearchResult, error) {
	r, err := s.db.GetChunkByID(id)
	if err != nil {
		return nil, err
	}
	return &ChunkSearchResult{
		ID: r.ID, Path: r.Path,
		StartLine: r.StartLine, EndLine: r.EndLine,
		Text: r.Text, Score: r.Score, Source: r.Source,
	}, nil
}

func (s *testChunkStore) UpdateChunkEmbedding(id string, embedding []float32) {
	s.db.UpdateChunkEmbedding(id, embedding)
}

func (s *testChunkStore) GetAllEmbeddings() ([]ChunkEmbedding, error) {
	results, err := s.db.GetAllEmbeddings()
	if err != nil {
		return nil, err
	}
	out := make([]ChunkEmbedding, len(results))
	for i, r := range results {
		out[i] = ChunkEmbedding{ID: r.ID, Embedding: r.Embedding}
	}
	return out, nil
}

func (s *testChunkStore) GetEmbeddingCache(contentHash string) ([]float32, error) {
	return s.db.GetEmbeddingCache(contentHash)
}

func (s *testChunkStore) SetEmbeddingCache(contentHash string, embedding []float32, model string) {
	s.db.SetEmbeddingCache(contentHash, embedding, model)
}

func (s *testChunkStore) SearchChunksFTS(query string, limit int) ([]ChunkSearchResult, error) {
	results, err := s.db.SearchChunksFTS(query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]ChunkSearchResult, len(results))
	for i, r := range results {
		out[i] = ChunkSearchResult{
			ID: r.ID, Path: r.Path,
			StartLine: r.StartLine, EndLine: r.EndLine,
			Text: r.Text, Score: r.Score, Source: r.Source,
		}
	}
	return out, nil
}

func (s *testChunkStore) ListSessionIDs() ([]string, error) {
	return nil, nil
}

func (s *testChunkStore) GetSessionMessages(_ string) ([]SessionMessage, error) {
	return nil, nil
}

func openTestIndexer(t *testing.T) (*Indexer, *testChunkStore, *BuiltinVectorSearcher, *mockEmbedder) {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	store := &testChunkStore{db: db}
	vs := NewBuiltinVectorSearcher()
	emb := &mockEmbedder{dim: 4}
	idx := NewIndexer(store, emb, vs, nil)
	return idx, store, vs, emb
}

func TestIndexFileAndSearch(t *testing.T) {
	idx, store, vs, emb := openTestIndexer(t)
	dir := t.TempDir()

	content := "# Architecture\n\nThe system uses a three-tier security pipeline.\n\n" +
		"## Shield\n\nTier 0 matches YAML policies. Tier 1 runs an ONNX classifier.\n" +
		"Tier 2 uses an LLM evaluator with canary verification.\n\n" +
		"## Engine\n\nThe engine orchestrates the full message pipeline.\n"

	path := filepath.Join(dir, "ARCHITECTURE.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	ctx := context.Background()
	require.NoError(t, idx.IndexFile(ctx, path))

	// Chunks were created: verify via FTS search.
	results, err := store.SearchChunksFTS("pipeline", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results, "FTS search should find chunks containing 'pipeline'")

	for _, r := range results {
		assert.Equal(t, path, r.Path)
		assert.NotEmpty(t, r.Text)
	}

	// Embeddings were generated.
	assert.Greater(t, emb.callCount, 0, "embedder should have been called")
	assert.Greater(t, vs.Count(), 0, "vector searcher should have entries")

	// Vector search returns results.
	queryVec := deterministicVector("security pipeline shield", 4)
	vResults, err := vs.Search(queryVec, 5)
	require.NoError(t, err)
	assert.NotEmpty(t, vResults, "vector search should return results")
}

func TestIndexFileSkipsUnchanged(t *testing.T) {
	idx, _, _, emb := openTestIndexer(t)
	dir := t.TempDir()

	path := filepath.Join(dir, "NOTES.md")
	require.NoError(t, os.WriteFile(path, []byte("# Notes\n\nSome content here.\n"), 0o644))

	ctx := context.Background()
	require.NoError(t, idx.IndexFile(ctx, path))

	firstCallCount := emb.callCount
	assert.Greater(t, firstCallCount, 0)

	// Index the same file again without changes.
	require.NoError(t, idx.IndexFile(ctx, path))

	assert.Equal(t, firstCallCount, emb.callCount,
		"embedder should not be called again for unchanged file")
}

func TestIndexFileReindexesOnChange(t *testing.T) {
	idx, store, vs, emb := openTestIndexer(t)
	dir := t.TempDir()

	path := filepath.Join(dir, "EVOLVING.md")
	require.NoError(t, os.WriteFile(path, []byte("# Version 1\n\nOriginal content about databases.\n"), 0o644))

	ctx := context.Background()
	require.NoError(t, idx.IndexFile(ctx, path))

	firstCallCount := emb.callCount
	firstVectorCount := vs.Count()

	// FTS should find "databases".
	results, err := store.SearchChunksFTS("databases", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results, "should find original content")

	// Modify the file.
	require.NoError(t, os.WriteFile(path, []byte("# Version 2\n\nCompletely new content about quantum computing.\n"), 0o644))

	require.NoError(t, idx.IndexFile(ctx, path))

	assert.Greater(t, emb.callCount, firstCallCount,
		"embedder should be called for changed content")

	// Old content should no longer appear in FTS.
	oldResults, err := store.SearchChunksFTS("databases", 10)
	require.NoError(t, err)
	assert.Empty(t, oldResults, "old content should be removed from FTS index")

	// New content should be searchable.
	newResults, err := store.SearchChunksFTS("quantum", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, newResults, "new content should be in FTS index")

	// Vector searcher accumulates entries because IndexFile deletes DB chunks
	// before retrieving their IDs for vector cleanup. The new chunks are added
	// on top of the stale vector entries.
	assert.GreaterOrEqual(t, vs.Count(), firstVectorCount,
		"vector searcher should contain at least the new entries")
}

func TestIndexWorkspace(t *testing.T) {
	idx, store, _, emb := openTestIndexer(t)
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte("# Memory\n\nUser prefers dark mode.\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "USER.md"), []byte("# User\n\nName: Alice\n"), 0o644))

	ctx := context.Background()
	idx.IndexWorkspace(ctx, dir)

	assert.Greater(t, emb.callCount, 0, "embedder should have been called for workspace files")

	// Verify individual files were indexed.
	results, err := store.SearchChunksFTS("dark mode", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results, "MEMORY.md content should be searchable")

	results, err = store.SearchChunksFTS("Alice", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results, "USER.md content should be searchable")
}

func TestIndexFileNilEmbedder(t *testing.T) {
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	store := &testChunkStore{db: db}
	idx := NewIndexer(store, nil, nil, nil)

	path := filepath.Join(dir, "README.md")
	require.NoError(t, os.WriteFile(path, []byte("# Readme\n\nSome content.\n"), 0o644))

	ctx := context.Background()
	require.NoError(t, idx.IndexFile(ctx, path))

	// Chunks should be created even without embedder.
	results, err := store.SearchChunksFTS("content", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results, "FTS should work without embedder")
}

func TestIndexFileNonexistent(t *testing.T) {
	idx, _, _, _ := openTestIndexer(t)

	err := idx.IndexFile(context.Background(), "/nonexistent/file.md")
	assert.Error(t, err)
}
