//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RESTClient provides helpers for calling the OpenParallax REST API.
type RESTClient struct {
	baseURL string
	client  *http.Client
}

// NewRESTClient creates a client pointing at the given base URL.
func NewRESTClient(baseURL string) *RESTClient {
	return &RESTClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Get performs a GET request and returns the body, status code, and error.
func (r *RESTClient) Get(path string) ([]byte, int, error) {
	resp, err := r.client.Get(r.baseURL + path)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

// Post performs a POST request with a JSON body.
func (r *RESTClient) Post(path string, payload any) ([]byte, int, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}
	resp, err := r.client.Post(r.baseURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

// Delete performs a DELETE request.
func (r *RESTClient) Delete(path string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodDelete, r.baseURL+path, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

// GetJSON performs a GET and unmarshals the response into dest.
func (r *RESTClient) GetJSON(path string, dest any) (int, error) {
	body, code, err := r.Get(path)
	if err != nil {
		return 0, err
	}
	if dest != nil && len(body) > 0 {
		if jsonErr := json.Unmarshal(body, dest); jsonErr != nil {
			return code, fmt.Errorf("unmarshal %s: %w (body: %s)", path, jsonErr, string(body))
		}
	}
	return code, nil
}

// WaitForReady polls the status endpoint until the engine and agent are
// both up. The agent is considered connected when the sandbox status
// reports active (canary probes run in the agent process).
func (r *RESTClient) WaitForReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		body, code, err := r.Get("/api/status")
		if err != nil || code != 200 {
			time.Sleep(250 * time.Millisecond)
			continue
		}
		// Check if agent is connected by verifying sandbox is active.
		var status map[string]any
		if json.Unmarshal(body, &status) != nil {
			time.Sleep(250 * time.Millisecond)
			continue
		}
		if sb, ok := status["sandbox"].(map[string]any); ok {
			if active, _ := sb["active"].(bool); active {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("engine/agent not ready after %s", timeout)
}
