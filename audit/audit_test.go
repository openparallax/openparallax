package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewLogger(path)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	require.NoError(t, logger.Log(Entry{
		EventType: ActionExecuted,
		SessionID: "s1",
	}))

	assert.FileExists(t, path)

	data, _ := os.ReadFile(path)
	assert.Contains(t, string(data), "s1")
}

func TestLogHashChain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewLogger(path)
	require.NoError(t, err)

	require.NoError(t, logger.Log(Entry{EventType: ActionProposed, SessionID: "s1"}))
	require.NoError(t, logger.Log(Entry{EventType: ActionExecuted, SessionID: "s1"}))
	_ = logger.Close()

	// Verify the chain.
	assert.NoError(t, VerifyIntegrity(path))
}

func TestVerifyIntegrityOnValidLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewLogger(path)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		require.NoError(t, logger.Log(Entry{
			EventType:  ActionExecuted,
			ActionType: "read_file",
			SessionID:  "sess1",
		}))
	}
	_ = logger.Close()

	assert.NoError(t, VerifyIntegrity(path))
}

func TestVerifyIntegrityDetectsTampering(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewLogger(path)
	require.NoError(t, err)

	require.NoError(t, logger.Log(Entry{EventType: ActionProposed}))
	require.NoError(t, logger.Log(Entry{EventType: ActionExecuted}))
	_ = logger.Close()

	// Tamper with the file.
	data, _ := os.ReadFile(path)
	tampered := []byte(string(data)[:10] + "TAMPERED" + string(data)[18:])
	require.NoError(t, os.WriteFile(path, tampered, 0o644))

	err = VerifyIntegrity(path)
	assert.Error(t, err)
}

func TestVerifyIntegrityEmptyLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(""), 0o644))

	assert.NoError(t, VerifyIntegrity(path))
}

func TestViewerFiltersByEventType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewLogger(path)
	require.NoError(t, err)

	require.NoError(t, logger.Log(Entry{EventType: ActionProposed}))
	require.NoError(t, logger.Log(Entry{EventType: ActionExecuted}))
	require.NoError(t, logger.Log(Entry{EventType: ActionBlocked}))
	_ = logger.Close()

	entries, err := ReadEntries(path, Query{EventType: ActionExecuted, Limit: 10})
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, ActionExecuted, entries[0].EventType)
}

func TestViewerFiltersBySession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewLogger(path)
	require.NoError(t, err)

	require.NoError(t, logger.Log(Entry{EventType: ActionExecuted, SessionID: "s1"}))
	require.NoError(t, logger.Log(Entry{EventType: ActionExecuted, SessionID: "s2"}))
	require.NoError(t, logger.Log(Entry{EventType: ActionExecuted, SessionID: "s1"}))
	_ = logger.Close()

	entries, err := ReadEntries(path, Query{SessionID: "s1", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestChainContinuesAfterRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger1, err := NewLogger(path)
	require.NoError(t, err)
	require.NoError(t, logger1.Log(Entry{EventType: ActionProposed}))
	_ = logger1.Close()

	// Open a new logger on the same file — should continue the chain.
	logger2, err := NewLogger(path)
	require.NoError(t, err)
	require.NoError(t, logger2.Log(Entry{EventType: ActionExecuted}))
	_ = logger2.Close()

	assert.NoError(t, VerifyIntegrity(path))
}
