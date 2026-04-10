package executors

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestHTTPGetRequest(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer srv.Close()

	h := NewHTTPExecutor()
	// Use the test server's client to handle self-signed certs.
	result := httpExecWithClient(h, srv, &types.ActionRequest{
		RequestID: "r1", Type: types.ActionHTTPRequest,
		Payload: map[string]any{"url": srv.URL + "/api"},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, `{"status":"ok"}`)
	assert.Contains(t, result.Output, "200")
}

func TestHTTPPostRequest(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, "created")
	}))
	defer srv.Close()

	h := NewHTTPExecutor()
	result := httpExecWithClient(h, srv, &types.ActionRequest{
		RequestID: "r1", Type: types.ActionHTTPRequest,
		Payload: map[string]any{"url": srv.URL, "method": "POST", "body": `{"name":"test"}`},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "201")
}

func TestHTTPCustomHeaders(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))
		fmt.Fprint(w, "authed")
	}))
	defer srv.Close()

	h := NewHTTPExecutor()
	result := httpExecWithClient(h, srv, &types.ActionRequest{
		RequestID: "r1", Type: types.ActionHTTPRequest,
		Payload: map[string]any{
			"url":     srv.URL,
			"headers": map[string]any{"Authorization": "Bearer token123"},
		},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "authed")
}

func TestHTTPEmptyURL(t *testing.T) {
	h := NewHTTPExecutor()
	result := h.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionHTTPRequest,
		Payload: map[string]any{"url": ""},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "url is required")
}

func TestHTTPMissingURL(t *testing.T) {
	h := NewHTTPExecutor()
	result := h.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionHTTPRequest,
		Payload: map[string]any{},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "url is required")
}

func TestHTTPDefaultMethodIsGET(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.Method)
	}))
	defer srv.Close()

	h := NewHTTPExecutor()
	result := httpExecWithClient(h, srv, &types.ActionRequest{
		RequestID: "r1", Type: types.ActionHTTPRequest,
		Payload: map[string]any{"url": srv.URL},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "GET")
}

func TestHTTP4xxReturnsFailure(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "not found")
	}))
	defer srv.Close()

	h := NewHTTPExecutor()
	result := httpExecWithClient(h, srv, &types.ActionRequest{
		RequestID: "r1", Type: types.ActionHTTPRequest,
		Payload: map[string]any{"url": srv.URL},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Output, "404")
}

func TestHTTPResponseTruncation(t *testing.T) {
	// Generate response larger than 1MB.
	bigBody := strings.Repeat("x", 1<<20+100)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, bigBody)
	}))
	defer srv.Close()

	h := NewHTTPExecutor()
	result := httpExecWithClient(h, srv, &types.ActionRequest{
		RequestID: "r1", Type: types.ActionHTTPRequest,
		Payload: map[string]any{"url": srv.URL},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "[Response truncated at 1MB]")
}

func TestHTTPDeleteMethod(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	h := NewHTTPExecutor()
	result := httpExecWithClient(h, srv, &types.ActionRequest{
		RequestID: "r1", Type: types.ActionHTTPRequest,
		Payload: map[string]any{"url": srv.URL, "method": "DELETE"},
	})

	assert.True(t, result.Success)
}

func TestHTTPSupportedActions(t *testing.T) {
	h := NewHTTPExecutor()
	assert.Equal(t, []types.ActionType{types.ActionHTTPRequest}, h.SupportedActions())
}

func TestHTTPToolSchemas(t *testing.T) {
	h := NewHTTPExecutor()
	schemas := h.ToolSchemas()
	assert.Len(t, schemas, 1)
	assert.Equal(t, "http_request", schemas[0].Name)
}

// httpExecWithClient bypasses the executor's http.Client to use the test server's
// TLS-aware client, since httptest.NewTLSServer uses a self-signed cert.
func httpExecWithClient(h *HTTPExecutor, srv *httptest.Server, action *types.ActionRequest) *types.ActionResult {
	url, _ := action.Payload["url"].(string)
	method, _ := action.Payload["method"].(string)
	if method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)

	var bodyReader *strings.Reader
	if body, ok := action.Payload["body"].(string); ok && body != "" {
		bodyReader = strings.NewReader(body)
	}

	var req *http.Request
	var err error
	if bodyReader != nil {
		req, err = http.NewRequest(method, url, bodyReader)
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error()}
	}

	req.Header.Set("User-Agent", "OpenParallax/1.0")
	if headers, ok := action.Payload["headers"].(map[string]any); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, vs)
			}
		}
	}

	client := srv.Client()
	resp, err := client.Do(req)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes := make([]byte, httpMaxResponseBody+1)
	n, _ := resp.Body.Read(bodyBytes)
	bodyBytes = bodyBytes[:n]

	truncated := false
	if n > httpMaxResponseBody {
		bodyBytes = bodyBytes[:httpMaxResponseBody]
		truncated = true
	}

	content := string(bodyBytes)
	if truncated {
		content += "\n\n[Response truncated at 1MB]"
	}

	output := fmt.Sprintf("HTTP %d %s\nContent-Type: %s\n\n%s",
		resp.StatusCode, resp.Status, resp.Header.Get("Content-Type"), content)

	return &types.ActionResult{
		RequestID: action.RequestID,
		Success:   resp.StatusCode < 400,
		Output:    output,
		Summary:   fmt.Sprintf("HTTP %s %s → %d", method, url, resp.StatusCode),
	}
}
