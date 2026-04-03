package executors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/internal/generation"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/types"
)

// VideoExecutor handles video generation via configured providers.
type VideoExecutor struct {
	provider  generation.VideoProvider
	workspace string
	log       *logging.Logger
}

// NewVideoExecutor creates a video executor. Returns nil if no provider is configured.
func NewVideoExecutor(cfg types.GenProviderConfig, workspace string, log *logging.Logger) *VideoExecutor {
	if cfg.Provider == "" || cfg.Provider == "none" {
		return nil
	}

	provider, err := generation.NewVideoProvider(cfg)
	if err != nil {
		if log != nil {
			log.Warn("video_provider_failed", "error", err)
		}
		return nil
	}
	if provider == nil {
		return nil
	}

	if log != nil {
		log.Info("video_provider_registered", "provider", cfg.Provider, "model", provider.ModelID())
	}
	return &VideoExecutor{provider: provider, workspace: workspace, log: log}
}

// SupportedActions returns video action types.
func (e *VideoExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{types.ActionGenerateVideo}
}

// ToolSchemas returns tool definitions for the LLM.
func (e *VideoExecutor) ToolSchemas() []ToolSchema {
	return []ToolSchema{
		{
			ActionType:  types.ActionGenerateVideo,
			Name:        "generate_video",
			Description: "Generate a short video from a text description. Returns the file path of the saved video.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"prompt":     map[string]any{"type": "string", "description": "Detailed description of the video to generate."},
					"duration":   map[string]any{"type": "integer", "description": "Video duration in seconds (5-20).", "default": 10},
					"resolution": map[string]any{"type": "string", "description": "Resolution: 480p, 720p, or 1080p.", "default": "720p"},
					"filename":   map[string]any{"type": "string", "description": "Output filename. Auto-generated if empty."},
				},
				"required": []string{"prompt"},
			},
		},
	}
}

// Execute dispatches video actions.
func (e *VideoExecutor) Execute(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	if action.Type != types.ActionGenerateVideo {
		return ErrorResult(action.RequestID, "unknown video action", "unknown action")
	}
	return e.generate(ctx, action)
}

func (e *VideoExecutor) generate(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	prompt, _ := action.Payload["prompt"].(string)
	if prompt == "" {
		return ErrorResult(action.RequestID, "prompt is required", "missing prompt")
	}

	duration := 10
	if d, ok := action.Payload["duration"].(float64); ok {
		duration = int(d)
	}
	if duration < 5 {
		duration = 5
	}
	if duration > 20 {
		duration = 20
	}

	resolution := getStringOr(action.Payload, "resolution", "720p")
	filename := getStringOr(action.Payload, "filename", "")

	result, err := e.provider.Generate(ctx, generation.VideoRequest{
		Prompt:     prompt,
		Duration:   duration,
		Resolution: resolution,
	})
	if err != nil {
		return ErrorResult(action.RequestID, "video generation failed: "+err.Error(), "generation failed")
	}

	if filename == "" {
		filename = fmt.Sprintf("generated-%s.%s", crypto.NewID()[:8], result.Format)
	}

	outPath := filepath.Join(e.workspace, filename)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return ErrorResult(action.RequestID, "failed to create directory: "+err.Error(), "save failed")
	}
	if err := os.WriteFile(outPath, result.Data, 0o644); err != nil {
		return ErrorResult(action.RequestID, "failed to save video: "+err.Error(), "save failed")
	}

	if e.log != nil {
		e.log.Info("generation_call", "type", "video", "provider", e.provider.ModelID(),
			"prompt_length", len(prompt), "duration", duration, "resolution", resolution,
			"output_bytes", len(result.Data), "output_path", filename, "success", true)
	}

	return &types.ActionResult{
		RequestID: action.RequestID,
		Success:   true,
		Output:    outPath,
		Summary:   fmt.Sprintf("Generated %ds video saved to %s (%s)", duration, filename, resolution),
		Artifact: &types.Artifact{
			ID:          crypto.NewID(),
			Type:        "video",
			Title:       filename,
			Path:        outPath,
			PreviewType: "video",
			SizeBytes:   int64(len(result.Data)),
		},
	}
}
