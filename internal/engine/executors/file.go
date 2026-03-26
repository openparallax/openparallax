package executors

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/platform"
	"github.com/openparallax/openparallax/internal/types"
)

// FileExecutor handles file system operations.
type FileExecutor struct {
	workspacePath string
}

// NewFileExecutor creates a FileExecutor for the given workspace.
func NewFileExecutor(workspace string) *FileExecutor {
	return &FileExecutor{workspacePath: workspace}
}

// SupportedActions returns the file operation action types.
func (f *FileExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{
		types.ActionReadFile, types.ActionWriteFile, types.ActionDeleteFile,
		types.ActionMoveFile, types.ActionCopyFile, types.ActionCreateDir,
		types.ActionListDir, types.ActionSearchFiles,
	}
}

// Execute dispatches to the appropriate file operation.
func (f *FileExecutor) Execute(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	switch action.Type {
	case types.ActionReadFile:
		return f.readFile(action)
	case types.ActionWriteFile:
		return f.writeFile(action)
	case types.ActionDeleteFile:
		return f.deleteFile(action)
	case types.ActionMoveFile:
		return f.moveFile(action)
	case types.ActionCopyFile:
		return f.copyFile(action)
	case types.ActionCreateDir:
		return f.createDir(action)
	case types.ActionListDir:
		return f.listDir(action)
	case types.ActionSearchFiles:
		return f.searchFiles(action)
	default:
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "unknown file action"}
	}
}

func (f *FileExecutor) readFile(action *types.ActionRequest) *types.ActionResult {
	path := f.resolvePath(action.Payload["path"])

	data, err := os.ReadFile(path)
	if err != nil {
		return &types.ActionResult{
			RequestID: action.RequestID, Success: false,
			Error: err.Error(), Summary: fmt.Sprintf("failed to read %s", filepath.Base(path)),
		}
	}

	content := string(data)
	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output: content, Summary: fmt.Sprintf("read %s (%d bytes)", filepath.Base(path), len(data)),
		Artifact: &types.Artifact{
			ID: crypto.NewID(), Type: "file", Title: filepath.Base(path),
			Path: path, Content: content, Language: detectLanguage(path),
			SizeBytes: int64(len(data)), PreviewType: detectPreviewType(path),
		},
	}
}

func (f *FileExecutor) writeFile(action *types.ActionRequest) *types.ActionResult {
	path := f.resolvePath(action.Payload["path"])
	content, _ := action.Payload["content"].(string)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return &types.ActionResult{
			RequestID: action.RequestID, Success: false,
			Error: err.Error(), Summary: fmt.Sprintf("failed to create directory for %s", filepath.Base(path)),
		}
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return &types.ActionResult{
			RequestID: action.RequestID, Success: false,
			Error: err.Error(), Summary: fmt.Sprintf("failed to write %s", filepath.Base(path)),
		}
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:       fmt.Sprintf("wrote %d bytes to %s", len(content), path),
		Summary:      fmt.Sprintf("wrote %s (%d bytes)", filepath.Base(path), len(content)),
		BytesWritten: int64(len(content)),
		Artifact: &types.Artifact{
			ID: crypto.NewID(), Type: "file", Title: filepath.Base(path),
			Path: path, Content: content, Language: detectLanguage(path),
			SizeBytes: int64(len(content)), PreviewType: detectPreviewType(path),
		},
	}
}

func (f *FileExecutor) deleteFile(action *types.ActionRequest) *types.ActionResult {
	path := f.resolvePath(action.Payload["path"])
	if err := os.Remove(path); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "delete failed"}
	}
	return &types.ActionResult{RequestID: action.RequestID, Success: true, Summary: fmt.Sprintf("deleted %s", filepath.Base(path))}
}

func (f *FileExecutor) moveFile(action *types.ActionRequest) *types.ActionResult {
	src := f.resolvePath(action.Payload["source"])
	dst := f.resolvePath(action.Payload["destination"])
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "move failed"}
	}
	if err := os.Rename(src, dst); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "move failed"}
	}
	return &types.ActionResult{RequestID: action.RequestID, Success: true, Summary: fmt.Sprintf("moved to %s", filepath.Base(dst))}
}

func (f *FileExecutor) copyFile(action *types.ActionRequest) *types.ActionResult {
	src := f.resolvePath(action.Payload["source"])
	dst := f.resolvePath(action.Payload["destination"])
	data, err := os.ReadFile(src)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "copy failed: read error"}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "copy failed"}
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "copy failed: write error"}
	}
	return &types.ActionResult{RequestID: action.RequestID, Success: true, Summary: fmt.Sprintf("copied to %s", filepath.Base(dst))}
}

func (f *FileExecutor) createDir(action *types.ActionRequest) *types.ActionResult {
	path := f.resolvePath(action.Payload["path"])
	if err := os.MkdirAll(path, 0o755); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "mkdir failed"}
	}
	return &types.ActionResult{RequestID: action.RequestID, Success: true, Summary: fmt.Sprintf("created directory %s", filepath.Base(path))}
}

func (f *FileExecutor) listDir(action *types.ActionRequest) *types.ActionResult {
	path := f.resolvePath(action.Payload["path"])
	entries, err := os.ReadDir(path)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "list failed"}
	}

	var lines []string
	for _, e := range entries {
		info, _ := e.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		prefix := "  "
		if e.IsDir() {
			prefix = "d "
		}
		lines = append(lines, fmt.Sprintf("%s%-30s %d bytes", prefix, e.Name(), size))
	}

	output := strings.Join(lines, "\n")
	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output: output, Summary: fmt.Sprintf("listed %d entries in %s", len(entries), filepath.Base(path)),
		Artifact: &types.Artifact{
			ID: crypto.NewID(), Type: "command_output", Title: fmt.Sprintf("ls %s", filepath.Base(path)),
			Content: output, SizeBytes: int64(len(output)), PreviewType: "terminal",
		},
	}
}

func (f *FileExecutor) searchFiles(action *types.ActionRequest) *types.ActionResult {
	root := f.resolvePath(action.Payload["path"])
	pattern, _ := action.Payload["pattern"].(string)
	if pattern == "" {
		pattern = "*"
	}

	var matches []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		matched, _ := filepath.Match(pattern, d.Name())
		if matched {
			rel, _ := filepath.Rel(root, path)
			matches = append(matches, rel)
		}
		if len(matches) >= 100 {
			return filepath.SkipAll
		}
		return nil
	})

	output := strings.Join(matches, "\n")
	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output: output, Summary: fmt.Sprintf("found %d files matching '%s'", len(matches), pattern),
	}
}

// resolvePath resolves a path relative to the workspace, with tilde expansion.
func (f *FileExecutor) resolvePath(raw any) string {
	path, _ := raw.(string)
	path = platform.NormalizePath(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(f.workspacePath, path)
	}
	return filepath.Clean(path)
}

// detectLanguage returns the programming language for syntax highlighting.
func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	m := map[string]string{
		".go": "go", ".ts": "typescript", ".tsx": "typescript", ".js": "javascript",
		".py": "python", ".rs": "rust", ".sh": "bash", ".bash": "bash",
		".json": "json", ".yaml": "yaml", ".yml": "yaml", ".toml": "toml",
		".html": "html", ".htm": "html", ".css": "css", ".sql": "sql",
		".md": "markdown", ".xml": "xml", ".svelte": "svelte",
	}
	if l, ok := m[ext]; ok {
		return l
	}
	return "text"
}

// detectPreviewType determines how the frontend should render this file.
func detectPreviewType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html", ".htm":
		return "html"
	case ".md", ".markdown":
		return "markdown"
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp":
		return "image"
	default:
		return "code"
	}
}
