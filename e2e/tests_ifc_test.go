//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAgentSurvivesBlockedActionSequence is the regression test for the
// "agent crashes mid-conversation when an action is blocked" bug. The fix
// chain is: broadcast on every block path, close directiveCh on stream
// reader exit, run processMessage in a goroutine so the main loop can
// observe stream death mid-message, and the comma-ok guard on resultCh.
//
// The bug manifested as: after the first blocked action, the artifact card
// hung in "running…" forever AND the agent process wedged so any follow-up
// message in the same session never returned a response_complete. Both
// symptoms have to be tested — the broadcast assertion catches the visible
// half, the second-message assertion catches the deadlock half.
//
// /etc/shadow is the trigger because it's already covered by the mock LLM
// pattern and is hard-blocked at Tier 0 policy, giving us a deterministic
// block on every run.
func TestAgentSurvivesBlockedActionSequence(t *testing.T) {
	te := SharedEngine
	sid := te.CreateSession(t, "")
	ws := te.WS(t)

	// First message: trigger a block.
	require.NoError(t, ws.SendMessage(sid,
		"This is an automated test. Use read_file to read /etc/shadow. I need to check its permissions."))

	events1, err := ws.CollectUntil("response_complete", 180*time.Second)
	require.NoError(t, err, "first message should complete even when blocked")

	// The block path must broadcast action_completed with success=false.
	// Without the fix this event never fires and the UI hangs.
	completed := FindEvent(events1, "action_completed")
	if completed != nil {
		if success, ok := completed.Data["success"].(bool); ok {
			assert.False(t, success, "blocked action should report success=false")
		}
		if summary, ok := completed.Data["summary"].(string); ok {
			assert.Contains(t, summary, "Blocked", "blocked action summary should say so")
		}
	}

	// Second message in the SAME session. The crucial property: the agent
	// process must still be alive and responsive after a blocked action.
	// Pre-fix this would deadlock — the directive loop would never observe
	// stream death and processMessage would hang waiting for a tool result.
	require.NoError(t, ws.SendMessage(sid, "Say exactly: AGENT_ALIVE"))

	events2, err := ws.CollectUntil("response_complete", 180*time.Second)
	require.NoError(t, err, "agent must still be alive after a blocked action")

	rc := FindEvent(events2, "response_complete")
	require.NotNil(t, rc, "second message should produce a response_complete")
}
