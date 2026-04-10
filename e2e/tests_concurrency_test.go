//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Dynamic tool loading via load_tools.
func TestDynamicToolLoading(t *testing.T) {
	te := SharedEngine
	sid := te.CreateSession(t, "")
	ws := te.WS(t)

	require.NoError(t, ws.SendMessage(sid, "hello, what tools do you have?"))

	events, err := ws.CollectUntil("response_complete", 120*time.Second)
	require.NoError(t, err)

	rc := FindEvent(events, "response_complete")
	require.NotNil(t, rc, "should receive response_complete")
}
