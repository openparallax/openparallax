//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOTRBlocksWrites(t *testing.T) {
	te := SharedEngine
	sid := te.CreateSession(t, "otr")
	ws := te.WS(t)

	require.NoError(t, ws.SendOTRMessage(sid,
		"This is an automated test. Use write_file to create a file named 'otr-test.txt' with content 'should not exist'."))

	events, err := ws.CollectUntil("response_complete", 120*time.Second)
	require.NoError(t, err)

	rc := FindEvent(events, "response_complete")
	require.NotNil(t, rc, "should get response_complete")

	// File should not exist — OTR blocks writes.
	_, statErr := os.Stat(filepath.Join(te.WorkspaceDir, "otr-test.txt"))
	assert.True(t, os.IsNotExist(statErr), "file should not exist in OTR mode")
}

func TestSandboxActive(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Landlock sandbox test only runs on Linux")
	}

	te := SharedEngine

	var status map[string]any
	code, err := te.REST().GetJSON("/api/status", &status)
	require.NoError(t, err)
	assert.Equal(t, 200, code)

	sandbox, ok := status["sandbox"].(map[string]any)
	if !ok {
		t.Fatal("status response missing sandbox field")
	}

	assert.Equal(t, true, sandbox["active"], "sandbox should be active on Linux")
	assert.Equal(t, "landlock", sandbox["mode"])
}
