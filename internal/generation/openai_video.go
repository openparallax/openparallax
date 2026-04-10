package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIVideoProvider generates videos via the OpenAI Videos API (Sora).
type OpenAIVideoProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewOpenAIVideoProvider creates an OpenAI video provider.
func NewOpenAIVideoProvider(apiKey, model, baseURL string) *OpenAIVideoProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIVideoProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 300 * time.Second},
	}
}

// ModelID returns the model string.
func (p *OpenAIVideoProvider) ModelID() string { return p.model }

// Generate creates a video by submitting a generation job and polling for completion.
func (p *OpenAIVideoProvider) Generate(ctx context.Context, req VideoRequest) (*VideoResult, error) {
	duration := req.Duration
	if duration < 5 {
		duration = 5
	}
	if duration > 20 {
		duration = 20
	}
	resolution := req.Resolution
	if resolution == "" {
		resolution = "720p"
	}

	// Submit generation job.
	body := map[string]any{
		"model":      p.model,
		"prompt":     req.Prompt,
		"duration":   duration,
		"resolution": resolution,
	}
	if req.Style != "" {
		body["style"] = req.Style
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/videos/generations", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("submit video job: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, truncateErr(respBody))
	}

	var submitResult struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &submitResult); err != nil {
		return nil, fmt.Errorf("parse submit response: %w", err)
	}

	if submitResult.ID == "" {
		return nil, fmt.Errorf("no video job ID returned")
	}

	// Poll for completion.
	return p.pollForResult(ctx, submitResult.ID)
}

func (p *OpenAIVideoProvider) pollForResult(ctx context.Context, jobID string) (*VideoResult, error) {
	pollURL := fmt.Sprintf("%s/videos/%s", p.baseURL, jobID)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}

		httpReq, err := http.NewRequestWithContext(ctx, "GET", pollURL, nil)
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

		resp, err := p.client.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("poll request: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read poll response: %w", err)
		}

		var status struct {
			Status string `json:"status"`
			URL    string `json:"url"`
			Error  string `json:"error"`
		}
		if err := json.Unmarshal(respBody, &status); err != nil {
			return nil, fmt.Errorf("parse poll response: %w", err)
		}

		switch status.Status {
		case "completed":
			if status.URL == "" {
				return nil, fmt.Errorf("video completed but no download URL")
			}
			return p.downloadVideo(ctx, status.URL)
		case "failed":
			return nil, fmt.Errorf("video generation failed: %s", status.Error)
		case "queued", "processing":
			continue
		default:
			continue
		}
	}
}

func (p *OpenAIVideoProvider) downloadVideo(ctx context.Context, url string) (*VideoResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("download video: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read video: %w", err)
	}

	return &VideoResult{Data: data, Format: "mp4"}, nil
}
