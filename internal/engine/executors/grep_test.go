package executors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGrepTestWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeTestFile(t, dir, "main.go", "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n")
	writeTestFile(t, dir, "utils.go", "package main\n\nfunc helper() string {\n\treturn \"helper\"\n}\n")
	writeTestFile(t, dir, "readme.md", "# Project\n\nThis is a test project.\n")
	writeTestFile(t, dir, "data.csv", "name,value\nalice,100\nbob,200\n")

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
	writeTestFile(t, dir, ".git/config", "this should be excluded")

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "vendor"), 0o755))
	writeTestFile(t, dir, "vendor/dep.go", "package dep\nfunc main() {}")

	return dir
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestGrepLiteralString(t *testing.T) {
	dir := newGrepTestWorkspace(t)
	exec := NewFileExecutor(dir)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionGrepFiles, Payload: map[string]any{"pattern": "hello"},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "main.go")
	assert.Contains(t, result.Output, "hello")
	assert.Contains(t, result.Summary, "1 match")
}

func TestGrepRegexPattern(t *testing.T) {
	dir := newGrepTestWorkspace(t)
	exec := NewFileExecutor(dir)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionGrepFiles, Payload: map[string]any{"pattern": `func \w+\(`},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "main.go")
	assert.Contains(t, result.Output, "utils.go")
	assert.Contains(t, result.Output, "func main()")
	assert.Contains(t, result.Output, "func helper()")
}

func TestGrepIncludeGlob(t *testing.T) {
	dir := newGrepTestWorkspace(t)
	exec := NewFileExecutor(dir)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionGrepFiles, Payload: map[string]any{"pattern": "package", "include": "*.go"},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "main.go")
	assert.NotContains(t, result.Output, "readme.md")
}

func TestGrepExcludeGlob(t *testing.T) {
	dir := newGrepTestWorkspace(t)
	exec := NewFileExecutor(dir)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionGrepFiles, Payload: map[string]any{"pattern": "package", "exclude": "*.go"},
	})
	require.True(t, result.Success)
	assert.Equal(t, "No matches found.", result.Output)
}

func TestGrepDefaultExcludesGitDir(t *testing.T) {
	dir := newGrepTestWorkspace(t)
	exec := NewFileExecutor(dir)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionGrepFiles, Payload: map[string]any{"pattern": "excluded"},
	})
	require.True(t, result.Success)
	assert.NotContains(t, result.Output, ".git")
}

func TestGrepDefaultExcludesVendor(t *testing.T) {
	dir := newGrepTestWorkspace(t)
	exec := NewFileExecutor(dir)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionGrepFiles, Payload: map[string]any{"pattern": "package dep"},
	})
	require.True(t, result.Success)
	assert.Equal(t, "No matches found.", result.Output)
}

func TestGrepCaseInsensitive(t *testing.T) {
	dir := newGrepTestWorkspace(t)
	exec := NewFileExecutor(dir)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionGrepFiles, Payload: map[string]any{"pattern": "HELLO", "case_sensitive": false},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "hello")
}

func TestGrepNoMatches(t *testing.T) {
	dir := newGrepTestWorkspace(t)
	exec := NewFileExecutor(dir)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionGrepFiles, Payload: map[string]any{"pattern": "nonexistent_string_xyz"},
	})
	require.True(t, result.Success)
	assert.Equal(t, "No matches found.", result.Output)
}

func TestGrepInvalidRegex(t *testing.T) {
	dir := newGrepTestWorkspace(t)
	exec := NewFileExecutor(dir)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionGrepFiles, Payload: map[string]any{"pattern": "[invalid"},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "invalid pattern")
}

func TestGrepBinaryFileSkipped(t *testing.T) {
	dir := t.TempDir()
	// Write a binary file with null bytes.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "binary.bin"), []byte("hello\x00world"), 0o644))
	writeTestFile(t, dir, "text.txt", "hello world")

	exec := NewFileExecutor(dir)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionGrepFiles, Payload: map[string]any{"pattern": "hello"},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "text.txt")
	assert.NotContains(t, result.Output, "binary.bin")
}

func TestGrepMaxResults(t *testing.T) {
	dir := t.TempDir()
	// Create many files with the same content.
	for i := range 10 {
		name := filepath.Join(dir, fmt.Sprintf("file%d.txt", i))
		content := "line one match\nline two match\nline three match\n"
		require.NoError(t, os.WriteFile(name, []byte(content), 0o644))
	}

	exec := NewFileExecutor(dir)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionGrepFiles, Payload: map[string]any{"pattern": "match", "max_results": float64(5)},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Summary, "5 match")
}

func TestIsBinaryFile(t *testing.T) {
	dir := t.TempDir()

	textPath := filepath.Join(dir, "text.txt")
	require.NoError(t, os.WriteFile(textPath, []byte("plain text"), 0o644))
	assert.False(t, isBinaryFile(textPath))

	binPath := filepath.Join(dir, "binary.bin")
	require.NoError(t, os.WriteFile(binPath, []byte("has\x00null"), 0o644))
	assert.True(t, isBinaryFile(binPath))
}
