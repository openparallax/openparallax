package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTier3ManagerApprove(t *testing.T) {
	m := NewTier3Manager(10, 5)

	pa := &PendingAction{
		ID:     "test-1",
		Action: &types.ActionRequest{RequestID: "r1", Type: types.ActionExecCommand},
	}

	done := make(chan bool, 1)
	go func() {
		approved, err := m.Submit(context.Background(), pa)
		require.NoError(t, err)
		done <- approved
	}()

	time.Sleep(10 * time.Millisecond)
	err := m.Decide("test-1", true)
	require.NoError(t, err)

	result := <-done
	assert.True(t, result)
}

func TestTier3ManagerDeny(t *testing.T) {
	m := NewTier3Manager(10, 5)

	pa := &PendingAction{
		ID:     "test-2",
		Action: &types.ActionRequest{RequestID: "r2", Type: types.ActionDeleteFile},
	}

	done := make(chan bool, 1)
	go func() {
		approved, _ := m.Submit(context.Background(), pa)
		done <- approved
	}()

	time.Sleep(10 * time.Millisecond)
	_ = m.Decide("test-2", false)

	assert.False(t, <-done)
}

func TestTier3ManagerTimeout(t *testing.T) {
	m := NewTier3Manager(10, 1) // 1 second timeout

	pa := &PendingAction{
		ID:     "test-3",
		Action: &types.ActionRequest{RequestID: "r3", Type: types.ActionExecCommand},
	}

	start := time.Now()
	approved, err := m.Submit(context.Background(), pa)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.False(t, approved)
	assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond)
}

func TestTier3RateLimitExceeded(t *testing.T) {
	m := NewTier3Manager(3, 1)

	for i := 0; i < 3; i++ {
		pa := &PendingAction{
			ID:     "rl-" + string(rune('a'+i)),
			Action: &types.ActionRequest{RequestID: "r"},
		}
		go func() {
			_, _ = m.Submit(context.Background(), pa)
		}()
		time.Sleep(5 * time.Millisecond)
	}

	assert.True(t, m.RateLimitExceeded())
}

func TestTier3RateLimitNotExceeded(t *testing.T) {
	m := NewTier3Manager(10, 5)
	assert.False(t, m.RateLimitExceeded())
}

func TestTier3DecideUnknownAction(t *testing.T) {
	m := NewTier3Manager(10, 5)
	err := m.Decide("nonexistent", true)
	assert.Error(t, err)
}

func TestTier3HourlyRemaining(t *testing.T) {
	m := NewTier3Manager(10, 5)
	assert.Equal(t, 10, m.HourlyRemaining())
}

func TestTier3Pending(t *testing.T) {
	m := NewTier3Manager(10, 30)

	pa := &PendingAction{
		ID:     "pending-1",
		Action: &types.ActionRequest{RequestID: "r1"},
	}

	go func() {
		_, _ = m.Submit(context.Background(), pa)
	}()

	time.Sleep(10 * time.Millisecond)
	pending := m.Pending()
	assert.Len(t, pending, 1)
	assert.Equal(t, "pending-1", pending[0].ID)

	_ = m.Decide("pending-1", true)
	time.Sleep(10 * time.Millisecond)

	assert.Empty(t, m.Pending())
}

func TestTier3ContextCancellation(t *testing.T) {
	m := NewTier3Manager(10, 30)
	ctx, cancel := context.WithCancel(context.Background())

	pa := &PendingAction{
		ID:     "ctx-1",
		Action: &types.ActionRequest{RequestID: "r1"},
	}

	done := make(chan error, 1)
	go func() {
		_, err := m.Submit(ctx, pa)
		done <- err
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	err := <-done
	assert.Error(t, err)
}
