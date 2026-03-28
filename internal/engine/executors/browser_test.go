package executors

import (
	"context"
	"os/exec"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestBrowserExecutorNavigateMissingURL(t *testing.T) {
	b := &BrowserExecutor{browserPath: "/usr/bin/echo"}
	result := b.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionBrowserNav,
		Payload: map[string]any{},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "url is required")
}

func TestBrowserExecutorUnknownAction(t *testing.T) {
	b := &BrowserExecutor{browserPath: "/usr/bin/echo"}
	result := b.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: "browser_unknown",
		Payload: map[string]any{},
	})

	assert.False(t, result.Success)
}

func TestNewBrowserExecutorNilSafe(t *testing.T) {
	b := NewBrowserExecutor(nil)
	_ = b // May be nil or non-nil depending on system — just ensure no panic.
}

func TestBrowserSupportedActions(t *testing.T) {
	b := &BrowserExecutor{browserPath: "/usr/bin/echo"}
	actions := b.SupportedActions()
	assert.NotEmpty(t, actions)
	assert.Contains(t, actions, types.ActionBrowserNav)
}

func TestBrowserToolSchemas(t *testing.T) {
	b := &BrowserExecutor{browserPath: "/usr/bin/echo"}
	schemas := b.ToolSchemas()
	assert.NotEmpty(t, schemas)
	for _, s := range schemas {
		assert.NotEmpty(t, s.Name)
		assert.NotEmpty(t, s.Description)
	}
}

func TestBrowserNavigateWithEcho(t *testing.T) {
	echoPath, err := exec.LookPath("echo")
	if err != nil {
		t.Skip("echo not in PATH")
	}

	b := &BrowserExecutor{browserPath: echoPath}
	result := b.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionBrowserNav,
		Payload: map[string]any{"url": "https://example.com"},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "https://example.com")
}

func TestDetectBrowserReturnsPath(t *testing.T) {
	path := DetectBrowser()
	if path == "" {
		t.Skip("no browser detected on this system")
	}
	assert.NotEmpty(t, path)
}
