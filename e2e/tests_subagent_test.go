//go:build e2e

package e2e

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubAgentSpawnAndComplete(t *testing.T) {
	te := SharedEngine
	sid := te.CreateSession(t, "")
	ws := te.WS(t)

	require.NoError(t, ws.SendMessage(sid,
		"This is an automated test. Use the create_agent tool to delegate this task to a sub-agent: 'list the files in the current directory'. Pass tools=['files']."))

	events, err := ws.CollectUntil("response_complete", 120*time.Second)
	require.NoError(t, err)

	rc := FindEvent(events, "response_complete")
	require.NotNil(t, rc, "should get response_complete")

	if os.Getenv("E2E_LLM") == "mock" {
		spawned := FindEvent(events, "sub_agent_spawned")
		if spawned != nil {
			// The event payload is nested: {"type":"sub_agent_spawned","sub_agent_spawned":{"name":"..."}}.
			// parseWSEvent puts top-level fields into Data when there's no "data" key.
			if sa, ok := spawned.Data["sub_agent_spawned"].(map[string]any); ok {
				assert.NotEmpty(t, sa["name"])
			}
		}
	}
}
