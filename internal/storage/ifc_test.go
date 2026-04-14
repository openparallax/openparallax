package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openIFCTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestRecordIFCWrite_InsertsEntry(t *testing.T) {
	db := openIFCTestDB(t)
	db.RecordIFCWrite("/home/user/notes.txt", 4, "/home/user/.env", "session-1")

	sens := db.LookupIFCClassification("/home/user/notes.txt")
	assert.Equal(t, 4, sens)
}

func TestRecordIFCWrite_UpgradesNeverDowngrades(t *testing.T) {
	db := openIFCTestDB(t)

	// Write Confidential first.
	db.RecordIFCWrite("/home/user/notes.txt", 2, "/home/user/config.yaml", "s1")
	assert.Equal(t, 2, db.LookupIFCClassification("/home/user/notes.txt"))

	// Upgrade to Critical.
	db.RecordIFCWrite("/home/user/notes.txt", 4, "/home/user/.env", "s2")
	assert.Equal(t, 4, db.LookupIFCClassification("/home/user/notes.txt"))

	// Attempt to downgrade to Confidential — should stay Critical.
	db.RecordIFCWrite("/home/user/notes.txt", 2, "/home/user/config.yaml", "s3")
	assert.Equal(t, 4, db.LookupIFCClassification("/home/user/notes.txt"))
}

func TestLookupIFCClassification_NotFound(t *testing.T) {
	db := openIFCTestDB(t)
	sens := db.LookupIFCClassification("/nonexistent/path")
	assert.Equal(t, -1, sens)
}

func TestSweepIFCActivity_RemovesDeletedFiles(t *testing.T) {
	db := openIFCTestDB(t)
	dir := t.TempDir()

	// Create a file and record it.
	path := filepath.Join(dir, "secret.txt")
	require.NoError(t, os.WriteFile(path, []byte("data"), 0o644))
	db.RecordIFCWrite(path, 4, "/source", "s1")

	// Delete the file.
	require.NoError(t, os.Remove(path))

	// Sweep.
	removed, err := db.SweepIFCActivity(dir)
	require.NoError(t, err)
	assert.Len(t, removed, 1)
	assert.Equal(t, path, removed[0].Path)

	// Verify it's gone.
	assert.Equal(t, -1, db.LookupIFCClassification(path))
}

func TestSweepIFCActivity_KeepsExistingFiles(t *testing.T) {
	db := openIFCTestDB(t)
	dir := t.TempDir()

	path := filepath.Join(dir, "still-here.txt")
	require.NoError(t, os.WriteFile(path, []byte("data"), 0o644))
	db.RecordIFCWrite(path, 3, "/source", "s1")

	removed, err := db.SweepIFCActivity(dir)
	require.NoError(t, err)
	assert.Len(t, removed, 0)

	// Still tracked.
	assert.Equal(t, 3, db.LookupIFCClassification(path))
}

func TestListIFCActivity(t *testing.T) {
	db := openIFCTestDB(t)
	db.RecordIFCWrite("/a", 4, "/src-a", "s1")
	db.RecordIFCWrite("/b", 2, "/src-b", "s2")

	entries, err := db.ListIFCActivity()
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	// Ordered by sensitivity DESC.
	assert.Equal(t, 4, entries[0].Sensitivity)
	assert.Equal(t, 2, entries[1].Sensitivity)
}
