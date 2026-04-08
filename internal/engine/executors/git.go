package executors

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/openparallax/openparallax/internal/types"
)

const gitTimeout = 30 * time.Second

// GitExecutor handles git operations via the system git binary.
type GitExecutor struct {
	workspacePath string
}

// NewGitExecutor creates a git executor.
func NewGitExecutor(workspace string) *GitExecutor {
	return &GitExecutor{workspacePath: workspace}
}

// WorkspaceScope reports that git operations are confined to the workspace.
func (g *GitExecutor) WorkspaceScope() WorkspaceScope { return ScopeScoped }

func (g *GitExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{
		types.ActionGitStatus, types.ActionGitDiff, types.ActionGitLog,
		types.ActionGitCommit, types.ActionGitPush, types.ActionGitPull,
		types.ActionGitBranch, types.ActionGitCheckout,
	}
}

func (g *GitExecutor) ToolSchemas() []ToolSchema {
	pathDesc := "Repository path. Defaults to workspace."
	return []ToolSchema{
		{ActionType: types.ActionGitStatus, Name: "git_status", Description: "Show the working tree status of a git repository — modified, staged, and untracked files.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": pathDesc}}}},
		{ActionType: types.ActionGitDiff, Name: "git_diff", Description: "Show changes in the working tree or between commits.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": pathDesc}, "staged": map[string]any{"type": "boolean", "description": "Show staged changes only."}, "commit": map[string]any{"type": "string", "description": "Compare against a specific commit."}}}},
		{ActionType: types.ActionGitLog, Name: "git_log", Description: "Show recent commit history.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": pathDesc}, "limit": map[string]any{"type": "integer", "description": "Number of commits to show. Default 10."}, "branch": map[string]any{"type": "string", "description": "Branch to show log for."}}}},
		{ActionType: types.ActionGitCommit, Name: "git_commit", Description: "Stage and commit changes. By default stages all modified files.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": pathDesc}, "message": map[string]any{"type": "string", "description": "Commit message."}, "files": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Specific files to stage. Omit to stage all."}}, "required": []string{"message"}}},
		{ActionType: types.ActionGitPush, Name: "git_push", Description: "Push commits to a remote repository.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": pathDesc}, "remote": map[string]any{"type": "string", "description": "Remote name. Default 'origin'."}, "branch": map[string]any{"type": "string", "description": "Branch to push."}}}},
		{ActionType: types.ActionGitPull, Name: "git_pull", Description: "Pull changes from a remote repository.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": pathDesc}, "remote": map[string]any{"type": "string", "description": "Remote name. Default 'origin'."}, "branch": map[string]any{"type": "string", "description": "Branch to pull."}}}},
		{ActionType: types.ActionGitBranch, Name: "git_branch", Description: "List, create, or switch branches.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": pathDesc}, "action": map[string]any{"type": "string", "description": "Action: 'list', 'create', or 'switch'.", "enum": []string{"list", "create", "switch"}}, "name": map[string]any{"type": "string", "description": "Branch name (for create/switch)."}}, "required": []string{"action"}}},
		{ActionType: types.ActionGitCheckout, Name: "git_checkout", Description: "Check out a branch or commit.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": pathDesc}, "ref": map[string]any{"type": "string", "description": "Branch name or commit hash."}}, "required": []string{"ref"}}},
	}
}

func (g *GitExecutor) Execute(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	repoPath := g.resolveRepoPath(action.Payload)

	switch action.Type {
	case types.ActionGitStatus:
		return g.runGit(action.RequestID, repoPath, "status", "--short")
	case types.ActionGitDiff:
		return g.diff(action, repoPath)
	case types.ActionGitLog:
		return g.log(action, repoPath)
	case types.ActionGitCommit:
		return g.commit(action, repoPath)
	case types.ActionGitPush:
		return g.push(action, repoPath)
	case types.ActionGitPull:
		return g.pull(action, repoPath)
	case types.ActionGitBranch:
		return g.branch(action, repoPath)
	case types.ActionGitCheckout:
		return g.checkout(action, repoPath)
	default:
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "unknown git action"}
	}
}

func (g *GitExecutor) diff(action *types.ActionRequest, repoPath string) *types.ActionResult {
	args := []string{"diff"}
	if staged, ok := action.Payload["staged"].(bool); ok && staged {
		args = append(args, "--cached")
	}
	if commit, ok := action.Payload["commit"].(string); ok && commit != "" {
		args = append(args, commit)
	}
	return g.runGit(action.RequestID, repoPath, args...)
}

func (g *GitExecutor) log(action *types.ActionRequest, repoPath string) *types.ActionResult {
	limit := 10
	if l, ok := action.Payload["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}
	args := []string{"log", "--oneline", fmt.Sprintf("-n%d", limit)}
	if branch, ok := action.Payload["branch"].(string); ok && branch != "" {
		args = append(args, branch)
	}
	return g.runGit(action.RequestID, repoPath, args...)
}

func (g *GitExecutor) commit(action *types.ActionRequest, repoPath string) *types.ActionResult {
	message, _ := action.Payload["message"].(string)
	if message == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "commit message is required"}
	}

	// Stage files. Use the `--` separator so any filename beginning with a
	// dash is treated as a path, not a flag, and reject empty entries up
	// front so the LLM can't smuggle a flag through an empty string.
	if files, ok := action.Payload["files"].([]any); ok && len(files) > 0 {
		fileArgs := make([]string, 0, len(files))
		for _, f := range files {
			fname, ok := f.(string)
			if !ok || fname == "" {
				return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "files entries must be non-empty strings"}
			}
			fileArgs = append(fileArgs, fname)
		}
		args := append([]string{"add", "--"}, fileArgs...)
		g.runGit(action.RequestID, repoPath, args...)
	} else {
		g.runGit(action.RequestID, repoPath, "add", "-A")
	}

	return g.runGit(action.RequestID, repoPath, "commit", "-m", message)
}

func (g *GitExecutor) push(action *types.ActionRequest, repoPath string) *types.ActionResult {
	remote := "origin"
	if r, ok := action.Payload["remote"].(string); ok && r != "" {
		remote = r
	}
	args := []string{"push", remote}
	if branch, ok := action.Payload["branch"].(string); ok && branch != "" {
		args = append(args, branch)
	}
	return g.runGit(action.RequestID, repoPath, args...)
}

func (g *GitExecutor) pull(action *types.ActionRequest, repoPath string) *types.ActionResult {
	remote := "origin"
	if r, ok := action.Payload["remote"].(string); ok && r != "" {
		remote = r
	}
	args := []string{"pull", remote}
	if branch, ok := action.Payload["branch"].(string); ok && branch != "" {
		args = append(args, branch)
	}
	return g.runGit(action.RequestID, repoPath, args...)
}

func (g *GitExecutor) branch(action *types.ActionRequest, repoPath string) *types.ActionResult {
	branchAction, _ := action.Payload["action"].(string)
	name, _ := action.Payload["name"].(string)

	switch branchAction {
	case "list":
		return g.runGit(action.RequestID, repoPath, "branch", "-a")
	case "create":
		if name == "" {
			return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "branch name is required"}
		}
		return g.runGit(action.RequestID, repoPath, "branch", name)
	case "switch":
		if name == "" {
			return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "branch name is required"}
		}
		return g.runGit(action.RequestID, repoPath, "checkout", name)
	default:
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "action must be 'list', 'create', or 'switch'"}
	}
}

func (g *GitExecutor) checkout(action *types.ActionRequest, repoPath string) *types.ActionResult {
	ref, _ := action.Payload["ref"].(string)
	if ref == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "ref is required"}
	}
	return g.runGit(action.RequestID, repoPath, "checkout", ref)
}

func (g *GitExecutor) runGit(requestID, repoPath string, args ...string) *types.ActionResult {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	if err != nil {
		return &types.ActionResult{
			RequestID: requestID, Success: false,
			Error: err.Error(), Output: output,
			Summary: fmt.Sprintf("git %s failed", strings.Join(args, " ")),
		}
	}

	return &types.ActionResult{
		RequestID: requestID, Success: true,
		Output:  output,
		Summary: fmt.Sprintf("git %s", strings.Join(args, " ")),
	}
}

func (g *GitExecutor) resolveRepoPath(payload map[string]any) string {
	if path, ok := payload["path"].(string); ok && path != "" {
		return ResolvePath(path, g.workspacePath)
	}
	return g.workspacePath
}
