package executors

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanvasCreateHTML(t *testing.T) {
	dir := t.TempDir()
	c := NewCanvasExecutor(dir)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCanvasCreate,
		Payload: map[string]any{"path": "index.html", "content": "<h1>Hello</h1>", "type": "html"},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "index.html")
	assert.NotNil(t, result.Artifact)

	data, err := os.ReadFile(filepath.Join(dir, "index.html"))
	require.NoError(t, err)
	assert.Equal(t, "<h1>Hello</h1>", string(data))
}

func TestCanvasCreateNestedPath(t *testing.T) {
	dir := t.TempDir()
	c := NewCanvasExecutor(dir)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCanvasCreate,
		Payload: map[string]any{"path": "deep/nested/file.css", "content": "body{}", "type": "css"},
	})

	assert.True(t, result.Success)
	data, err := os.ReadFile(filepath.Join(dir, "deep", "nested", "file.css"))
	require.NoError(t, err)
	assert.Equal(t, "body{}", string(data))
}

func TestCanvasUpdate(t *testing.T) {
	dir := t.TempDir()
	c := NewCanvasExecutor(dir)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "page.html"), []byte("old"), 0o644))

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCanvasUpdate,
		Payload: map[string]any{"path": "page.html", "content": "<h1>Updated</h1>"},
	})

	assert.True(t, result.Success)
	data, err := os.ReadFile(filepath.Join(dir, "page.html"))
	require.NoError(t, err)
	assert.Equal(t, "<h1>Updated</h1>", string(data))
}

func TestCanvasProject(t *testing.T) {
	dir := t.TempDir()
	c := NewCanvasExecutor(dir)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCanvasCreate,
		Payload: map[string]any{
			"_tool_name": "canvas_project",
			"path":       "mysite",
			"files": []any{
				map[string]any{"name": "index.html", "content": "<h1>Home</h1>", "type": "html"},
				map[string]any{"name": "style.css", "content": "body{margin:0}", "type": "css"},
			},
		},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "2 files")

	data, err := os.ReadFile(filepath.Join(dir, "mysite", "index.html"))
	require.NoError(t, err)
	assert.Equal(t, "<h1>Home</h1>", string(data))
}

func TestCanvasProjectEmptyFiles(t *testing.T) {
	dir := t.TempDir()
	c := NewCanvasExecutor(dir)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCanvasCreate,
		Payload: map[string]any{"_tool_name": "canvas_project", "path": "empty", "files": []any{}},
	})

	assert.False(t, result.Success)
}

func TestCanvasPreview(t *testing.T) {
	dir := t.TempDir()
	c := NewCanvasExecutor(dir)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>Preview</h1>"), 0o644))

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCanvasCreate,
		Payload: map[string]any{"_tool_name": "canvas_preview", "path": "index.html"},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "127.0.0.1")
}

func TestCanvasPreviewNonexistent(t *testing.T) {
	dir := t.TempDir()
	c := NewCanvasExecutor(dir)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCanvasCreate,
		Payload: map[string]any{"_tool_name": "canvas_preview", "path": "nonexistent"},
	})

	assert.False(t, result.Success)
}

func TestCanvasSupportedActions(t *testing.T) {
	c := NewCanvasExecutor(t.TempDir())
	assert.NotEmpty(t, c.SupportedActions())
}

func TestCanvasToolSchemas(t *testing.T) {
	c := NewCanvasExecutor(t.TempDir())
	schemas := c.ToolSchemas()
	assert.NotEmpty(t, schemas)
	for _, s := range schemas {
		assert.NotEmpty(t, s.Name)
	}
}
