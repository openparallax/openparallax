// Package sqlite implements the memory.ChunkStore and memory.Store interfaces
// backed by the internal/storage SQLite database. This subpackage exists to
// break the circular dependency between memory and internal/storage.
package sqlite

import (
	"database/sql"

	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/memory"
)

// Store wraps a *storage.DB to satisfy both the memory.ChunkStore and
// memory.Store interfaces.
type Store struct {
	db *storage.DB
}

// NewStore creates a ChunkStore backed by the given storage.DB.
func NewStore(db *storage.DB) *Store {
	return &Store{db: db}
}

// GetFileHash returns the stored hash for a file path.
func (s *Store) GetFileHash(path string) (string, error) {
	return s.db.GetFileHash(path)
}

// SetFileHash stores or updates the hash for a file path.
func (s *Store) SetFileHash(path, hash string) {
	s.db.SetFileHash(path, hash)
}

// InsertChunk adds a text chunk to the chunks table and FTS5 index.
func (s *Store) InsertChunk(id, path string, startLine, endLine int, text, hash string) {
	s.db.InsertChunk(id, path, startLine, endLine, text, hash)
}

// DeleteChunksByPath removes all chunks for a given file path.
func (s *Store) DeleteChunksByPath(path string) {
	s.db.DeleteChunksByPath(path)
}

// GetChunkIDsByPath returns all chunk IDs for a given path.
func (s *Store) GetChunkIDsByPath(path string) ([]string, error) {
	return s.db.GetChunkIDsByPath(path)
}

// GetChunkByID retrieves a chunk by its ID, converting to memory types.
func (s *Store) GetChunkByID(id string) (*memory.ChunkSearchResult, error) {
	r, err := s.db.GetChunkByID(id)
	if err != nil {
		return nil, err
	}
	return &memory.ChunkSearchResult{
		ID:        r.ID,
		Path:      r.Path,
		StartLine: r.StartLine,
		EndLine:   r.EndLine,
		Text:      r.Text,
		Score:     r.Score,
		Source:    r.Source,
	}, nil
}

// UpdateChunkEmbedding stores the embedding vector on a chunk.
func (s *Store) UpdateChunkEmbedding(id string, embedding []float32) {
	s.db.UpdateChunkEmbedding(id, embedding)
}

// GetAllEmbeddings returns all chunks that have embeddings.
func (s *Store) GetAllEmbeddings() ([]memory.ChunkEmbedding, error) {
	results, err := s.db.GetAllEmbeddings()
	if err != nil {
		return nil, err
	}
	out := make([]memory.ChunkEmbedding, len(results))
	for i, r := range results {
		out[i] = memory.ChunkEmbedding{ID: r.ID, Embedding: r.Embedding}
	}
	return out, nil
}

// GetEmbeddingCache retrieves a cached embedding by content hash.
func (s *Store) GetEmbeddingCache(contentHash string) ([]float32, error) {
	return s.db.GetEmbeddingCache(contentHash)
}

// SetEmbeddingCache stores an embedding in the cache.
func (s *Store) SetEmbeddingCache(contentHash string, embedding []float32, model string) {
	s.db.SetEmbeddingCache(contentHash, embedding, model)
}

// SearchChunksFTS performs full-text search on chunks.
func (s *Store) SearchChunksFTS(query string, limit int) ([]memory.ChunkSearchResult, error) {
	results, err := s.db.SearchChunksFTS(query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]memory.ChunkSearchResult, len(results))
	for i, r := range results {
		out[i] = memory.ChunkSearchResult{
			ID:        r.ID,
			Path:      r.Path,
			StartLine: r.StartLine,
			EndLine:   r.EndLine,
			Text:      r.Text,
			Score:     r.Score,
			Source:    r.Source,
		}
	}
	return out, nil
}

// ListSessionIDs returns all non-OTR session IDs.
func (s *Store) ListSessionIDs() ([]string, error) {
	sessions, err := s.db.ListSessions()
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(sessions))
	for i, sess := range sessions {
		ids[i] = sess.ID
	}
	return ids, nil
}

// GetSessionMessages returns messages for a session as role/content pairs.
func (s *Store) GetSessionMessages(sessionID string) ([]memory.SessionMessage, error) {
	messages, err := s.db.GetMessages(sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]memory.SessionMessage, len(messages))
	for i, m := range messages {
		out[i] = memory.SessionMessage{Role: m.Role, Content: m.Content}
	}
	return out, nil
}

// RawConn returns the underlying *sql.DB for advanced queries such as
// sqlite-vec extension operations.
func (s *Store) RawConn() *sql.DB {
	return s.db.Conn()
}

// --- memory.Store interface (FTS5 indexing for the Manager) ---

// IndexMemoryFile indexes a memory file for FTS5 search.
func (s *Store) IndexMemoryFile(path string, content string) {
	s.db.IndexMemoryFile(path, content)
}

// ClearMemoryIndex removes all FTS5 entries.
func (s *Store) ClearMemoryIndex() {
	s.db.ClearMemoryIndex()
}

// SearchMemory performs FTS5 search across indexed memory content,
// converting storage results to memory-local types.
func (s *Store) SearchMemory(query string, limit int) ([]memory.SearchResult, error) {
	results, err := s.db.SearchMemory(query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]memory.SearchResult, len(results))
	for i, r := range results {
		out[i] = memory.SearchResult{
			Path:    r.Path,
			Section: r.Section,
			Snippet: r.Snippet,
			Score:   r.Score,
		}
	}
	return out, nil
}
