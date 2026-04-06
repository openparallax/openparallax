package executors

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/openparallax/openparallax/internal/types"
)

const (
	httpDefaultTimeout  = 30 * time.Second
	httpMaxResponseBody = 1 << 20 // 1MB
)

// HTTPExecutor handles HTTP requests.
type HTTPExecutor struct{}

// NewHTTPExecutor creates an HTTP executor.
func NewHTTPExecutor() *HTTPExecutor {
	return &HTTPExecutor{}
}

func (h *HTTPExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{types.ActionHTTPRequest}
}

func (h *HTTPExecutor) ToolSchemas() []ToolSchema {
	return []ToolSchema{
		{
			ActionType:  types.ActionHTTPRequest,
			Name:        "http_request",
			Description: "Make an HTTP request to a URL and return the response. Use for fetching web pages, calling APIs, or checking URLs. Supports GET, POST, PUT, DELETE, PATCH methods.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url":     map[string]any{"type": "string", "description": "The URL to request."},
					"method":  map[string]any{"type": "string", "description": "HTTP method: GET, POST, PUT, DELETE, PATCH. Defaults to GET.", "enum": []string{"GET", "POST", "PUT", "DELETE", "PATCH"}},
					"headers": map[string]any{"type": "object", "description": "Optional request headers as key-value pairs."},
					"body":    map[string]any{"type": "string", "description": "Optional request body for POST/PUT/PATCH."},
				},
				"required": []string{"url"},
			},
		},
	}
}

func (h *HTTPExecutor) Execute(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	url, _ := action.Payload["url"].(string)
	if url == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "url is required", Summary: "http request failed"}
	}

	if strings.HasPrefix(strings.ToLower(url), "http://") {
		return &types.ActionResult{
			RequestID: action.RequestID, Success: false,
			Error:   "insecure HTTP request blocked — use HTTPS for secure data transfer",
			Summary: "blocked: insecure HTTP",
		}
	}

	if err := validateURLNotPrivate(url); err != nil {
		return &types.ActionResult{
			RequestID: action.RequestID, Success: false,
			Error:   err.Error(),
			Summary: "blocked: private/internal address",
		}
	}

	method, _ := action.Payload["method"].(string)
	if method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)

	var bodyReader io.Reader
	if body, ok := action.Payload["body"].(string); ok && body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "http request failed"}
	}

	req.Header.Set("User-Agent", "OpenParallax/1.0")

	if headers, ok := action.Payload["headers"].(map[string]any); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, vs)
			}
		}
	}

	client := &http.Client{
		Timeout: httpDefaultTimeout,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: fmt.Sprintf("HTTP %s %s failed", method, url)}
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, httpMaxResponseBody+1))
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "failed to read response body"}
	}

	truncated := false
	if len(bodyBytes) > httpMaxResponseBody {
		bodyBytes = bodyBytes[:httpMaxResponseBody]
		truncated = true
	}

	body := string(bodyBytes)
	if truncated {
		body += "\n\n[Response truncated at 1MB]"
	}

	output := fmt.Sprintf("HTTP %d %s\nContent-Type: %s\n\n%s",
		resp.StatusCode, resp.Status, resp.Header.Get("Content-Type"), body)

	return &types.ActionResult{
		RequestID: action.RequestID,
		Success:   resp.StatusCode < 400,
		Output:    output,
		Summary:   fmt.Sprintf("HTTP %s %s → %d", method, url, resp.StatusCode),
	}
}
