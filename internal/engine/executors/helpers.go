package executors

import (
	"path/filepath"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/platform"
)

// ResolvePath resolves a raw payload value to an absolute path using
// the workspace as the base for relative paths. Handles tilde expansion
// via platform.NormalizePath.
func ResolvePath(raw any, workspacePath string) string {
	path, _ := raw.(string)
	path = platform.NormalizePath(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(workspacePath, path)
	}
	return filepath.Clean(path)
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
