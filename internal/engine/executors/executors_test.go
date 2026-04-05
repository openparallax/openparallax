package executors

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileReadExisting(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello world"), 0o644))

	f := NewFileExecutor(dir)
	result := f.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionReadFile,
		Payload: map[string]any{"path": "test.txt"},
	})

	assert.True(t, result.Success)
	assert.Equal(t, "hello world", result.Output)
}

func TestFileReadNonexistent(t *testing.T) {
	f := NewFileExecutor(t.TempDir())
	result := f.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionReadFile,
		Payload: map[string]any{"path": "nonexistent.txt"},
	})

	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)
}

func TestFileWriteCreates(t *testing.T) {
	dir := t.TempDir()
	f := NewFileExecutor(dir)
	result := f.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionWriteFile,
		Payload: map[string]any{"path": "new.txt", "content": "hello"},
	})

	assert.True(t, result.Success)

	data, err := os.ReadFile(filepath.Join(dir, "new.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestFileWriteCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	f := NewFileExecutor(dir)
	result := f.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionWriteFile,
		Payload: map[string]any{"path": "subdir/deep/file.txt", "content": "nested"},
	})

	assert.True(t, result.Success)
	data, err := os.ReadFile(filepath.Join(dir, "subdir", "deep", "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "nested", string(data))
}

func TestFileDelete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deleteme.txt")
	require.NoError(t, os.WriteFile(path, []byte("bye"), 0o644))

	f := NewFileExecutor(dir)
	result := f.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionDeleteFile,
		Payload: map[string]any{"path": "deleteme.txt"},
	})

	assert.True(t, result.Success)
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestFileMove(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src.txt"), []byte("content"), 0o644))

	f := NewFileExecutor(dir)
	result := f.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionMoveFile,
		Payload: map[string]any{"source": "src.txt", "destination": "dst.txt"},
	})

	assert.True(t, result.Success)
	_, err := os.Stat(filepath.Join(dir, "src.txt"))
	assert.True(t, os.IsNotExist(err))
	data, err := os.ReadFile(filepath.Join(dir, "dst.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content", string(data))
}

func TestFileCopy(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "original.txt"), []byte("data"), 0o644))

	f := NewFileExecutor(dir)
	result := f.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCopyFile,
		Payload: map[string]any{"source": "original.txt", "destination": "copy.txt"},
	})

	assert.True(t, result.Success)
	origData, _ := os.ReadFile(filepath.Join(dir, "original.txt"))
	copyData, _ := os.ReadFile(filepath.Join(dir, "copy.txt"))
	assert.Equal(t, origData, copyData)
}

func TestFileCreateDir(t *testing.T) {
	dir := t.TempDir()
	f := NewFileExecutor(dir)
	result := f.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCreateDir,
		Payload: map[string]any{"path": "newdir"},
	})

	assert.True(t, result.Success)
	info, err := os.Stat(filepath.Join(dir, "newdir"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestFileListDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bb"), 0o644))

	f := NewFileExecutor(dir)
	result := f.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionListDir,
		Payload: map[string]any{"path": "."},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "a.txt")
	assert.Contains(t, result.Output, "b.txt")
	assert.Contains(t, result.Summary, "2 entries")
}

func TestFileSearchFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.go"), []byte("package main"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("text"), 0o644))

	f := NewFileExecutor(dir)
	result := f.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionSearchFiles,
		Payload: map[string]any{"path": ".", "pattern": "*.go"},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "hello.go")
	assert.NotContains(t, result.Output, "hello.txt")
}

func TestFileTildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	f := NewFileExecutor(home)
	path := f.resolvePath("~/test_resolve_path_check")
	assert.Equal(t, filepath.Join(home, "test_resolve_path_check"), path)
}

func TestFileRelativePathResolution(t *testing.T) {
	workspace := "/tmp/test-workspace"
	f := NewFileExecutor(workspace)
	path := f.resolvePath("subdir/file.txt")
	assert.Equal(t, filepath.Join(workspace, "subdir", "file.txt"), path)
}

func TestShellEchoHello(t *testing.T) {
	s := NewShellExecutor()
	result := s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionExecCommand,
		Payload: map[string]any{"command": "echo hello"},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "hello")
}

func TestShellTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("timeout test uses sleep which behaves differently on Windows")
	}

	s := NewShellExecutor()
	// The executor has a 30-second timeout. Use a 1-second context timeout instead
	// to avoid waiting 30 seconds in tests.
	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()

	result := s.Execute(ctx, &types.ActionRequest{
		RequestID: "r1", Type: types.ActionExecCommand,
		Payload: map[string]any{"command": "sleep 60"},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "timed out")
}

func TestShellNonexistentCommand(t *testing.T) {
	s := NewShellExecutor()
	result := s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionExecCommand,
		Payload: map[string]any{"command": "nonexistent_command_12345"},
	})

	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)
}

func TestShellEmptyCommand(t *testing.T) {
	s := NewShellExecutor()
	result := s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionExecCommand,
		Payload: map[string]any{"command": ""},
	})

	assert.False(t, result.Success)
	assert.Equal(t, "empty command", result.Error)
}

func TestRegistryDispatchesKnownAction(t *testing.T) {
	r := NewRegistry(t.TempDir(), nil, nil, nil)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("content"), 0o644))

	// Use a file executor-backed action with the workspace from the registry.
	result := r.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionExecCommand,
		Payload: map[string]any{"command": "echo registry_works"},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "registry_works")
}

func TestRegistryRejectsUnknownAction(t *testing.T) {
	r := NewRegistry(t.TempDir(), nil, nil, nil)
	result := r.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: "unknown_action_type",
		Payload: map[string]any{},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "no executor registered")
}

func TestDetectLanguage(t *testing.T) {
	assert.Equal(t, "go", detectLanguage("main.go"))
	assert.Equal(t, "python", detectLanguage("script.py"))
	assert.Equal(t, "markdown", detectLanguage("README.md"))
	assert.Equal(t, "text", detectLanguage("data.xyz"))
}

func TestDetectPreviewType(t *testing.T) {
	assert.Equal(t, "markdown", detectPreviewType("README.md"))
	assert.Equal(t, "html", detectPreviewType("page.html"))
	assert.Equal(t, "image", detectPreviewType("photo.png"))
	assert.Equal(t, "code", detectPreviewType("main.go"))
}
