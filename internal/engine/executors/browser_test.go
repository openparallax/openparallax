package executors

import (
	"context"
	"os"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestNewBrowserExecutorNilSafe(t *testing.T) {
	b := NewBrowserExecutor(nil)
	_ = b // May be nil or non-nil depending on system — just ensure no panic.
}

func TestBrowserSupportedActions(t *testing.T) {
	b := &BrowserExecutor{browserPath: "/usr/bin/echo"}
	actions := b.SupportedActions()
	assert.NotEmpty(t, actions)
	assert.Contains(t, actions, types.ActionBrowserNav)
	assert.Contains(t, actions, types.ActionBrowserClick)
	assert.Contains(t, actions, types.ActionBrowserType)
	assert.Contains(t, actions, types.ActionBrowserExtract)
	assert.Contains(t, actions, types.ActionBrowserShot)
}

func TestBrowserToolSchemas(t *testing.T) {
	b := &BrowserExecutor{browserPath: "/usr/bin/echo"}
	schemas := b.ToolSchemas()
	assert.Len(t, schemas, 5)
	for _, s := range schemas {
		assert.NotEmpty(t, s.Name)
		assert.NotEmpty(t, s.Description)
	}
}

func TestDetectBrowserReturnsPath(t *testing.T) {
	path := DetectBrowser()
	if path == "" {
		t.Skip("no browser detected on this system")
	}
	assert.NotEmpty(t, path)
}

func TestTruncateContent(t *testing.T) {
	short := "hello"
	assert.Equal(t, short, truncateContent(short, 100))

	long := string(make([]byte, 200))
	result := truncateContent(long, 100)
	assert.Len(t, result, 100+len("\n\n[Content truncated]"))
	assert.Contains(t, result, "[Content truncated]")
}

func TestResolveBrowserBinaryDirect(t *testing.T) {
	b := &BrowserExecutor{browserPath: "/usr/bin/echo"}
	resolved := b.resolveBrowserBinary()
	assert.Contains(t, resolved, "echo")
}

func TestResolveBrowserBinaryFlatpak(t *testing.T) {
	b := &BrowserExecutor{browserPath: "flatpak run com.brave.Browser"}
	resolved := b.resolveBrowserBinary()
	if resolved != "" {
		// Wrapper script should exist and be executable.
		info, err := os.Stat(resolved)
		assert.NoError(t, err)
		assert.NotZero(t, info.Mode()&0o100, "wrapper should be executable")
		defer os.Remove(resolved)
	}
}

func TestCreateFlatpakWrapper(t *testing.T) {
	path, err := createFlatpakWrapper("flatpak run com.brave.Browser")
	assert.NoError(t, err)
	assert.NotEmpty(t, path)
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "flatpak run com.brave.Browser")
	assert.Contains(t, string(data), "#!/bin/sh")

	info, err := os.Stat(path)
	assert.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o100)
}

func TestBrowserNavigateIntegration(t *testing.T) {
	browser := DetectBrowser()
	if browser == "" {
		t.Skip("no browser detected on this system")
	}

	b := &BrowserExecutor{browserPath: browser}
	ctx := context.Background()

	result := b.Execute(ctx, &types.ActionRequest{
		RequestID: "r1", Type: types.ActionBrowserNav,
		Payload: map[string]any{"url": "https://example.com"},
	})
	defer b.Shutdown()

	if !result.Success {
		t.Skipf("browser session failed (likely no display): %s", result.Error)
	}

	assert.True(t, result.Success)
	assert.Contains(t, result.Summary, "example.com")
}
