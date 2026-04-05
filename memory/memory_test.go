package memory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testStore wraps storage.DB to satisfy the Store interface while converting
// storage.SearchResult to memory.SearchResult.
type testStore struct {
	db *storage.DB
}

func (s *testStore) IndexMemoryFile(path string, content string) {
	s.db.IndexMemoryFile(path, content)
}

func (s *testStore) ClearMemoryIndex() {
	s.db.ClearMemoryIndex()
}

func (s *testStore) SearchMemory(query string, limit int) ([]SearchResult, error) {
	results, err := s.db.SearchMemory(query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]SearchResult, len(results))
	for i, r := range results {
		out[i] = SearchResult{Path: r.Path, Section: r.Section, Snippet: r.Snippet, Score: r.Score}
	}
	return out, nil
}

func openTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mgr := NewManager(dir, &testStore{db: db}, nil)
	return mgr, dir
}

func TestReadExistingFile(t *testing.T) {
	mgr, dir := openTestManager(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("Be helpful."), 0o644))

	content, err := mgr.Read(MemorySoul)
	require.NoError(t, err)
	assert.Equal(t, "Be helpful.", content)
}

func TestReadNonexistentFile(t *testing.T) {
	mgr, _ := openTestManager(t)

	_, err := mgr.Read(MemorySoul)
	assert.ErrorIs(t, err, ErrFileNotFound)
}

func TestAppendAndReindex(t *testing.T) {
	mgr, dir := openTestManager(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte("# Memory\n"), 0o644))

	require.NoError(t, mgr.Append(MemoryMain, "\n## Today\nLearned about quantum computing.\n"))

	content, err := mgr.Read(MemoryMain)
	require.NoError(t, err)
	assert.Contains(t, content, "quantum computing")

	// Search should find the new content.
	results, err := mgr.Search("quantum", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestSearchNoResults(t *testing.T) {
	mgr, dir := openTestManager(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte("Nothing relevant here."), 0o644))
	mgr.ReindexAll()

	results, err := mgr.Search("xyznonexistent", 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestReindexAll(t *testing.T) {
	mgr, dir := openTestManager(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("Safety first."), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "IDENTITY.md"), []byte("Name: Atlas"), 0o644))

	mgr.ReindexAll()

	results, err := mgr.Search("Atlas", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}
