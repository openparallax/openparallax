package executors

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/openparallax/openparallax/internal/crypto"
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
		types.ActionCopyDir, types.ActionMoveDir, types.ActionDeleteDir,
	}
}

// ToolSchemas returns tool definitions for all file and directory operations.
func (f *FileExecutor) ToolSchemas() []ToolSchema {
	pathParam := map[string]any{"type": "string", "description": "File path relative to workspace, or absolute. Supports ~ for home directory."}
	return []ToolSchema{
		{ActionType: types.ActionReadFile, Name: "read_file", Description: "Read the contents of a file. Use when the user asks to see, read, show, or check a file.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": pathParam}, "required": []string{"path"}}},
		{ActionType: types.ActionWriteFile, Name: "write_file", Description: "Create or overwrite a file with the given content. Use when the user asks to create, write, save, or update a file.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": pathParam, "content": map[string]any{"type": "string", "description": "The content to write to the file."}}, "required": []string{"path", "content"}}},
		{ActionType: types.ActionDeleteFile, Name: "delete_file", Description: "Delete a file. Use when the user asks to remove or delete a specific file.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": pathParam}, "required": []string{"path"}}},
		{ActionType: types.ActionMoveFile, Name: "move_file", Description: "Move or rename a file.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"source": map[string]any{"type": "string", "description": "Source file path."}, "destination": map[string]any{"type": "string", "description": "Destination file path."}}, "required": []string{"source", "destination"}}},
		{ActionType: types.ActionCopyFile, Name: "copy_file", Description: "Copy a file to a new location.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"source": map[string]any{"type": "string", "description": "Source file path."}, "destination": map[string]any{"type": "string", "description": "Destination file path."}}, "required": []string{"source", "destination"}}},
		{ActionType: types.ActionCreateDir, Name: "create_directory", Description: "Create a directory (including parent directories).", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": pathParam}, "required": []string{"path"}}},
		{ActionType: types.ActionListDir, Name: "list_directory", Description: "List the contents of a directory with file names and sizes. Set recursive to true to list all nested contents.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": pathParam, "recursive": map[string]any{"type": "boolean", "description": "If true, list contents recursively including subdirectories. Defaults to false."}}, "required": []string{"path"}}},
		{ActionType: types.ActionSearchFiles, Name: "search_files", Description: "Search for files matching a glob pattern within a directory.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": pathParam, "pattern": map[string]any{"type": "string", "description": "Glob pattern to match (e.g., *.go, *.md)."}}, "required": []string{"path"}}},
		{ActionType: types.ActionCopyDir, Name: "copy_directory", Description: "Copy an entire directory recursively to a new location. Use for copying folders with all their contents.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"source": map[string]any{"type": "string", "description": "Source directory path."}, "destination": map[string]any{"type": "string", "description": "Destination directory path."}}, "required": []string{"source", "destination"}}},
		{ActionType: types.ActionMoveDir, Name: "move_directory", Description: "Move or rename an entire directory.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"source": map[string]any{"type": "string", "description": "Source directory path."}, "destination": map[string]any{"type": "string", "description": "Destination directory path."}}, "required": []string{"source", "destination"}}},
		{ActionType: types.ActionDeleteDir, Name: "delete_directory", Description: "Delete a directory and all its contents recursively. Use with caution.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": pathParam}, "required": []string{"path"}}},
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
	case types.ActionCopyDir:
		return f.copyDir(action)
	case types.ActionMoveDir:
		return f.moveDir(action)
	case types.ActionDeleteDir:
		return f.deleteDir(action)
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
	recursive, _ := action.Payload["recursive"].(bool)

	if recursive {
		return f.listDirRecursive(action.RequestID, path)
	}

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

func (f *FileExecutor) listDirRecursive(requestID, root string) *types.ActionResult {
	var lines []string
	count := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if count >= 500 {
			return filepath.SkipAll
		}
		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			return nil
		}
		info, _ := d.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		prefix := "  "
		if d.IsDir() {
			prefix = "d "
		}
		lines = append(lines, fmt.Sprintf("%s%-40s %d bytes", prefix, rel, size))
		count++
		return nil
	})
	if err != nil {
		return &types.ActionResult{RequestID: requestID, Success: false, Error: err.Error(), Summary: "recursive list failed"}
	}

	output := strings.Join(lines, "\n")
	return &types.ActionResult{
		RequestID: requestID, Success: true,
		Output: output, Summary: fmt.Sprintf("listed %d entries recursively in %s", count, filepath.Base(root)),
		Artifact: &types.Artifact{
			ID: crypto.NewID(), Type: "command_output", Title: fmt.Sprintf("ls -R %s", filepath.Base(root)),
			Content: output, SizeBytes: int64(len(output)), PreviewType: "terminal",
		},
	}
}

func (f *FileExecutor) copyDir(action *types.ActionRequest) *types.ActionResult {
	src := f.resolvePath(action.Payload["source"])
	dst := f.resolvePath(action.Payload["destination"])

	info, err := os.Stat(src)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "copy_directory failed: source not found"}
	}
	if !info.IsDir() {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "source is not a directory", Summary: "copy_directory failed: not a directory"}
	}

	count := 0
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		count++
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "copy_directory failed"}
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  fmt.Sprintf("copied %d files from %s to %s", count, src, dst),
		Summary: fmt.Sprintf("copied directory %s (%d files)", filepath.Base(src), count),
	}
}

func (f *FileExecutor) moveDir(action *types.ActionRequest) *types.ActionResult {
	src := f.resolvePath(action.Payload["source"])
	dst := f.resolvePath(action.Payload["destination"])

	info, err := os.Stat(src)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "move_directory failed: source not found"}
	}
	if !info.IsDir() {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "source is not a directory", Summary: "move_directory failed: not a directory"}
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "move_directory failed"}
	}
	if err := os.Rename(src, dst); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "move_directory failed"}
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  fmt.Sprintf("moved directory %s to %s", src, dst),
		Summary: fmt.Sprintf("moved directory %s to %s", filepath.Base(src), filepath.Base(dst)),
	}
}

func (f *FileExecutor) deleteDir(action *types.ActionRequest) *types.ActionResult {
	path := f.resolvePath(action.Payload["path"])

	info, err := os.Stat(path)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "delete_directory failed: not found"}
	}
	if !info.IsDir() {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "path is not a directory", Summary: "delete_directory failed: not a directory"}
	}

	if err := os.RemoveAll(path); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "delete_directory failed"}
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  fmt.Sprintf("deleted directory %s", path),
		Summary: fmt.Sprintf("deleted directory %s", filepath.Base(path)),
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

func (f *FileExecutor) resolvePath(raw any) string {
	return ResolvePath(raw, f.workspacePath)
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
