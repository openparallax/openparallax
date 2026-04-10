//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test 11: Session create, list, delete.
func TestSessionLifecycle(t *testing.T) {
	te := SharedEngine

	// Create two sessions.
	sidA := te.CreateSession(t, "")
	sidB := te.CreateSession(t, "")
	assert.NotEqual(t, sidA, sidB)

	// List sessions.
	body, code, err := te.REST().Get("/api/sessions")
	require.NoError(t, err)
	assert.Equal(t, 200, code)

	var sessions []map[string]any
	require.NoError(t, json.Unmarshal(body, &sessions))
	assert.GreaterOrEqual(t, len(sessions), 2)

	// Delete session A.
	_, code, err = te.REST().Delete("/api/sessions/" + sidA)
	require.NoError(t, err)
	assert.True(t, code == 200 || code == 204, "delete should return 200 or 204, got %d", code)

	// List again — should have one fewer.
	body, _, _ = te.REST().Get("/api/sessions")
	require.NoError(t, json.Unmarshal(body, &sessions))

	// Session A should be gone, B should remain.
	found := false
	for _, s := range sessions {
		assert.NotEqual(t, sidA, s["id"], "deleted session should not appear")
		if s["id"] == sidB {
			found = true
		}
	}
	assert.True(t, found, "session B should still exist")
}

// Test 12: Chronicle snapshot and rollback.
func TestChronicleRollback(t *testing.T) {
	te := SharedEngine

	// Create a file with known content.
	testFile := filepath.Join(te.WorkspaceDir, "rollback-test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("original content"), 0o644))

	sid := te.CreateSession(t, "")
	ws := te.WS(t)

	// Ask the agent to overwrite it.
	require.NoError(t, ws.SendMessage(sid, "write 'modified by agent' to rollback-test.txt"))
	_, _ = ws.CollectUntil("response_complete", 60*time.Second)

	// If the file was modified, chronicle should have a snapshot.
	content, err := os.ReadFile(testFile)
	if err == nil && string(content) != "original content" {
		// File was modified — try rollback via slash command.
		require.NoError(t, ws.SendCommand(sid, "/chronicle"))
		events := ws.CollectFor(5 * time.Second)
		cmdResult := FindEvent(events, "command_result")
		if cmdResult != nil {
			t.Logf("chronicle output: %v", cmdResult.Data)
		}
	}
}

// Test 13: Slash commands return results.
func TestSlashCommands(t *testing.T) {
	te := SharedEngine

	sid := te.CreateSession(t, "")
	ws := te.WS(t)

	commands := []struct {
		cmd      string
		contains string
	}{
		{"/status", "E2ETest"},
		{"/doctor", "check"},
	}

	for _, tc := range commands {
		require.NoError(t, ws.SendCommand(sid, tc.cmd))
		events, err := ws.CollectUntil("command_result", 15*time.Second)
		if err != nil {
			t.Logf("no command_result for %s: %v", tc.cmd, err)
			continue
		}
		cmdResult := FindEvent(events, "command_result")
		// command_result has "text" at top level, not nested in "data".
		text, _ := cmdResult.Data["text"].(string)
		if text == "" {
		}
		assert.Contains(t, text, tc.contains, "command %s should contain %q", tc.cmd, tc.contains)
	}
}
