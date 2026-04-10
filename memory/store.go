package memory

// ChunkStore is the persistence interface for chunk-level indexing, embeddings,
// and session retrieval. Implementations live in subpackages (e.g. memory/sqlite)
// to avoid circular imports with internal/storage.
type ChunkStore interface {
	// GetFileHash returns the stored hash for a file path.
	GetFileHash(path string) (string, error)
	// SetFileHash stores or updates the hash for a file path.
	SetFileHash(path, hash string)
	// InsertChunk adds a text chunk to the chunks table and FTS5 index.
	InsertChunk(id, path string, startLine, endLine int, text, hash string)
	// DeleteChunksByPath removes all chunks for a given file path.
	DeleteChunksByPath(path string)
	// GetChunkIDsByPath returns all chunk IDs for a given path.
	GetChunkIDsByPath(path string) ([]string, error)
	// GetChunkByID retrieves a chunk by its ID.
	GetChunkByID(id string) (*ChunkSearchResult, error)
	// UpdateChunkEmbedding stores the embedding vector on a chunk.
	UpdateChunkEmbedding(id string, embedding []float32)
	// GetAllEmbeddings returns all chunks that have embeddings.
	GetAllEmbeddings() ([]ChunkEmbedding, error)
	// GetEmbeddingCache retrieves a cached embedding by content hash.
	GetEmbeddingCache(contentHash string) ([]float32, error)
	// SetEmbeddingCache stores an embedding in the cache.
	SetEmbeddingCache(contentHash string, embedding []float32, model string)
	// SearchChunksFTS performs full-text search on chunks.
	SearchChunksFTS(query string, limit int) ([]ChunkSearchResult, error)
	// ListSessionIDs returns all non-OTR session IDs.
	ListSessionIDs() ([]string, error)
	// GetSessionMessages returns messages for a session as role/content pairs.
	GetSessionMessages(sessionID string) ([]SessionMessage, error)
}

// RawConnProvider is an optional interface that ChunkStore implementations
// may satisfy to expose the underlying *sql.DB for advanced queries
// (e.g. sqlite-vec extension operations).
type RawConnProvider interface {
	// RawConn returns the underlying database connection.
	RawConn() interface{}
}
