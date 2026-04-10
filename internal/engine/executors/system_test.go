package executors

import (
	"context"
	"testing"
	"time"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemExecutorSupportedActions(t *testing.T) {
	exec := newSystemExecutorUnchecked(t.TempDir())
	actions := exec.SupportedActions()
	assert.Len(t, actions, 6)
}

func TestSystemExecutorToolSchemas(t *testing.T) {
	exec := newSystemExecutorUnchecked(t.TempDir())
	schemas := exec.ToolSchemas()
	assert.Len(t, schemas, 6)
	names := make(map[string]bool)
	for _, s := range schemas {
		names[s.Name] = true
	}
	assert.True(t, names["clipboard_read"])
	assert.True(t, names["clipboard_write"])
	assert.True(t, names["open"])
	assert.True(t, names["notify"])
	assert.True(t, names["system_info"])
	assert.True(t, names["screenshot"])
}

func TestClipboardWriteEmptyContent(t *testing.T) {
	exec := newSystemExecutorUnchecked(t.TempDir())
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionClipboardWrite, Payload: map[string]any{},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "content is required")
}

func TestClipboardWriteTooLarge(t *testing.T) {
	exec := newSystemExecutorUnchecked(t.TempDir())
	large := make([]byte, clipboardMaxWrite+1)
	for i := range large {
		large[i] = 'x'
	}
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionClipboardWrite, Payload: map[string]any{"content": string(large)},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "too large")
}

func TestOpenRejectsUnsupportedScheme(t *testing.T) {
	exec := newSystemExecutorUnchecked(t.TempDir())
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionOpen, Payload: map[string]any{"target": "file:///etc/passwd"},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "unsupported scheme")
}

func TestOpenRejectsJavascriptScheme(t *testing.T) {
	exec := newSystemExecutorUnchecked(t.TempDir())
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionOpen, Payload: map[string]any{"target": "javascript:alert(1)"},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "unsupported scheme")
}

func TestOpenRejectsRelativePath(t *testing.T) {
	exec := newSystemExecutorUnchecked(t.TempDir())
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionOpen, Payload: map[string]any{"target": "relative/file.txt"},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "absolute")
}

func TestOpenEmptyTarget(t *testing.T) {
	exec := newSystemExecutorUnchecked(t.TempDir())
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionOpen, Payload: map[string]any{},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "target is required")
}

func TestNotifyEmptyFields(t *testing.T) {
	exec := newSystemExecutorUnchecked(t.TempDir())
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionNotify, Payload: map[string]any{},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "title and message are required")
}

func TestNotifyRateLimit(t *testing.T) {
	exec := newSystemExecutorUnchecked(t.TempDir())
	// Fill the rate limit window.
	now := time.Now()
	for range notifyRateLimit {
		exec.notifyMu.Lock()
		exec.notifyTimes = append(exec.notifyTimes, now)
		exec.notifyMu.Unlock()
	}

	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionNotify, Payload: map[string]any{"title": "Test", "message": "Hello"},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "rate limit")
}

func TestSystemInfoAllCategory(t *testing.T) {
	exec := newSystemExecutorUnchecked(t.TempDir())
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionSystemInfo, Payload: map[string]any{"category": "all"},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "Host:")
	assert.Contains(t, result.Output, "CPU:")
	assert.Contains(t, result.Output, "Memory:")
	assert.Contains(t, result.Output, "Network:")
}

func TestSystemInfoMemoryCategory(t *testing.T) {
	exec := newSystemExecutorUnchecked(t.TempDir())
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionSystemInfo, Payload: map[string]any{"category": "memory"},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "Memory:")
	assert.NotContains(t, result.Output, "CPU:")
}

func TestSystemInfoCPUCategory(t *testing.T) {
	exec := newSystemExecutorUnchecked(t.TempDir())
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionSystemInfo, Payload: map[string]any{"category": "cpu"},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "CPU:")
	assert.Contains(t, result.Output, "Cores:")
}

func TestSystemInfoNetworkCategory(t *testing.T) {
	exec := newSystemExecutorUnchecked(t.TempDir())
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionSystemInfo, Payload: map[string]any{"category": "network"},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "Network:")
}

func TestSystemInfoHostCategory(t *testing.T) {
	exec := newSystemExecutorUnchecked(t.TempDir())
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionSystemInfo, Payload: map[string]any{"category": "host"},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "Host:")
	assert.Contains(t, result.Output, "os:")
	assert.Contains(t, result.Output, "arch:")
	assert.Contains(t, result.Output, "username:")
	assert.Contains(t, result.Output, "home:")
	assert.Contains(t, result.Output, "timezone:")
}

func TestSystemInfoUnknownCategory(t *testing.T) {
	exec := newSystemExecutorUnchecked(t.TempDir())
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionSystemInfo, Payload: map[string]any{"category": "invalid"},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "unknown category")
}

func TestFormatFileSize(t *testing.T) {
	assert.Equal(t, "100 B", formatFileSize(100))
	assert.Equal(t, "1.0 KB", formatFileSize(1024))
	assert.Equal(t, "1.5 MB", formatFileSize(1572864))
}
