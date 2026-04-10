package executors

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed: %s", args, out)
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0o644))
	run("add", ".")
	run("commit", "-m", "initial commit")
	return dir
}

func TestGitStatus(t *testing.T) {
	dir := initGitRepo(t)
	g := NewGitExecutor(dir)

	// Clean repo — status should succeed with empty or minimal output.
	result := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionGitStatus,
		Payload: map[string]any{},
	})
	assert.True(t, result.Success)

	// Create untracked file — should appear in status.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0o644))
	result = g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r2", Type: types.ActionGitStatus,
		Payload: map[string]any{},
	})
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "new.txt")
}

func TestGitDiff(t *testing.T) {
	dir := initGitRepo(t)
	g := NewGitExecutor(dir)

	// Modify tracked file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Updated"), 0o644))

	result := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionGitDiff,
		Payload: map[string]any{},
	})
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "Updated")
}

func TestGitDiffStaged(t *testing.T) {
	dir := initGitRepo(t)
	g := NewGitExecutor(dir)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Staged"), 0o644))
	cmd := exec.Command("git", "add", "README.md")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	result := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionGitDiff,
		Payload: map[string]any{"staged": true},
	})
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "Staged")
}

func TestGitLog(t *testing.T) {
	dir := initGitRepo(t)
	g := NewGitExecutor(dir)

	result := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionGitLog,
		Payload: map[string]any{},
	})
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "initial commit")
}

func TestGitLogWithLimit(t *testing.T) {
	dir := initGitRepo(t)
	g := NewGitExecutor(dir)

	// Add a second commit.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "second.txt"), []byte("2"), 0o644))
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "commit", "-m", "second commit")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
	require.NoError(t, cmd.Run())

	result := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionGitLog,
		Payload: map[string]any{"limit": float64(1)},
	})
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "second commit")
	assert.NotContains(t, result.Output, "initial commit")
}

func TestGitCommit(t *testing.T) {
	dir := initGitRepo(t)
	g := NewGitExecutor(dir)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "committed.txt"), []byte("data"), 0o644))

	result := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionGitCommit,
		Payload: map[string]any{"message": "add committed.txt"},
	})
	assert.True(t, result.Success)

	// Verify commit is in log.
	logResult := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r2", Type: types.ActionGitLog,
		Payload: map[string]any{},
	})
	assert.Contains(t, logResult.Output, "add committed.txt")
}

func TestGitCommitSpecificFiles(t *testing.T) {
	dir := initGitRepo(t)
	g := NewGitExecutor(dir)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644))

	result := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionGitCommit,
		Payload: map[string]any{"message": "add a only", "files": []any{"a.txt"}},
	})
	assert.True(t, result.Success)

	// b.txt should still be untracked.
	status := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r2", Type: types.ActionGitStatus,
		Payload: map[string]any{},
	})
	assert.Contains(t, status.Output, "b.txt")
}

func TestGitCommitMissingMessage(t *testing.T) {
	dir := initGitRepo(t)
	g := NewGitExecutor(dir)

	result := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionGitCommit,
		Payload: map[string]any{},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "commit message is required")
}

func TestGitBranchList(t *testing.T) {
	dir := initGitRepo(t)
	g := NewGitExecutor(dir)

	result := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionGitBranch,
		Payload: map[string]any{"action": "list"},
	})
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "main")
}

func TestGitBranchCreate(t *testing.T) {
	dir := initGitRepo(t)
	g := NewGitExecutor(dir)

	result := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionGitBranch,
		Payload: map[string]any{"action": "create", "name": "feature-x"},
	})
	assert.True(t, result.Success)

	// Verify branch exists.
	list := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r2", Type: types.ActionGitBranch,
		Payload: map[string]any{"action": "list"},
	})
	assert.Contains(t, list.Output, "feature-x")
}

func TestGitBranchCreateMissingName(t *testing.T) {
	dir := initGitRepo(t)
	g := NewGitExecutor(dir)

	result := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionGitBranch,
		Payload: map[string]any{"action": "create"},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "branch name is required")
}

func TestGitBranchSwitch(t *testing.T) {
	dir := initGitRepo(t)
	g := NewGitExecutor(dir)

	// Create and switch.
	g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionGitBranch,
		Payload: map[string]any{"action": "create", "name": "dev"},
	})

	result := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r2", Type: types.ActionGitBranch,
		Payload: map[string]any{"action": "switch", "name": "dev"},
	})
	assert.True(t, result.Success)
}

func TestGitBranchInvalidAction(t *testing.T) {
	dir := initGitRepo(t)
	g := NewGitExecutor(dir)

	result := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionGitBranch,
		Payload: map[string]any{"action": "merge"},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "action must be")
}

func TestGitCheckout(t *testing.T) {
	dir := initGitRepo(t)
	g := NewGitExecutor(dir)

	// Create branch, then checkout.
	g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionGitBranch,
		Payload: map[string]any{"action": "create", "name": "release"},
	})

	result := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r2", Type: types.ActionGitCheckout,
		Payload: map[string]any{"ref": "release"},
	})
	assert.True(t, result.Success)
}

func TestGitCheckoutMissingRef(t *testing.T) {
	dir := initGitRepo(t)
	g := NewGitExecutor(dir)

	result := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionGitCheckout,
		Payload: map[string]any{},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "ref is required")
}

func TestGitCustomPath(t *testing.T) {
	dir := initGitRepo(t)
	g := NewGitExecutor(t.TempDir()) // Different workspace.

	result := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionGitStatus,
		Payload: map[string]any{"path": dir},
	})
	assert.True(t, result.Success)
}

func TestGitNotARepo(t *testing.T) {
	dir := t.TempDir()
	g := NewGitExecutor(dir)

	result := g.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionGitStatus,
		Payload: map[string]any{},
	})
	assert.False(t, result.Success)
}

func TestGitSupportedActions(t *testing.T) {
	g := NewGitExecutor(t.TempDir())
	actions := g.SupportedActions()
	assert.Len(t, actions, 8)
}

func TestGitToolSchemas(t *testing.T) {
	g := NewGitExecutor(t.TempDir())
	schemas := g.ToolSchemas()
	assert.Len(t, schemas, 8)
	for _, s := range schemas {
		assert.NotEmpty(t, s.Name)
		assert.NotEmpty(t, s.Description)
	}
}
