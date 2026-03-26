package chronicle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestChronicle(t *testing.T) (*Chronicle, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	cfg := types.ChronicleConfig{MaxSnapshots: 100, MaxAgeDays: 30}
	chron, err := New(dir, cfg, db)
	require.NoError(t, err)
	return chron, dir
}

func TestSnapshotBeforeWrite(t *testing.T) {
	chron, dir := openTestChronicle(t)

	// Create a file to be backed up.
	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("original"), 0o644))

	snap, err := chron.Snapshot(&types.ActionRequest{
		Type:    types.ActionWriteFile,
		Payload: map[string]any{"path": filePath},
	})
	require.NoError(t, err)
	require.NotNil(t, snap)
	assert.Len(t, snap.FilesBackedUp, 1)

	// Verify backup exists.
	backupPath := filepath.Join(chron.snapshotDir, snap.ID, "test.txt")
	data, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	assert.Equal(t, "original", string(data))
}

func TestSnapshotMetadataInSQLite(t *testing.T) {
	chron, dir := openTestChronicle(t)

	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o644))

	snap, err := chron.Snapshot(&types.ActionRequest{
		Type:    types.ActionWriteFile,
		Payload: map[string]any{"path": filePath},
	})
	require.NoError(t, err)

	assert.NotEmpty(t, snap.Hash)
	assert.Equal(t, "write_file", snap.ActionType)
}

func TestRollbackRestoresFile(t *testing.T) {
	chron, dir := openTestChronicle(t)

	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("original"), 0o644))

	snap, err := chron.Snapshot(&types.ActionRequest{
		Type:    types.ActionWriteFile,
		Payload: map[string]any{"path": filePath},
	})
	require.NoError(t, err)

	// Modify the file.
	require.NoError(t, os.WriteFile(filePath, []byte("modified"), 0o644))

	// Rollback.
	require.NoError(t, chron.Rollback(snap.ID))

	data, _ := os.ReadFile(filePath)
	assert.Equal(t, "original", string(data))
}

func TestDiffDetectsModification(t *testing.T) {
	chron, dir := openTestChronicle(t)

	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("original"), 0o644))

	snap, err := chron.Snapshot(&types.ActionRequest{
		Type:    types.ActionWriteFile,
		Payload: map[string]any{"path": filePath},
	})
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filePath, []byte("modified"), 0o644))

	diff, err := chron.Diff(snap.ID)
	require.NoError(t, err)
	require.Len(t, diff.Changes, 1)
	assert.Equal(t, "modified", diff.Changes[0].ChangeType)
}

func TestDiffDetectsDeletion(t *testing.T) {
	chron, dir := openTestChronicle(t)

	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o644))

	snap, err := chron.Snapshot(&types.ActionRequest{
		Type:    types.ActionDeleteFile,
		Payload: map[string]any{"path": filePath},
	})
	require.NoError(t, err)

	require.NoError(t, os.Remove(filePath))

	diff, err := chron.Diff(snap.ID)
	require.NoError(t, err)
	require.Len(t, diff.Changes, 1)
	assert.Equal(t, "deleted", diff.Changes[0].ChangeType)
}

func TestIntegrityChain(t *testing.T) {
	chron, dir := openTestChronicle(t)

	for i := 0; i < 3; i++ {
		filePath := filepath.Join(dir, "test.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("v"+string(rune('0'+i))), 0o644))
		_, err := chron.Snapshot(&types.ActionRequest{
			Type:    types.ActionWriteFile,
			Payload: map[string]any{"path": filePath},
		})
		require.NoError(t, err)
	}

	assert.NoError(t, chron.VerifyIntegrity())
}

func TestRetentionPrunesOld(t *testing.T) {
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	cfg := types.ChronicleConfig{MaxSnapshots: 3, MaxAgeDays: 365}
	chron, err := New(dir, cfg, db)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		filePath := filepath.Join(dir, "test.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("v"+string(rune('0'+i))), 0o644))
		_, err := chron.Snapshot(&types.ActionRequest{
			Type:    types.ActionWriteFile,
			Payload: map[string]any{"path": filePath},
		})
		require.NoError(t, err)
	}

	snapshots := chron.List()
	assert.LessOrEqual(t, len(snapshots), 3)
}

func TestSnapshotSkipsReadAction(t *testing.T) {
	chron, _ := openTestChronicle(t)

	snap, err := chron.Snapshot(&types.ActionRequest{
		Type:    types.ActionReadFile,
		Payload: map[string]any{"path": "/tmp/test.txt"},
	})
	require.NoError(t, err)
	assert.Nil(t, snap, "read actions should not create snapshots")
}

func TestSnapshotSkipsNonexistentFile(t *testing.T) {
	chron, _ := openTestChronicle(t)

	snap, err := chron.Snapshot(&types.ActionRequest{
		Type:    types.ActionWriteFile,
		Payload: map[string]any{"path": "/nonexistent/file.txt"},
	})
	require.NoError(t, err)
	assert.Nil(t, snap, "should skip when file doesn't exist yet")
}
