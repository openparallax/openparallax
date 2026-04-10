package storage

import (
	"encoding/binary"
	"math"
)

// InsertChunk adds a text chunk to the chunks table and FTS5 index.
func (db *DB) InsertChunk(id, path string, startLine, endLine int, text, hash string) {
	_, _ = db.conn.Exec(
		`INSERT OR REPLACE INTO chunks (id, path, start_line, end_line, text, hash) VALUES (?, ?, ?, ?, ?, ?)`,
		id, path, startLine, endLine, text, hash,
	)
	// Sync to FTS5 content table.
	_, _ = db.conn.Exec(`INSERT INTO chunks_fts (rowid, text) VALUES ((SELECT rowid FROM chunks WHERE id = ?), ?)`, id, text)
}

// DeleteChunksByPath removes all chunks for a given file path.
func (db *DB) DeleteChunksByPath(path string) {
	// Remove from FTS5 first.
	_, _ = db.conn.Exec(`DELETE FROM chunks_fts WHERE rowid IN (SELECT rowid FROM chunks WHERE path = ?)`, path)
	_, _ = db.conn.Exec(`DELETE FROM chunks WHERE path = ?`, path)
}

// GetChunkIDsByPath returns all chunk IDs for a given path.
func (db *DB) GetChunkIDsByPath(path string) ([]string, error) {
	rows, err := db.conn.Query(`SELECT id FROM chunks WHERE path = ?`, path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// UpdateChunkEmbedding stores the embedding vector as a BLOB on a chunk.
func (db *DB) UpdateChunkEmbedding(id string, embedding []float32) {
	blob := serializeEmbedding(embedding)
	_, _ = db.conn.Exec(`UPDATE chunks SET embedding = ? WHERE id = ?`, blob, id)
}

// GetAllEmbeddings returns all chunks that have embeddings.
func (db *DB) GetAllEmbeddings() ([]ChunkEmbedding, error) {
	rows, err := db.conn.Query(`SELECT id, embedding FROM chunks WHERE embedding IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []ChunkEmbedding
	for rows.Next() {
		var id string
		var blob []byte
		if err := rows.Scan(&id, &blob); err == nil {
			emb := deserializeEmbedding(blob)
			if len(emb) > 0 {
				results = append(results, ChunkEmbedding{ID: id, Embedding: emb})
			}
		}
	}
	return results, nil
}

// ChunkEmbedding holds a chunk ID and its embedding vector.
type ChunkEmbedding struct {
	ID        string
	Embedding []float32
}

// SearchChunksFTS performs full-text search on chunks and returns results.
func (db *DB) SearchChunksFTS(query string, limit int) ([]ChunkSearchResult, error) {
	rows, err := db.conn.Query(`
		SELECT c.id, c.path, c.start_line, c.end_line, c.text, rank
		FROM chunks_fts f
		JOIN chunks c ON c.rowid = f.rowid
		WHERE chunks_fts MATCH ?
		ORDER BY rank
		LIMIT ?`, query, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []ChunkSearchResult
	for rows.Next() {
		var r ChunkSearchResult
		var rank float64
		if err := rows.Scan(&r.ID, &r.Path, &r.StartLine, &r.EndLine, &r.Text, &rank); err == nil {
			r.Score = -rank // FTS5 rank is negative; negate for positive score
			r.Source = "keyword"
			results = append(results, r)
		}
	}
	return results, nil
}

// ChunkSearchResult is a search result from the chunks index.
type ChunkSearchResult struct {
	ID        string
	Path      string
	StartLine int
	EndLine   int
	Text      string
	Score     float64
	Source    string
}

// GetChunkByID retrieves a chunk by its ID.
func (db *DB) GetChunkByID(id string) (*ChunkSearchResult, error) {
	var r ChunkSearchResult
	err := db.conn.QueryRow(`SELECT id, path, start_line, end_line, text FROM chunks WHERE id = ?`, id).
		Scan(&r.ID, &r.Path, &r.StartLine, &r.EndLine, &r.Text)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// --- File hash tracking ---

// GetFileHash returns the stored hash for a file path.
func (db *DB) GetFileHash(path string) (string, error) {
	var hash string
	err := db.conn.QueryRow(`SELECT hash FROM file_hashes WHERE path = ?`, path).Scan(&hash)
	return hash, err
}

// SetFileHash stores or updates the hash for a file path.
func (db *DB) SetFileHash(path, hash string) {
	_, _ = db.conn.Exec(
		`INSERT OR REPLACE INTO file_hashes (path, hash, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)`,
		path, hash,
	)
}

// --- Embedding cache ---

// GetEmbeddingCache retrieves a cached embedding by content hash.
func (db *DB) GetEmbeddingCache(contentHash string) ([]float32, error) {
	var blob []byte
	err := db.conn.QueryRow(`SELECT embedding FROM embedding_cache WHERE content_hash = ?`, contentHash).Scan(&blob)
	if err != nil {
		return nil, err
	}
	return deserializeEmbedding(blob), nil
}

// SetEmbeddingCache stores an embedding in the cache.
func (db *DB) SetEmbeddingCache(contentHash string, embedding []float32, model string) {
	blob := serializeEmbedding(embedding)
	_, _ = db.conn.Exec(
		`INSERT OR REPLACE INTO embedding_cache (content_hash, embedding, model) VALUES (?, ?, ?)`,
		contentHash, blob, model,
	)
}

// --- Serialization ---

func serializeEmbedding(emb []float32) []byte {
	buf := make([]byte, len(emb)*4)
	for i, v := range emb {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

func deserializeEmbedding(blob []byte) []float32 {
	if len(blob)%4 != 0 {
		return nil
	}
	emb := make([]float32, len(blob)/4)
	for i := range emb {
		emb[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
	}
	return emb
}
