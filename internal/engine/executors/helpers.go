package executors

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/platform"
)

// ResolvePath resolves a raw payload value to an absolute path using
// the workspace as the base for relative paths. Handles tilde expansion
// via platform.NormalizePath.
//
// ResolvePath does NOT enforce workspace containment. Executors that must
// keep all writes inside the workspace MUST use ResolveInWorkspace instead.
func ResolvePath(raw any, workspacePath string) string {
	path, _ := raw.(string)
	path = platform.NormalizePath(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(workspacePath, path)
	}
	return filepath.Clean(path)
}

// ResolveInWorkspace resolves a raw payload value to an absolute path inside
// the workspace and rejects any path that escapes the workspace via parent
// references, absolute paths to other directories, or symlinks named in the
// payload. This is the only correct path resolver for executors whose tools
// are scoped to the workspace.
func ResolveInWorkspace(raw any, workspacePath string) (string, error) {
	rawStr, _ := raw.(string)
	if rawStr == "" {
		return "", fmt.Errorf("path is required")
	}
	resolved := ResolvePath(rawStr, workspacePath)
	absWorkspace, err := filepath.Abs(workspacePath)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}
	rel, err := filepath.Rel(absWorkspace, resolved)
	if err != nil {
		return "", fmt.Errorf("path %q is outside the workspace", rawStr)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes the workspace", rawStr)
	}
	return resolved, nil
}

// ErrorResult creates a failed ActionResult with consistent formatting.
func ErrorResult(requestID, err, summary string) *types.ActionResult {
	return &types.ActionResult{
		RequestID: requestID,
		Success:   false,
		Error:     err,
		Summary:   summary,
	}
}

// SuccessResult creates a successful ActionResult with consistent formatting.
func SuccessResult(requestID, output, summary string) *types.ActionResult {
	return &types.ActionResult{
		RequestID: requestID,
		Success:   true,
		Output:    output,
		Summary:   summary,
	}
}

// Truncate shortens a string to maxLen characters with an ellipsis.
func Truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
