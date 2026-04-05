package executors

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/internal/types"
)

// CanvasExecutor handles creative file generation, multi-file projects,
// and live preview serving.
type CanvasExecutor struct {
	workspacePath string
}

// NewCanvasExecutor creates a canvas executor.
func NewCanvasExecutor(workspace string) *CanvasExecutor {
	return &CanvasExecutor{workspacePath: workspace}
}

func (c *CanvasExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{
		types.ActionCanvasCreate, types.ActionCanvasUpdate,
		types.ActionCanvasProject, types.ActionCanvasPreview,
	}
}

func (c *CanvasExecutor) ToolSchemas() []ToolSchema {
	typeEnum := []string{"html", "svg", "markdown", "mermaid", "css", "javascript", "json", "yaml"}
	return []ToolSchema{
		{
			ActionType:  types.ActionCanvasCreate,
			Name:        "canvas_create",
			Description: "Create a file with typed content. Use for generating HTML pages, SVG graphics, Markdown documents, Mermaid diagrams, CSS stylesheets, JavaScript files, or data files.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":    map[string]any{"type": "string", "description": "File path relative to workspace."},
					"content": map[string]any{"type": "string", "description": "The file content."},
					"type":    map[string]any{"type": "string", "description": "Content type.", "enum": typeEnum},
				},
				"required": []string{"path", "content", "type"},
			},
		},
		{
			ActionType:  types.ActionCanvasUpdate,
			Name:        "canvas_update",
			Description: "Update an existing canvas file with new content (full replacement).",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":    map[string]any{"type": "string", "description": "File path relative to workspace."},
					"content": map[string]any{"type": "string", "description": "The new content."},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			ActionType:  types.ActionCanvasProject,
			Name:        "canvas_project",
			Description: "Create a multi-file project in a directory. Each file has a name, content, and type. Subdirectories are created automatically.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":        map[string]any{"type": "string", "description": "Directory path for the project."},
					"description": map[string]any{"type": "string", "description": "Brief description of the project."},
					"files": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"name":    map[string]any{"type": "string"},
								"content": map[string]any{"type": "string"},
								"type":    map[string]any{"type": "string"},
							},
						},
						"description": "Array of files to create in the project.",
					},
				},
				"required": []string{"path", "files"},
			},
		},
		{
			ActionType:  types.ActionCanvasPreview,
			Name:        "canvas_preview",
			Description: "Start a local preview server for HTML/CSS/JS files. Opens in the browser if available. Auto-closes after 30 minutes.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "File or directory to serve."},
				},
				"required": []string{"path"},
			},
		},
	}
}

func (c *CanvasExecutor) Execute(_ context.Context, action *types.ActionRequest) *types.ActionResult {
	switch action.Type {
	case types.ActionCanvasCreate:
		return c.createFile(action)
	case types.ActionCanvasUpdate:
		return c.updateFile(action)
	case types.ActionCanvasProject:
		return c.createProject(action)
	case types.ActionCanvasPreview:
		return c.startPreview(action)
	default:
		return ErrorResult(action.RequestID, "unknown canvas action", "unknown action")
	}
}

func (c *CanvasExecutor) createFile(action *types.ActionRequest) *types.ActionResult {
	rawPath, _ := action.Payload["path"].(string)
	contentType, _ := action.Payload["type"].(string)
	if rawPath == "" {
		// LLM omitted path — generate a default from the content type.
		ext := map[string]string{
			"html": ".html", "svg": ".svg", "markdown": ".md", "mermaid": ".mmd",
			"css": ".css", "javascript": ".js", "json": ".json", "yaml": ".yaml",
		}
		suffix := ext[contentType]
		if suffix == "" {
			suffix = ".txt"
		}
		rawPath = "canvas" + suffix
	}
	path := ResolvePath(rawPath, c.workspacePath)
	content, _ := action.Payload["content"].(string)
	if contentType == "" {
		contentType = inferTypeFromExt(rawPath)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ErrorResult(action.RequestID, err.Error(), "failed to create directory for "+filepath.Base(path))
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return ErrorResult(action.RequestID, err.Error(), "failed to write "+filepath.Base(path))
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:       fmt.Sprintf("Created %s (%s, %d bytes)", filepath.Base(path), contentType, len(content)),
		Summary:      fmt.Sprintf("created %s (%s)", filepath.Base(path), contentType),
		BytesWritten: int64(len(content)),
		Artifact: &types.Artifact{
			ID: crypto.NewID(), Type: "file", Title: filepath.Base(path),
			Path: path, Content: content, Language: contentType,
			SizeBytes: int64(len(content)), PreviewType: detectCanvasPreview(contentType),
		},
	}
}

func (c *CanvasExecutor) updateFile(action *types.ActionRequest) *types.ActionResult {
	rawPath, _ := action.Payload["path"].(string)
	if rawPath == "" {
		return ErrorResult(action.RequestID, "path is required", "canvas_update requires a file path")
	}
	path := ResolvePath(rawPath, c.workspacePath)
	content, _ := action.Payload["content"].(string)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return ErrorResult(action.RequestID, err.Error(), "failed to update "+filepath.Base(path))
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:       fmt.Sprintf("Updated %s (%d bytes)", filepath.Base(path), len(content)),
		Summary:      fmt.Sprintf("updated %s", filepath.Base(path)),
		BytesWritten: int64(len(content)),
	}
}

func (c *CanvasExecutor) createProject(action *types.ActionRequest) *types.ActionResult {
	rawPath, _ := action.Payload["path"].(string)
	if rawPath == "" {
		return ErrorResult(action.RequestID, "path is required", "canvas_project requires a directory path")
	}
	dir := ResolvePath(rawPath, c.workspacePath)
	filesRaw, ok := action.Payload["files"].([]any)
	if !ok || len(filesRaw) == 0 {
		return ErrorResult(action.RequestID, "files array is required", "project creation failed")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ErrorResult(action.RequestID, err.Error(), "failed to create project directory")
	}

	var created []string
	for _, f := range filesRaw {
		file, ok := f.(map[string]any)
		if !ok {
			continue
		}
		name, _ := file["name"].(string)
		content, _ := file["content"].(string)
		if name == "" {
			continue
		}

		filePath := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			continue
		}
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			continue
		}
		created = append(created, name)
	}

	return SuccessResult(action.RequestID,
		fmt.Sprintf("Created project with %d files:\n%s", len(created), strings.Join(created, "\n")),
		fmt.Sprintf("created project %s (%d files)", filepath.Base(dir), len(created)))
}

func (c *CanvasExecutor) startPreview(action *types.ActionRequest) *types.ActionResult {
	servePath := ResolvePath(action.Payload["path"], c.workspacePath)

	info, err := os.Stat(servePath)
	if err != nil {
		return ErrorResult(action.RequestID, "path not found: "+err.Error(), "preview failed")
	}

	if !info.IsDir() {
		servePath = filepath.Dir(servePath)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return ErrorResult(action.RequestID, "failed to find free port: "+err.Error(), "preview failed")
	}
	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return ErrorResult(action.RequestID, "failed to get port", "preview failed")
	}
	port := tcpAddr.Port

	server := &http.Server{Handler: http.FileServer(http.Dir(servePath))}
	go func() { _ = server.Serve(listener) }()

	go func() {
		time.Sleep(30 * time.Minute)
		_ = server.Close()
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	return SuccessResult(action.RequestID,
		fmt.Sprintf("Preview server running at %s (auto-closes in 30 minutes)", url),
		fmt.Sprintf("preview at %s", url))
}

func inferTypeFromExt(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".html", ".htm":
		return "html"
	case ".md", ".markdown":
		return "markdown"
	case ".svg":
		return "svg"
	case ".css":
		return "css"
	case ".js":
		return "javascript"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".mmd":
		return "mermaid"
	default:
		return ""
	}
}

func detectCanvasPreview(contentType string) string {
	switch contentType {
	case "html":
		return "html"
	case "markdown":
		return "markdown"
	case "svg":
		return "image"
	default:
		return "code"
	}
}
