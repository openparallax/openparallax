package storage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenCreatesDatabase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := Open(path)
	require.NoError(t, err)
	defer db.Close()

	assert.FileExists(t, path)
}

func TestMigrationsRunWithoutError(t *testing.T) {
	db := openTestDB(t)

	// Verify tables exist by running a query against each.
	tables := []string{"sessions", "messages", "snapshots", "transactions", "audit_index"}
	for _, table := range tables {
		_, err := db.conn.Exec("SELECT 1 FROM " + table + " LIMIT 1")
		assert.NoError(t, err, "table %s should exist", table)
	}

	// Verify FTS5 virtual table.
	_, err := db.conn.Exec("SELECT 1 FROM memory_fts LIMIT 1")
	assert.NoError(t, err, "memory_fts virtual table should exist")
}

func TestSessionInsertAndGet(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	sess := &types.Session{
		ID:        "sess-001",
		Mode:      types.SessionNormal,
		Title:     "Test Session",
		CreatedAt: now,
	}
	require.NoError(t, db.InsertSession(sess))

	got, err := db.GetSession("sess-001")
	require.NoError(t, err)
	assert.Equal(t, "sess-001", got.ID)
	assert.Equal(t, types.SessionNormal, got.Mode)
	assert.Equal(t, "Test Session", got.Title)
}

func TestGetSessionNotFound(t *testing.T) {
	db := openTestDB(t)
	_, err := db.GetSession("nonexistent")
	assert.ErrorIs(t, err, types.ErrSessionNotFound)
}

func TestListSessionsOrderedByRecency(t *testing.T) {
	db := openTestDB(t)

	t1 := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	t2 := time.Now().Add(-1 * time.Hour).Truncate(time.Second)

	require.NoError(t, db.InsertSession(&types.Session{ID: "old", Mode: types.SessionNormal, CreatedAt: t1}))
	require.NoError(t, db.InsertSession(&types.Session{ID: "new", Mode: types.SessionNormal, CreatedAt: t2}))

	sessions, err := db.ListSessions()
	require.NoError(t, err)
	require.Len(t, sessions, 2)
	assert.Equal(t, "new", sessions[0].ID, "most recent session should be first")
	assert.Equal(t, "old", sessions[1].ID)
}

func TestListSessionsExcludesOTR(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.InsertSession(&types.Session{ID: "normal", Mode: types.SessionNormal, CreatedAt: time.Now()}))
	require.NoError(t, db.InsertSession(&types.Session{ID: "otr", Mode: types.SessionOTR, CreatedAt: time.Now()}))

	sessions, err := db.ListSessions()
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "normal", sessions[0].ID)
}

func TestMessageInsertAndGet(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	require.NoError(t, db.InsertSession(&types.Session{ID: "sess-001", Mode: types.SessionNormal, CreatedAt: now}))

	msg := &types.Message{
		ID:        "msg-001",
		SessionID: "sess-001",
		Role:      "user",
		Content:   "Hello, world!",
		Timestamp: now,
	}
	require.NoError(t, db.InsertMessage(msg))

	msgs, err := db.GetMessages("sess-001")
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "msg-001", msgs[0].ID)
	assert.Equal(t, "Hello, world!", msgs[0].Content)
	assert.Equal(t, "user", msgs[0].Role)
}

func TestMessagesOrderedByTimestamp(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)
	require.NoError(t, db.InsertSession(&types.Session{ID: "s", Mode: types.SessionNormal, CreatedAt: now}))

	require.NoError(t, db.InsertMessage(&types.Message{ID: "m1", SessionID: "s", Role: "user", Content: "first", Timestamp: now}))
	require.NoError(t, db.InsertMessage(&types.Message{ID: "m2", SessionID: "s", Role: "assistant", Content: "second", Timestamp: now.Add(time.Second)}))

	msgs, err := db.GetMessages("s")
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, "first", msgs[0].Content)
	assert.Equal(t, "second", msgs[1].Content)
}

func TestDeleteSessionCascadesToMessages(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)
	require.NoError(t, db.InsertSession(&types.Session{ID: "s", Mode: types.SessionNormal, CreatedAt: now}))
	require.NoError(t, db.InsertMessage(&types.Message{ID: "m1", SessionID: "s", Role: "user", Content: "test", Timestamp: now}))

	require.NoError(t, db.DeleteSession("s"))

	msgs, err := db.GetMessages("s")
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestUpdateSessionTitle(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.InsertSession(&types.Session{ID: "s", Mode: types.SessionNormal, CreatedAt: time.Now()}))

	require.NoError(t, db.UpdateSessionTitle("s", "New Title"))

	got, err := db.GetSession("s")
	require.NoError(t, err)
	assert.Equal(t, "New Title", got.Title)
}

func TestFTSIndexAndSearch(t *testing.T) {
	db := openTestDB(t)

	content := `## Section One

This is about quantum computing and its applications.

## Section Two

This discusses machine learning algorithms.`

	db.IndexMemoryFile("MEMORY.md", content)

	results, err := db.SearchMemory("quantum", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.Equal(t, "MEMORY.md", results[0].Path)
}

func TestFTSSearchNoMatch(t *testing.T) {
	db := openTestDB(t)

	db.IndexMemoryFile("MEMORY.md", "Nothing relevant here.")

	results, err := db.SearchMemory("xyznonexistent", 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestFTSClearAndReindex(t *testing.T) {
	db := openTestDB(t)

	db.IndexMemoryFile("MEMORY.md", "old content about cats")

	results, err := db.SearchMemory("cats", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results)

	db.ClearMemoryIndex()

	results, err = db.SearchMemory("cats", 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSnapshotInsertAndGet(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	snap := &types.SnapshotMetadata{
		ID:            "snap-001",
		Timestamp:     now,
		ActionType:    "write_file",
		ActionSummary: "Writing test.txt",
		FilesBackedUp: []string{"/tmp/test.txt"},
		Hash:          "abc123",
		PreviousHash:  "",
	}
	require.NoError(t, db.InsertSnapshot(snap))

	got, err := db.GetSnapshot("snap-001")
	require.NoError(t, err)
	assert.Equal(t, "snap-001", got.ID)
	assert.Equal(t, "write_file", got.ActionType)
	assert.Equal(t, []string{"/tmp/test.txt"}, got.FilesBackedUp)
}

func TestGetLastSnapshotHashEmpty(t *testing.T) {
	db := openTestDB(t)
	hash := db.GetLastSnapshotHash()
	assert.Empty(t, hash, "fresh database should have empty last snapshot hash")
}

func TestGetLastSnapshotHash(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	require.NoError(t, db.InsertSnapshot(&types.SnapshotMetadata{
		ID: "s1", Timestamp: now, ActionType: "read_file", Hash: "hash1",
	}))
	require.NoError(t, db.InsertSnapshot(&types.SnapshotMetadata{
		ID: "s2", Timestamp: now.Add(time.Second), ActionType: "write_file", Hash: "hash2", PreviousHash: "hash1",
	}))

	assert.Equal(t, "hash2", db.GetLastSnapshotHash())
}

func TestPruneSnapshotsByCount(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	for i := 0; i < 5; i++ {
		require.NoError(t, db.InsertSnapshot(&types.SnapshotMetadata{
			ID: "s" + string(rune('a'+i)), Timestamp: now.Add(time.Duration(i) * time.Second),
			ActionType: "write_file", Hash: "h" + string(rune('a'+i)),
		}))
	}

	count, _ := db.SnapshotCount()
	assert.Equal(t, 5, count)

	db.PruneSnapshots(3, 365)

	count, _ = db.SnapshotCount()
	assert.Equal(t, 3, count, "should keep only 3 most recent snapshots")
}

func TestSessionCount(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.InsertSession(&types.Session{ID: "a", Mode: types.SessionNormal, CreatedAt: time.Now()}))
	require.NoError(t, db.InsertSession(&types.Session{ID: "b", Mode: types.SessionNormal, CreatedAt: time.Now()}))

	count, err := db.SessionCount()
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestAuditInsertAndCount(t *testing.T) {
	db := openTestDB(t)

	entry := &types.AuditEntry{
		ID:        "audit-001",
		EventType: types.AuditActionExecuted,
		Timestamp: time.Now().UnixMilli(),
		SessionID: "sess-001",
	}
	require.NoError(t, db.InsertAuditEntry(entry))

	count, err := db.AuditEntryCount()
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestSnapshotNotFound(t *testing.T) {
	db := openTestDB(t)
	_, err := db.GetSnapshot("nonexistent")
	assert.ErrorIs(t, err, types.ErrSnapshotNotFound)
}
