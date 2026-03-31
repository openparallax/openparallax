package generation

import (
	"context"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestNewImageProviderNone(t *testing.T) {
	p, err := NewImageProvider(types.GenProviderConfig{Provider: "none"})
	assert.NoError(t, err)
	assert.Nil(t, p)
}

func TestNewImageProviderEmpty(t *testing.T) {
	p, err := NewImageProvider(types.GenProviderConfig{})
	assert.NoError(t, err)
	assert.Nil(t, p)
}

func TestNewImageProviderUnsupported(t *testing.T) {
	_, err := NewImageProvider(types.GenProviderConfig{Provider: "nonexistent"})
	assert.Error(t, err)
}

func TestNewVideoProviderNone(t *testing.T) {
	p, err := NewVideoProvider(types.GenProviderConfig{Provider: "none"})
	assert.NoError(t, err)
	assert.Nil(t, p)
}

func TestNewVideoProviderUnsupported(t *testing.T) {
	_, err := NewVideoProvider(types.GenProviderConfig{Provider: "nonexistent"})
	assert.Error(t, err)
}

func TestOpenAIImageProviderModelID(t *testing.T) {
	p := NewOpenAIImageProvider("test-key", "gpt-image-1", "")
	assert.Equal(t, "gpt-image-1", p.ModelID())
}

func TestOpenAIVideoProviderModelID(t *testing.T) {
	p := NewOpenAIVideoProvider("test-key", "sora-2", "")
	assert.Equal(t, "sora-2", p.ModelID())
}

func TestGoogleImageProviderModelID(t *testing.T) {
	p := NewGoogleImageProvider("test-key", "imagen-3.0-generate-002")
	assert.Equal(t, "imagen-3.0-generate-002", p.ModelID())
}

func TestStabilityImageProviderModelID(t *testing.T) {
	p := NewStabilityImageProvider("test-key")
	assert.Equal(t, "sd3", p.ModelID())
}

func TestStabilityEditUnsupported(t *testing.T) {
	p := NewStabilityImageProvider("test-key")
	_, err := p.Edit(context.Background(), ImageEditRequest{})
	assert.ErrorIs(t, err, ErrUnsupported)
}

func TestGoogleEditUnsupported(t *testing.T) {
	p := NewGoogleImageProvider("test-key", "imagen-3.0-generate-002")
	_, err := p.Edit(context.Background(), ImageEditRequest{})
	assert.ErrorIs(t, err, ErrUnsupported)
}

func TestImageRequestDefaults(t *testing.T) {
	req := ImageRequest{Prompt: "test"}
	assert.Equal(t, "test", req.Prompt)
	assert.Empty(t, req.Size)    // provider fills default
	assert.Empty(t, req.Quality) // provider fills default
}

func TestVideoRequestDuration(t *testing.T) {
	req := VideoRequest{Prompt: "test", Duration: 0}
	assert.Zero(t, req.Duration)
	assert.Equal(t, "test", req.Prompt)
}
