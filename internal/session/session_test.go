package session

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewStore(db)
}

func TestCreateNormalSession(t *testing.T) {
	s := openTestStore(t)
	sess := s.Create(types.SessionNormal)
	assert.NotEmpty(t, sess.ID)
	assert.Equal(t, types.SessionNormal, sess.Mode)

	got, err := s.Get(sess.ID)
	require.NoError(t, err)
	assert.Equal(t, sess.ID, got.ID)
}

func TestCreateOTRSession(t *testing.T) {
	s := openTestStore(t)
	sess := s.Create(types.SessionOTR)
	assert.Equal(t, types.SessionOTR, sess.Mode)

	got, err := s.Get(sess.ID)
	require.NoError(t, err)
	assert.Equal(t, sess.ID, got.ID)
}

func TestListExcludesOTR(t *testing.T) {
	s := openTestStore(t)
	s.Create(types.SessionNormal)
	s.Create(types.SessionOTR)

	sessions, err := s.List()
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
}

func TestDeleteSession(t *testing.T) {
	s := openTestStore(t)
	sess := s.Create(types.SessionNormal)

	require.NoError(t, s.Delete(sess.ID))
	_, err := s.Get(sess.ID)
	assert.Error(t, err)
}

func TestRenameSession(t *testing.T) {
	s := openTestStore(t)
	sess := s.Create(types.SessionNormal)

	require.NoError(t, s.Rename(sess.ID, "Test Title"))
	got, err := s.Get(sess.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test Title", got.Title)
}

func TestGetHistory(t *testing.T) {
	s := openTestStore(t)
	sess := s.Create(types.SessionNormal)

	_ = s.db.InsertMessage(&types.Message{
		ID: "m1", SessionID: sess.ID, Role: "user", Content: "hello", Timestamp: time.Now(),
	})
	_ = s.db.InsertMessage(&types.Message{
		ID: "m2", SessionID: sess.ID, Role: "assistant", Content: "hi", Timestamp: time.Now().Add(time.Second),
	})

	history := s.GetHistory(sess.ID)
	assert.Len(t, history, 2)
	assert.Equal(t, "user", history[0].Role)
	assert.Equal(t, "assistant", history[1].Role)
}

func TestAutoTitle(t *testing.T) {
	assert.Equal(t, "Hello world", AutoTitle("Hello world"))
	assert.Equal(t, "New session", AutoTitle(""))

	long := "This is a very long message that should be truncated at a word boundary to fit within fifty chars"
	title := AutoTitle(long)
	assert.LessOrEqual(t, len(title), 54) // 50 + "..."
	assert.True(t, len(title) > 10)
}

func TestDestroyOTR(t *testing.T) {
	s := openTestStore(t)
	normal := s.Create(types.SessionNormal)
	otr := s.Create(types.SessionOTR)

	s.DestroyOTR()

	_, err := s.Get(otr.ID)
	assert.Error(t, err)

	_, err = s.Get(normal.ID)
	assert.NoError(t, err)
}

func TestOTRAllowsReadFile(t *testing.T) {
	assert.True(t, IsOTRAllowed(types.ActionReadFile))
}

func TestOTRAllowsListDir(t *testing.T) {
	assert.True(t, IsOTRAllowed(types.ActionListDir))
}

func TestOTRAllowsMemorySearch(t *testing.T) {
	assert.True(t, IsOTRAllowed(types.ActionMemorySearch))
}

func TestOTRBlocksWriteFile(t *testing.T) {
	assert.False(t, IsOTRAllowed(types.ActionWriteFile))
	assert.NotEmpty(t, OTRBlockReason(types.ActionWriteFile))
}

func TestOTRBlocksExecCommand(t *testing.T) {
	assert.False(t, IsOTRAllowed(types.ActionExecCommand))
}

func TestOTRBlocksSendMessage(t *testing.T) {
	assert.False(t, IsOTRAllowed(types.ActionSendMessage))
}

func TestOTRBlocksMemoryWrite(t *testing.T) {
	assert.False(t, IsOTRAllowed(types.ActionMemoryWrite))
}

func TestOTRBlockReasonSpecific(t *testing.T) {
	reason := OTRBlockReason(types.ActionWriteFile)
	assert.Contains(t, reason, "Off the Record")
}
