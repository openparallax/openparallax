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

// OpenAIImageProvider generates images via the OpenAI Images API.
type OpenAIImageProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewOpenAIImageProvider creates an OpenAI image provider.
func NewOpenAIImageProvider(apiKey, model, baseURL string) *OpenAIImageProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIImageProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// ModelID returns the model string.
func (p *OpenAIImageProvider) ModelID() string { return p.model }

// Generate creates an image from a text prompt.
func (p *OpenAIImageProvider) Generate(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	size := req.Size
	if size == "" {
		size = "1024x1024"
	}
	quality := req.Quality
	if quality == "" {
		quality = "standard"
	}
	n := req.N
	if n <= 0 {
		n = 1
	}

	body := map[string]any{
		"model":           p.model,
		"prompt":          req.Prompt,
		"size":            size,
		"quality":         quality,
		"n":               n,
		"response_format": "b64_json",
	}
	if req.Style != "" {
		body["style"] = req.Style
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/images/generations", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

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
		Data []struct {
			B64JSON string `json:"b64_json"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no images returned")
	}

	imgData, err := base64.StdEncoding.DecodeString(result.Data[0].B64JSON)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	return &ImageResult{Data: imgData, Format: "png"}, nil
}

// Edit is not yet implemented for the OpenAI provider.
func (p *OpenAIImageProvider) Edit(_ context.Context, _ ImageEditRequest) (*ImageResult, error) {
	return nil, fmt.Errorf("image editing is not yet implemented for the openai provider: %w", ErrUnsupported)
}

func truncateErr(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
