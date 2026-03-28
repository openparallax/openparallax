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

	"github.com/openparallax/openparallax/internal/crypto"
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
	return []types.ActionType{types.ActionCanvasCreate, types.ActionCanvasUpdate}
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
			ActionType:  types.ActionCanvasCreate,
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
			ActionType:  types.ActionCanvasCreate,
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

func (c *CanvasExecutor) Execute(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	// Route by tool name since canvas_project and canvas_preview share ActionCanvasCreate.
	toolName := string(action.Type)
	if name, ok := action.Payload["_tool_name"].(string); ok {
		toolName = name
	}

	switch toolName {
	case "canvas_project":
		return c.createProject(action)
	case "canvas_preview":
		return c.startPreview(action)
	case "canvas_update":
		return c.update(action)
	default:
		return c.create(action)
	}
}

func (c *CanvasExecutor) create(action *types.ActionRequest) *types.ActionResult {
	path := c.resolvePath(action.Payload["path"])
	content, _ := action.Payload["content"].(string)
	contentType, _ := action.Payload["type"].(string)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "canvas create failed"}
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "canvas create failed"}
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

func (c *CanvasExecutor) update(action *types.ActionRequest) *types.ActionResult {
	path := c.resolvePath(action.Payload["path"])
	content, _ := action.Payload["content"].(string)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "canvas update failed"}
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:       fmt.Sprintf("Updated %s (%d bytes)", filepath.Base(path), len(content)),
		Summary:      fmt.Sprintf("updated %s", filepath.Base(path)),
		BytesWritten: int64(len(content)),
	}
}

func (c *CanvasExecutor) createProject(action *types.ActionRequest) *types.ActionResult {
	dir := c.resolvePath(action.Payload["path"])
	filesRaw, ok := action.Payload["files"].([]any)
	if !ok || len(filesRaw) == 0 {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "files array is required", Summary: "canvas project failed"}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "canvas project failed"}
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

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  fmt.Sprintf("Created project with %d files:\n%s", len(created), strings.Join(created, "\n")),
		Summary: fmt.Sprintf("created project %s (%d files)", filepath.Base(dir), len(created)),
	}
}

func (c *CanvasExecutor) startPreview(action *types.ActionRequest) *types.ActionResult {
	servePath := c.resolvePath(action.Payload["path"])

	info, err := os.Stat(servePath)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "path not found: " + err.Error(), Summary: "preview failed"}
	}

	if !info.IsDir() {
		servePath = filepath.Dir(servePath)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "failed to find free port: " + err.Error(), Summary: "preview failed"}
	}
	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "failed to get port", Summary: "preview failed"}
	}
	port := tcpAddr.Port

	server := &http.Server{Handler: http.FileServer(http.Dir(servePath))}
	go func() { _ = server.Serve(listener) }()

	go func() {
		time.Sleep(30 * time.Minute)
		_ = server.Close()
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  fmt.Sprintf("Preview server running at %s (auto-closes in 30 minutes)", url),
		Summary: fmt.Sprintf("preview at %s", url),
	}
}

func (c *CanvasExecutor) resolvePath(raw any) string {
	path, _ := raw.(string)
	if !filepath.IsAbs(path) {
		path = filepath.Join(c.workspacePath, path)
	}
	return filepath.Clean(path)
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
