package generation

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// StabilityImageProvider generates images via the Stability AI REST API.
type StabilityImageProvider struct {
	apiKey string
	client *http.Client
}

// NewStabilityImageProvider creates a Stability AI image provider.
func NewStabilityImageProvider(apiKey string) *StabilityImageProvider {
	return &StabilityImageProvider{
		apiKey: apiKey,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

// ModelID returns the model identifier.
func (p *StabilityImageProvider) ModelID() string { return "sd3" }

// Generate creates an image via the Stability AI SD3 API.
func (p *StabilityImageProvider) Generate(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	_ = writer.WriteField("prompt", req.Prompt)
	_ = writer.WriteField("output_format", "png")

	if req.Style != "" {
		_ = writer.WriteField("style_preset", req.Style)
	}

	_ = writer.Close()

	url := "https://api.stability.ai/v2beta/stable-image/generate/sd3"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, &body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Accept", "image/*")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, truncateErr(respBody))
	}

	return &ImageResult{Data: respBody, Format: "png"}, nil
}

// Edit is not supported by Stability AI in this implementation.
func (p *StabilityImageProvider) Edit(_ context.Context, _ ImageEditRequest) (*ImageResult, error) {
	return nil, ErrUnsupported
}
