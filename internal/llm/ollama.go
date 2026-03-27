package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// OllamaProvider implements Provider for Ollama local models.
// It communicates with Ollama's HTTP API directly — no SDK required.
type OllamaProvider struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewOllamaProvider creates an Ollama provider. BaseURL defaults to http://localhost:11434.
func NewOllamaProvider(baseURL, model string) (*OllamaProvider, error) {
	return &OllamaProvider{
		baseURL:    baseURL,
		model:      model,
		httpClient: &http.Client{},
	}, nil
}

// ollamaChatRequest is the request body for /api/chat.
type ollamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []ollamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
	Options  *ollamaOptions      `json:"options,omitempty"`
}

type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// ollamaChatResponse is a single response line from /api/chat.
type ollamaChatResponse struct {
	Message ollamaChatMessage `json:"message"`
	Done    bool              `json:"done"`
}

// Complete sends a prompt and returns the full response.
func (o *OllamaProvider) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
	cfg := applyOptions(opts)
	messages := []ollamaChatMessage{{Role: "user", Content: prompt}}
	if cfg.SystemPrompt != "" {
		messages = append([]ollamaChatMessage{{Role: "system", Content: cfg.SystemPrompt}}, messages...)
	}
	return o.doChat(ctx, messages, cfg)
}

// CompleteWithHistory sends a conversation and returns the full response.
func (o *OllamaProvider) CompleteWithHistory(ctx context.Context, messages []ChatMessage, opts ...Option) (string, error) {
	cfg := applyOptions(opts)
	oMsgs := toOllamaMessages(messages)
	if cfg.SystemPrompt != "" {
		oMsgs = append([]ollamaChatMessage{{Role: "system", Content: cfg.SystemPrompt}}, oMsgs...)
	}
	return o.doChat(ctx, oMsgs, cfg)
}

// Stream sends a prompt and returns a StreamReader.
func (o *OllamaProvider) Stream(ctx context.Context, prompt string, opts ...Option) (StreamReader, error) {
	cfg := applyOptions(opts)
	messages := []ollamaChatMessage{{Role: "user", Content: prompt}}
	if cfg.SystemPrompt != "" {
		messages = append([]ollamaChatMessage{{Role: "system", Content: cfg.SystemPrompt}}, messages...)
	}
	return o.doStreamChat(ctx, messages, cfg)
}

// StreamWithHistory sends a conversation and returns a StreamReader.
func (o *OllamaProvider) StreamWithHistory(ctx context.Context, messages []ChatMessage, opts ...Option) (StreamReader, error) {
	cfg := applyOptions(opts)
	oMsgs := toOllamaMessages(messages)
	if cfg.SystemPrompt != "" {
		oMsgs = append([]ollamaChatMessage{{Role: "system", Content: cfg.SystemPrompt}}, oMsgs...)
	}
	return o.doStreamChat(ctx, oMsgs, cfg)
}

// StreamWithTools sends a conversation with tool definitions.
func (o *OllamaProvider) StreamWithTools(ctx context.Context, messages []ChatMessage, tools []ToolDefinition, opts ...Option) (ToolStreamReader, error) {
	return nil, fmt.Errorf("StreamWithTools not yet implemented for Ollama provider")
}

// EstimateTokens returns a rough token estimate (1 token per 4 characters).
func (o *OllamaProvider) EstimateTokens(text string) int { return len(text) / 4 }

// Name returns "ollama".
func (o *OllamaProvider) Name() string { return "ollama" }

// Model returns the model name.
func (o *OllamaProvider) Model() string { return o.model }

// doChat performs a non-streaming chat request and returns the full response.
func (o *OllamaProvider) doChat(ctx context.Context, messages []ollamaChatMessage, cfg *CompletionConfig) (string, error) {
	reqBody := ollamaChatRequest{
		Model:    o.model,
		Messages: messages,
		Stream:   false,
		Options:  &ollamaOptions{Temperature: cfg.Temperature, NumPredict: cfg.MaxTokens},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, respBody)
	}

	var chatResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode ollama response: %w", err)
	}

	return chatResp.Message.Content, nil
}

// doStreamChat performs a streaming chat request and returns a StreamReader.
func (o *OllamaProvider) doStreamChat(ctx context.Context, messages []ollamaChatMessage, cfg *CompletionConfig) (StreamReader, error) {
	reqBody := ollamaChatRequest{
		Model:    o.model,
		Messages: messages,
		Stream:   true,
		Options:  &ollamaOptions{Temperature: cfg.Temperature, NumPredict: cfg.MaxTokens},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama stream request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, respBody)
	}

	return &ollamaStreamReader{
		body:    resp.Body,
		scanner: bufio.NewScanner(resp.Body),
	}, nil
}

// ollamaStreamReader reads newline-delimited JSON from an Ollama streaming response.
type ollamaStreamReader struct {
	body    io.ReadCloser
	scanner *bufio.Scanner
	mu      sync.Mutex
	buf     string
}

// Next returns the next text chunk from the stream.
func (r *ollamaStreamReader) Next() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for r.scanner.Scan() {
		line := r.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var resp ollamaChatResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}

		if resp.Done {
			return "", io.EOF
		}

		text := resp.Message.Content
		if text != "" {
			r.buf += text
			return text, nil
		}
	}

	if err := r.scanner.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}

// Close releases stream resources.
func (r *ollamaStreamReader) Close() error { return r.body.Close() }

// FullText returns all accumulated text.
func (r *ollamaStreamReader) FullText() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf
}

// toOllamaMessages converts ChatMessage slice to Ollama message format.
func toOllamaMessages(msgs []ChatMessage) []ollamaChatMessage {
	result := make([]ollamaChatMessage, len(msgs))
	for i, m := range msgs {
		result[i] = ollamaChatMessage{Role: m.Role, Content: m.Content}
	}
	return result
}
