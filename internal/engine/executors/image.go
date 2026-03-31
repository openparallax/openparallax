package executors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/generation"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/types"
)

// ImageExecutor handles image generation and editing via configured providers.
type ImageExecutor struct {
	provider  generation.ImageProvider
	workspace string
	log       *logging.Logger
}

// NewImageExecutor creates an image executor. Returns nil if no provider is configured.
func NewImageExecutor(cfg types.GenProviderConfig, workspace string, log *logging.Logger) *ImageExecutor {
	if cfg.Provider == "" || cfg.Provider == "none" {
		return nil
	}

	provider, err := generation.NewImageProvider(cfg)
	if err != nil {
		if log != nil {
			log.Warn("image_provider_failed", "error", err)
		}
		return nil
	}
	if provider == nil {
		return nil
	}

	if log != nil {
		log.Info("image_provider_registered", "provider", cfg.Provider, "model", provider.ModelID())
	}
	return &ImageExecutor{provider: provider, workspace: workspace, log: log}
}

// SupportedActions returns image action types.
func (e *ImageExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{types.ActionGenerateImage, types.ActionEditImage}
}

// ToolSchemas returns tool definitions for the LLM.
func (e *ImageExecutor) ToolSchemas() []ToolSchema {
	return []ToolSchema{
		{
			ActionType:  types.ActionGenerateImage,
			Name:        "generate_image",
			Description: "Generate an image from a text description. Returns the file path of the saved image.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"prompt":   map[string]any{"type": "string", "description": "Detailed description of the image to generate."},
					"size":     map[string]any{"type": "string", "description": "Image dimensions: 1024x1024, 1792x1024, or 1024x1792.", "default": "1024x1024"},
					"style":    map[string]any{"type": "string", "description": "Style preset: natural or vivid."},
					"quality":  map[string]any{"type": "string", "description": "Quality: standard or hd.", "default": "standard"},
					"filename": map[string]any{"type": "string", "description": "Output filename. Auto-generated if empty."},
				},
				"required": []string{"prompt"},
			},
		},
		{
			ActionType:  types.ActionEditImage,
			Name:        "edit_image",
			Description: "Edit an existing image based on a text description.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":      map[string]any{"type": "string", "description": "Path to the source image in the workspace."},
					"prompt":    map[string]any{"type": "string", "description": "Description of the changes to make."},
					"mask_path": map[string]any{"type": "string", "description": "Optional path to a mask image defining the edit region."},
				},
				"required": []string{"path", "prompt"},
			},
		},
	}
}

// Execute dispatches image actions.
func (e *ImageExecutor) Execute(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	switch action.Type {
	case types.ActionGenerateImage:
		return e.generate(ctx, action)
	case types.ActionEditImage:
		return e.edit(ctx, action)
	default:
		return ErrorResult(action.RequestID, "unknown image action", "unknown action")
	}
}

func (e *ImageExecutor) generate(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	prompt, _ := action.Payload["prompt"].(string)
	if prompt == "" {
		return ErrorResult(action.RequestID, "prompt is required", "missing prompt")
	}

	size := getStringOr(action.Payload, "size", "1024x1024")
	style := getStringOr(action.Payload, "style", "")
	quality := getStringOr(action.Payload, "quality", "standard")
	filename := getStringOr(action.Payload, "filename", "")

	result, err := e.provider.Generate(ctx, generation.ImageRequest{
		Prompt:  prompt,
		Size:    size,
		Style:   style,
		Quality: quality,
		N:       1,
	})
	if err != nil {
		return ErrorResult(action.RequestID, "image generation failed: "+err.Error(), "generation failed")
	}

	if filename == "" {
		filename = fmt.Sprintf("generated-%s.%s", crypto.NewID()[:8], result.Format)
	}

	outPath := filepath.Join(e.workspace, filename)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return ErrorResult(action.RequestID, "failed to create directory: "+err.Error(), "save failed")
	}
	if err := os.WriteFile(outPath, result.Data, 0o644); err != nil {
		return ErrorResult(action.RequestID, "failed to save image: "+err.Error(), "save failed")
	}

	if e.log != nil {
		e.log.Info("generation_call", "type", "image", "provider", e.provider.ModelID(),
			"prompt_length", len(prompt), "size", size, "output_bytes", len(result.Data),
			"output_path", filename, "success", true)
	}

	return &types.ActionResult{
		RequestID: action.RequestID,
		Success:   true,
		Output:    outPath,
		Summary:   fmt.Sprintf("Generated image saved to %s (%s, %d bytes)", filename, size, len(result.Data)),
		Artifact: &types.Artifact{
			ID:          crypto.NewID(),
			Type:        "image",
			Title:       filename,
			Path:        outPath,
			PreviewType: "image",
			SizeBytes:   int64(len(result.Data)),
		},
	}
}

func (e *ImageExecutor) edit(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	sourcePath, _ := action.Payload["path"].(string)
	prompt, _ := action.Payload["prompt"].(string)
	maskPath := getStringOr(action.Payload, "mask_path", "")

	if sourcePath == "" || prompt == "" {
		return ErrorResult(action.RequestID, "path and prompt are required", "missing params")
	}

	// Resolve relative to workspace.
	if !filepath.IsAbs(sourcePath) {
		sourcePath = filepath.Join(e.workspace, sourcePath)
	}

	result, err := e.provider.Edit(ctx, generation.ImageEditRequest{
		SourcePath: sourcePath,
		Prompt:     prompt,
		MaskPath:   maskPath,
	})
	if err != nil {
		return ErrorResult(action.RequestID, "image edit failed: "+err.Error(), "edit failed")
	}

	outPath := sourcePath // overwrite source by default
	if err := os.WriteFile(outPath, result.Data, 0o644); err != nil {
		return ErrorResult(action.RequestID, "failed to save edited image: "+err.Error(), "save failed")
	}

	return &types.ActionResult{
		RequestID: action.RequestID,
		Success:   true,
		Output:    outPath,
		Summary:   fmt.Sprintf("Edited image saved to %s (%d bytes)", filepath.Base(outPath), len(result.Data)),
	}
}

func getStringOr(m map[string]any, key, defaultVal string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return defaultVal
}
