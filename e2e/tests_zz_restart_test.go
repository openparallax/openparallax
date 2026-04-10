//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Audit chain valid after multiple actions.
func TestAuditChainIntegrity(t *testing.T) {
	te := SharedEngine
	sid := te.CreateSession(t, "")
	ws := te.WS(t)

	for _, msg := range []string{"hello", "hello again", "one more"} {
		require.NoError(t, ws.SendMessage(sid, msg))
		_, _ = ws.CollectUntil("response_complete", 120*time.Second)
	}

	var audit map[string]any
	code, err := te.REST().GetJSON("/api/audit?verify=true", &audit)
	require.NoError(t, err)
	assert.Equal(t, 200, code)

	if valid, ok := audit["valid"].(bool); ok {
		assert.True(t, valid, "audit chain should be valid")
	}
}
