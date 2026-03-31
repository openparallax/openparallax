package generation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GoogleImageProvider generates images via the Google AI Imagen API.
type GoogleImageProvider struct {
	apiKey string
	model  string
	client *http.Client
}

// NewGoogleImageProvider creates a Google image provider.
func NewGoogleImageProvider(apiKey, model string) *GoogleImageProvider {
	return &GoogleImageProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

// ModelID returns the model string.
func (p *GoogleImageProvider) ModelID() string { return p.model }

// Generate creates an image from a text prompt via Google Imagen.
func (p *GoogleImageProvider) Generate(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1/models/%s:predict?key=%s",
		p.model, p.apiKey)

	body := map[string]any{
		"instances": []map[string]string{
			{"prompt": req.Prompt},
		},
		"parameters": map[string]any{
			"sampleCount": 1,
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

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

	var result struct {
		Predictions []struct {
			BytesBase64Encoded string `json:"bytesBase64Encoded"`
			MimeType           string `json:"mimeType"`
		} `json:"predictions"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(result.Predictions) == 0 {
		return nil, fmt.Errorf("no images returned")
	}

	imgData, err := base64.StdEncoding.DecodeString(result.Predictions[0].BytesBase64Encoded)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	format := "png"
	if result.Predictions[0].MimeType == "image/jpeg" {
		format = "jpg"
	}

	return &ImageResult{Data: imgData, Format: format}, nil
}

// Edit is not supported by Google Imagen.
func (p *GoogleImageProvider) Edit(_ context.Context, _ ImageEditRequest) (*ImageResult, error) {
	return nil, ErrUnsupported
}
