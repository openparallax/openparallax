package memory

import (
	"database/sql"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"runtime"

	"github.com/openparallax/openparallax/internal/logging"
)

// NativeVectorSearcher uses the sqlite-vec extension for in-database vector
// queries. Only available when the extension is downloaded via
// `openparallax get-vector-ext`. Falls back to BuiltinVectorSearcher otherwise.
type NativeVectorSearcher struct {
	conn *sql.DB
}

// NewVectorSearcher tries to detect the sqlite-vec extension. If found and
// loadable, returns a NativeVectorSearcher with the vec0 table rebuilt from
// BLOB embeddings. Otherwise returns a BuiltinVectorSearcher.
func NewVectorSearcher(store ChunkStore, log *logging.Logger) VectorSearcher {
	extPath := sqliteVecExtensionPath()
	if extPath == "" {
		if log != nil {
			log.Info("vector_search_mode", "mode", "builtin")
		}
		return loadBuiltinFromStore(store, log)
	}

	if log != nil {
		log.Info("vector_ext_detected", "path", extPath)
	}

	// Native vector search requires raw SQL access for the vec0 virtual table.
	provider, ok := store.(RawConnProvider)
	if !ok {
		if log != nil {
			log.Warn("vector_native_unavailable", "reason", "store does not provide raw connection")
			log.Info("vector_search_mode", "mode", "builtin")
		}
		return loadBuiltinFromStore(store, log)
	}

	conn, ok := provider.RawConn().(*sql.DB)
	if !ok {
		if log != nil {
			log.Warn("vector_native_unavailable", "reason", "raw connection is not *sql.DB")
			log.Info("vector_search_mode", "mode", "builtin")
		}
		return loadBuiltinFromStore(store, log)
	}

	native := &NativeVectorSearcher{conn: conn}
	if err := native.RebuildFromBlobs(); err != nil {
		if log != nil {
			log.Warn("vector_rebuild_failed", "error", err)
			log.Info("vector_search_mode", "mode", "builtin")
		}
		return loadBuiltinFromStore(store, log)
	}

	if log != nil {
		log.Info("vector_search_mode", "mode", "native")
	}
	return native
}

// RebuildFromBlobs drops and recreates the vec0 table from BLOB embeddings
// in the chunks table. Called on every startup when sqlite-vec is loaded.
func (n *NativeVectorSearcher) RebuildFromBlobs() error {
	_, _ = n.conn.Exec("DROP TABLE IF EXISTS chunks_vec")
	_, err := n.conn.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS chunks_vec USING vec0(
		id TEXT PRIMARY KEY,
		embedding float[1536]
	)`)
	if err != nil {
		return err
	}

	rows, err := n.conn.Query(`SELECT id, embedding FROM chunks WHERE embedding IS NOT NULL`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id string
		var blob []byte
		if scanErr := rows.Scan(&id, &blob); scanErr == nil && len(blob) > 0 {
			_, _ = n.conn.Exec(`INSERT INTO chunks_vec (id, embedding) VALUES (?, ?)`, id, blob)
		}
	}
	return nil
}

// Insert adds or replaces a vector in the vec0 table.
func (n *NativeVectorSearcher) Insert(id string, embedding []float32) error {
	blob := vecSerialize(embedding)
	_, err := n.conn.Exec(`INSERT OR REPLACE INTO chunks_vec (id, embedding) VALUES (?, ?)`, id, blob)
	return err
}

// Search performs nearest-neighbor vector search using the vec0 table.
func (n *NativeVectorSearcher) Search(query []float32, limit int) ([]VectorResult, error) {
	blob := vecSerialize(query)
	rows, err := n.conn.Query(`
		SELECT id, distance
		FROM chunks_vec
		WHERE embedding MATCH ?
		ORDER BY distance ASC
		LIMIT ?`, blob, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []VectorResult
	for rows.Next() {
		var id string
		var dist float64
		if scanErr := rows.Scan(&id, &dist); scanErr == nil {
			results = append(results, VectorResult{ID: id, Score: 1 - dist})
		}
	}
	return results, nil
}

// Delete removes a vector from the vec0 table.
func (n *NativeVectorSearcher) Delete(id string) error {
	_, err := n.conn.Exec(`DELETE FROM chunks_vec WHERE id = ?`, id)
	return err
}

// vecSerialize converts a float32 slice to little-endian bytes for sqlite-vec.
func vecSerialize(emb []float32) []byte {
	buf := make([]byte, len(emb)*4)
	for i, v := range emb {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// sqliteVecExtensionPath returns the path to the sqlite-vec shared library.
func sqliteVecExtensionPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	var ext string
	switch runtime.GOOS {
	case "linux":
		ext = "so"
	case "darwin":
		ext = "dylib"
	case "windows":
		ext = "dll"
	default:
		return ""
	}

	path := filepath.Join(homeDir, ".openparallax", "extensions", "sqlite-vec."+ext)
	if _, statErr := os.Stat(path); statErr == nil {
		return path
	}
	return ""
}

// loadBuiltinFromStore creates a BuiltinVectorSearcher preloaded from the store.
func loadBuiltinFromStore(store ChunkStore, log *logging.Logger) *BuiltinVectorSearcher {
	vs := NewBuiltinVectorSearcher()
	embeddings, err := store.GetAllEmbeddings()
	if err != nil {
		return vs
	}
	for _, e := range embeddings {
		_ = vs.Insert(e.ID, e.Embedding)
	}
	if log != nil && len(embeddings) > 0 {
		log.Info("builtin_vectors_loaded", "count", len(embeddings))
	}
	return vs
}
