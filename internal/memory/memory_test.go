package memory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mgr := NewManager(dir, db, nil)
	return mgr, dir
}

func TestReadExistingFile(t *testing.T) {
	mgr, dir := openTestManager(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("Be helpful."), 0o644))

	content, err := mgr.Read(types.MemorySoul)
	require.NoError(t, err)
	assert.Equal(t, "Be helpful.", content)
}

func TestReadNonexistentFile(t *testing.T) {
	mgr, _ := openTestManager(t)

	_, err := mgr.Read(types.MemorySoul)
	assert.ErrorIs(t, err, types.ErrMemoryFileNotFound)
}

func TestAppendAndReindex(t *testing.T) {
	mgr, dir := openTestManager(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte("# Memory\n"), 0o644))

	require.NoError(t, mgr.Append(types.MemoryMain, "\n## Today\nLearned about quantum computing.\n"))

	content, err := mgr.Read(types.MemoryMain)
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

func TestDailyLog(t *testing.T) {
	mgr, dir := openTestManager(t)

	actions := []*types.ActionRequest{
		{Type: types.ActionReadFile, Payload: map[string]any{"path": "test.txt"}},
	}
	results := []*types.ActionResult{
		{Success: true, Summary: "read test.txt (100 bytes)"},
	}

	mgr.LogAction(actions, results)

	logDir := filepath.Join(dir, "memory")
	entries, err := os.ReadDir(logDir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries, "daily log directory should have a file")

	data, err := os.ReadFile(filepath.Join(logDir, entries[0].Name()))
	require.NoError(t, err)
	assert.Contains(t, string(data), "read_file")
	assert.Contains(t, string(data), "OK")
}

func TestDailyLogCreatesDirectory(t *testing.T) {
	mgr, dir := openTestManager(t)

	logDir := filepath.Join(dir, "memory")
	_, err := os.Stat(logDir)
	assert.True(t, os.IsNotExist(err), "memory dir should not exist yet")

	mgr.LogAction(
		[]*types.ActionRequest{{Type: types.ActionWriteFile}},
		[]*types.ActionResult{{Success: true, Summary: "wrote file"}},
	)

	_, err = os.Stat(logDir)
	assert.NoError(t, err, "memory dir should exist after LogAction")
}
