// Package generation provides image and video generation via external APIs.
// Supports OpenAI (DALL-E/GPT Image, Sora), Google (Imagen), and Stability AI.
package generation

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/openparallax/openparallax/internal/types"
)

// ErrUnsupported is returned when an operation is not supported by the provider.
var ErrUnsupported = errors.New("operation not supported by this provider")

// ImageProvider generates and edits images.
type ImageProvider interface {
	// Generate creates an image from a text prompt.
	Generate(ctx context.Context, req ImageRequest) (*ImageResult, error)

	// Edit modifies an existing image. Returns ErrUnsupported if not available.
	Edit(ctx context.Context, req ImageEditRequest) (*ImageResult, error)

	// ModelID returns the model identifier for logging.
	ModelID() string
}

// VideoProvider generates videos from text prompts.
type VideoProvider interface {
	// Generate creates a video. May involve async polling internally.
	Generate(ctx context.Context, req VideoRequest) (*VideoResult, error)

	// ModelID returns the model identifier for logging.
	ModelID() string
}

// ImageRequest specifies image generation parameters.
type ImageRequest struct {
	Prompt  string
	Size    string // "1024x1024", "1792x1024", "1024x1792"
	Style   string // provider-specific: "natural", "vivid"
	Quality string // "standard", "hd"
	N       int    // number of images (default 1)
}

// ImageEditRequest specifies image editing parameters.
type ImageEditRequest struct {
	SourcePath string
	Prompt     string
	MaskPath   string
}

// ImageResult holds generated image data.
type ImageResult struct {
	Data   []byte
	Format string // "png", "jpg", "webp"
}

// VideoRequest specifies video generation parameters.
type VideoRequest struct {
	Prompt     string
	Duration   int    // seconds: 5-20
	Resolution string // "480p", "720p", "1080p"
	Style      string
}

// VideoResult holds generated video data.
type VideoResult struct {
	Data   []byte
	Format string // "mp4"
}

// NewImageProvider creates an image provider from configuration.
func NewImageProvider(cfg types.GenProviderConfig) (ImageProvider, error) {
	apiKey := resolveKey(cfg.APIKeyEnv)

	switch cfg.Provider {
	case "openai":
		if apiKey == "" {
			return nil, fmt.Errorf("API key not set: %s", cfg.APIKeyEnv)
		}
		model := cfg.Model
		if model == "" {
			model = "gpt-image-1"
		}
		return NewOpenAIImageProvider(apiKey, model, cfg.BaseURL), nil
	case "google":
		if apiKey == "" {
			return nil, fmt.Errorf("API key not set: %s", cfg.APIKeyEnv)
		}
		model := cfg.Model
		if model == "" {
			model = "imagen-3.0-generate-002"
		}
		return NewGoogleImageProvider(apiKey, model), nil
	case "stability":
		if apiKey == "" {
			return nil, fmt.Errorf("API key not set: %s", cfg.APIKeyEnv)
		}
		return NewStabilityImageProvider(apiKey), nil
	case "", "none":
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported image provider: %s", cfg.Provider)
	}
}

// NewVideoProvider creates a video provider from configuration.
func NewVideoProvider(cfg types.GenProviderConfig) (VideoProvider, error) {
	apiKey := resolveKey(cfg.APIKeyEnv)

	switch cfg.Provider {
	case "openai":
		if apiKey == "" {
			return nil, fmt.Errorf("API key not set: %s", cfg.APIKeyEnv)
		}
		model := cfg.Model
		if model == "" {
			model = "sora-2"
		}
		return NewOpenAIVideoProvider(apiKey, model, cfg.BaseURL), nil
	case "", "none":
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported video provider: %s", cfg.Provider)
	}
}

func resolveKey(envName string) string {
	if envName == "" {
		return ""
	}
	return os.Getenv(envName)
}
