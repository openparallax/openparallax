package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/internal/logging"
)

// Indexer manages the chunk index and embedding pipeline.
type Indexer struct {
	store    ChunkStore
	embedder EmbeddingProvider
	vector   VectorSearcher
	log      *logging.Logger
}

// NewIndexer creates a new Indexer backed by the given ChunkStore.
func NewIndexer(store ChunkStore, embedder EmbeddingProvider, vector VectorSearcher, log *logging.Logger) *Indexer {
	return &Indexer{store: store, embedder: embedder, vector: vector, log: log}
}

// IndexFile chunks a file and indexes it. Skips unchanged files.
func (idx *Indexer) IndexFile(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	contentHash := hashContent(data)

	// Check if file has changed since last index.
	existing, err := idx.store.GetFileHash(path)
	if err == nil && existing == contentHash {
		return nil
	}

	// Remove old chunks for this file.
	idx.store.DeleteChunksByPath(path)
	if idx.vector != nil {
		ids, _ := idx.store.GetChunkIDsByPath(path)
		for _, id := range ids {
			_ = idx.vector.Delete(id)
		}
	}

	// Chunk the content.
	chunks := ChunkMarkdown(string(data), 400, 80)

	// Batch texts for embedding.
	var texts []string
	var chunkIDs []string
	var chunkHashes []string

	for _, chunk := range chunks {
		id := crypto.NewID()
		h := hashContent([]byte(chunk.Text))
		chunkIDs = append(chunkIDs, id)
		chunkHashes = append(chunkHashes, h)
		texts = append(texts, chunk.Text)

		idx.store.InsertChunk(id, path, chunk.StartLine, chunk.EndLine, chunk.Text, h)
	}

	// Embed if provider available.
	if idx.embedder != nil && len(texts) > 0 {
		idx.embedBatch(ctx, chunkIDs, chunkHashes, texts)
	}

	// Update file hash record.
	idx.store.SetFileHash(path, contentHash)

	if idx.log != nil {
		idx.log.Info("indexed_file", "path", path, "chunks", len(chunks))
	}
	return nil
}

// IndexWorkspace indexes all memory-related files in the workspace.
func (idx *Indexer) IndexWorkspace(ctx context.Context, workspacePath string) {
	memFiles := []string{"MEMORY.md", "USER.md", "AGENTS.md", "HEARTBEAT.md"}
	for _, f := range memFiles {
		path := filepath.Join(workspacePath, f)
		if _, err := os.Stat(path); err == nil {
			if err := idx.IndexFile(ctx, path); err != nil && idx.log != nil {
				idx.log.Warn("index_file_failed", "path", path, "error", err)
			}
		}
	}
}

// IndexSessions indexes past session transcripts.
func (idx *Indexer) IndexSessions(ctx context.Context) {
	sessionIDs, err := idx.store.ListSessionIDs()
	if err != nil {
		return
	}

	for _, sid := range sessionIDs {
		path := fmt.Sprintf("session:%s", sid)

		// Check if already indexed.
		if existing, err := idx.store.GetFileHash(path); err == nil && existing != "" {
			continue
		}

		messages, err := idx.store.GetSessionMessages(sid)
		if err != nil || len(messages) < 2 {
			continue
		}

		var transcript strings.Builder
		for _, msg := range messages {
			fmt.Fprintf(&transcript, "%s: %s\n", msg.Role, msg.Content)
		}

		text := transcript.String()
		contentHash := hashContent([]byte(text))
		chunks := ChunkMarkdown(text, 400, 80)

		var texts []string
		var chunkIDs []string
		var chunkHashes []string

		for _, chunk := range chunks {
			id := crypto.NewID()
			h := hashContent([]byte(chunk.Text))
			chunkIDs = append(chunkIDs, id)
			chunkHashes = append(chunkHashes, h)
			texts = append(texts, chunk.Text)
			idx.store.InsertChunk(id, path, chunk.StartLine, chunk.EndLine, chunk.Text, h)
		}

		if idx.embedder != nil && len(texts) > 0 {
			idx.embedBatch(ctx, chunkIDs, chunkHashes, texts)
		}

		idx.store.SetFileHash(path, contentHash)
	}
}

func (idx *Indexer) embedBatch(ctx context.Context, ids, hashes, texts []string) {
	// Check embedding cache for each chunk.
	var toEmbed []int
	cachedEmbeddings := make(map[int][]float32)

	for i, h := range hashes {
		if cached, err := idx.store.GetEmbeddingCache(h); err == nil {
			cachedEmbeddings[i] = cached
		} else {
			toEmbed = append(toEmbed, i)
		}
	}

	// Insert cached embeddings.
	for i, emb := range cachedEmbeddings {
		idx.store.UpdateChunkEmbedding(ids[i], emb)
		if idx.vector != nil {
			_ = idx.vector.Insert(ids[i], emb)
		}
	}

	// Embed remaining in batches of 20.
	batchSize := 20
	for start := 0; start < len(toEmbed); start += batchSize {
		end := start + batchSize
		if end > len(toEmbed) {
			end = len(toEmbed)
		}
		batch := toEmbed[start:end]

		batchTexts := make([]string, len(batch))
		for j, idx := range batch {
			batchTexts[j] = texts[idx]
		}

		embeddings, err := idx.embedder.Embed(ctx, batchTexts)
		if err != nil {
			if idx.log != nil {
				idx.log.Warn("embedding_failed", "error", err)
			}
			continue
		}

		for j, emb := range embeddings {
			origIdx := batch[j]
			idx.store.UpdateChunkEmbedding(ids[origIdx], emb)
			idx.store.SetEmbeddingCache(hashes[origIdx], emb, idx.embedder.ModelID())
			if idx.vector != nil {
				_ = idx.vector.Insert(ids[origIdx], emb)
			}
		}
	}
}

func hashContent(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
