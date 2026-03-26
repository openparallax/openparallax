package tier1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/openparallax/openparallax/internal/types"
)

// OnnxClient is the interface for the ONNX classifier sidecar.
type OnnxClient interface {
	// Classify sends the action to the ONNX sidecar for classification.
	Classify(ctx context.Context, action *types.ActionRequest) (*ClassifierResult, error)
	// IsAvailable returns true if the sidecar is reachable.
	IsAvailable() bool
}

// HTTPOnnxClient communicates with the ONNX classifier sidecar via HTTP.
type HTTPOnnxClient struct {
	baseURL    string
	httpClient *http.Client
	available  bool
}

// NewHTTPOnnxClient creates a client for the ONNX sidecar at the given address.
func NewHTTPOnnxClient(addr string) *HTTPOnnxClient {
	client := &HTTPOnnxClient{
		baseURL: fmt.Sprintf("http://%s", addr),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
	client.available = client.ping()
	return client
}

// Classify sends the action to the ONNX sidecar for classification.
func (c *HTTPOnnxClient) Classify(ctx context.Context, action *types.ActionRequest) (*ClassifierResult, error) {
	if !c.available {
		return nil, fmt.Errorf("ONNX classifier sidecar not available")
	}

	text := fmt.Sprintf("%s: %v", action.Type, action.Payload)

	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/classify", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.available = false
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("classifier returned %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		Label      string  `json:"label"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	decision := types.VerdictAllow
	if result.Label == "INJECTION" && result.Confidence >= 0.85 {
		decision = types.VerdictBlock
	}

	return &ClassifierResult{
		Decision:   decision,
		Confidence: result.Confidence,
		Reason:     fmt.Sprintf("ONNX: %s (%.2f)", result.Label, result.Confidence),
		Source:     "onnx",
	}, nil
}

// IsAvailable returns true if the sidecar was reachable at initialization.
func (c *HTTPOnnxClient) IsAvailable() bool { return c.available }

func (c *HTTPOnnxClient) ping() bool {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
