//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
)

// MockLLMServer implements an OpenAI-compatible chat completions API for
// deterministic E2E testing. It matches user messages against registered
// patterns and returns canned responses as SSE streams.
type MockLLMServer struct {
	server   *http.Server
	listener net.Listener
	port     int
	patterns []mockPattern
	callID   atomic.Int64
	mu       sync.Mutex
}

type mockPattern struct {
	match    func(messages []chatMessage, hasTools bool) bool
	response MockResponse
}

// MockResponse defines what the mock returns for a matched pattern.
type MockResponse struct {
	Text      string          // Plain text response
	ToolCalls []MockToolCall  // Tool calls to return
}

// MockToolCall defines a single tool call in a mock response.
type MockToolCall struct {
	Name      string
	Arguments map[string]any
}

// chatMessage is a minimal representation of an OpenAI chat message.
type chatMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// chatRequest is the incoming OpenAI-compatible request.
type chatRequest struct {
	Messages []chatMessage `json:"messages"`
	Tools    []any         `json:"tools,omitempty"`
	Stream   bool          `json:"stream"`
}

// NewMockLLMServer starts a mock OpenAI-compatible server on a random port.
func NewMockLLMServer() (*MockLLMServer, error) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("mock llm listen: %w", err)
	}

	m := &MockLLMServer{
		listener: lis,
		port:     lis.Addr().(*net.TCPAddr).Port,
	}
	m.registerDefaults()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chat/completions", m.handleCompletions)
	mux.HandleFunc("GET /v1/models", m.handleModels)

	m.server = &http.Server{Handler: mux}
	go func() { _ = m.server.Serve(lis) }()

	return m, nil
}

// Port returns the port the mock is listening on.
func (m *MockLLMServer) Port() int { return m.port }

// BaseURL returns the full base URL for config.
func (m *MockLLMServer) BaseURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d/v1", m.port)
}

// Stop shuts down the mock server.
func (m *MockLLMServer) Stop() {
	_ = m.server.Close()
}

func (m *MockLLMServer) handleModels(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"data":[{"id":"mock-v1","object":"model"}]}`))
}

func (m *MockLLMServer) handleCompletions(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := m.matchResponse(req.Messages, len(req.Tools) > 0)

	if !req.Stream {
		m.writeNonStreaming(w, resp)
		return
	}

	m.writeSSE(w, resp)
}

func (m *MockLLMServer) matchResponse(messages []chatMessage, hasTools bool) MockResponse {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.patterns {
		if p.match(messages, hasTools) {
			return p.response
		}
	}
	return MockResponse{Text: "I understand. How can I help?"}
}

func (m *MockLLMServer) nextCallID() string {
	return fmt.Sprintf("call_mock_%d", m.callID.Add(1))
}

func (m *MockLLMServer) writeSSE(w http.ResponseWriter, resp MockResponse) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)

	if len(resp.ToolCalls) > 0 {
		for _, tc := range resp.ToolCalls {
			argsJSON, _ := json.Marshal(tc.Arguments)
			callID := m.nextCallID()
			chunk := fmt.Sprintf(`{"id":"mock","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"%s","type":"function","function":{"name":"%s","arguments":"%s"}}]},"finish_reason":"tool_calls"}]}`,
				callID, tc.Name, strings.ReplaceAll(string(argsJSON), `"`, `\"`))
			fmt.Fprintf(w, "data: %s\n\n", chunk)
			if flusher != nil {
				flusher.Flush()
			}
		}
	} else {
		// Stream text in small chunks for realism.
		words := strings.Fields(resp.Text)
		for i, word := range words {
			sep := " "
			if i == 0 {
				sep = ""
			}
			chunk := fmt.Sprintf(`{"id":"mock","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"%s%s"},"finish_reason":null}]}`, sep, word)
			fmt.Fprintf(w, "data: %s\n\n", chunk)
			if flusher != nil {
				flusher.Flush()
			}
		}
		// Final chunk with finish_reason.
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"mock","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)
	}

	// Send a usage chunk (OpenAI sends this when stream_options.include_usage is set).
	usageChunk := `{"id":"mock","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":50,"completion_tokens":10,"total_tokens":60}}`
	fmt.Fprintf(w, "data: %s\n\n", usageChunk)
	if flusher != nil {
		flusher.Flush()
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func (m *MockLLMServer) writeNonStreaming(w http.ResponseWriter, resp MockResponse) {
	w.Header().Set("Content-Type", "application/json")
	result := map[string]any{
		"id":      "mock",
		"object":  "chat.completion",
		"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": resp.Text}, "finish_reason": "stop"}},
		"usage":   map[string]int{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
	}
	_ = json.NewEncoder(w).Encode(result)
}

// lastUserMessage extracts the content of the last user message.
func lastUserMessage(messages []chatMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

// hasToolResult returns true if the last message is a tool result.
func hasToolResult(messages []chatMessage) bool {
	if len(messages) == 0 {
		return false
	}
	return messages[len(messages)-1].Role == "tool"
}
