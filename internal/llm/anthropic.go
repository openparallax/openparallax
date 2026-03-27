package llm

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
)

// AnthropicProvider implements Provider for Anthropic's Claude models.
type AnthropicProvider struct {
	client *anthropic.Client
	model  string
}

// NewAnthropicProvider creates an Anthropic provider with the given API key and model.
func NewAnthropicProvider(apiKey, model string) (*AnthropicProvider, error) {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &AnthropicProvider{client: &client, model: model}, nil
}

// Complete sends a prompt and returns the full response.
func (a *AnthropicProvider) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
	cfg := applyOptions(opts)

	params := anthropic.MessageNewParams{
		Model:     a.model,
		MaxTokens: int64(cfg.MaxTokens),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	}
	if cfg.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: cfg.SystemPrompt}}
	}
	if cfg.Temperature > 0 {
		params.Temperature = anthropic.Float(cfg.Temperature)
	}

	msg, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return "", err
	}

	return extractAnthropicText(msg), nil
}

// CompleteWithHistory sends a conversation and returns the full response.
func (a *AnthropicProvider) CompleteWithHistory(ctx context.Context, messages []ChatMessage, opts ...Option) (string, error) {
	cfg := applyOptions(opts)

	params := anthropic.MessageNewParams{
		Model:     a.model,
		MaxTokens: int64(cfg.MaxTokens),
		Messages:  toAnthropicMessages(messages),
	}
	if cfg.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: cfg.SystemPrompt}}
	}
	if cfg.Temperature > 0 {
		params.Temperature = anthropic.Float(cfg.Temperature)
	}

	msg, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return "", err
	}

	return extractAnthropicText(msg), nil
}

// Stream sends a prompt and returns a StreamReader.
func (a *AnthropicProvider) Stream(ctx context.Context, prompt string, opts ...Option) (StreamReader, error) {
	cfg := applyOptions(opts)

	params := anthropic.MessageNewParams{
		Model:     a.model,
		MaxTokens: int64(cfg.MaxTokens),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	}
	if cfg.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: cfg.SystemPrompt}}
	}
	if cfg.Temperature > 0 {
		params.Temperature = anthropic.Float(cfg.Temperature)
	}

	stream := a.client.Messages.NewStreaming(ctx, params)
	return &anthropicStreamReader{stream: stream}, nil
}

// StreamWithHistory sends a conversation and returns a StreamReader.
func (a *AnthropicProvider) StreamWithHistory(ctx context.Context, messages []ChatMessage, opts ...Option) (StreamReader, error) {
	cfg := applyOptions(opts)

	params := anthropic.MessageNewParams{
		Model:     a.model,
		MaxTokens: int64(cfg.MaxTokens),
		Messages:  toAnthropicMessages(messages),
	}
	if cfg.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: cfg.SystemPrompt}}
	}
	if cfg.Temperature > 0 {
		params.Temperature = anthropic.Float(cfg.Temperature)
	}

	stream := a.client.Messages.NewStreaming(ctx, params)
	return &anthropicStreamReader{stream: stream}, nil
}

// StreamWithTools sends a conversation with tool definitions.
func (a *AnthropicProvider) StreamWithTools(ctx context.Context, messages []ChatMessage, tools []ToolDefinition, opts ...Option) (ToolStreamReader, error) {
	return nil, fmt.Errorf("StreamWithTools not yet implemented for Anthropic provider")
}

// EstimateTokens returns a rough token estimate (1 token per 4 characters).
func (a *AnthropicProvider) EstimateTokens(text string) int { return len(text) / 4 }

// Name returns "anthropic".
func (a *AnthropicProvider) Name() string { return "anthropic" }

// Model returns the model name.
func (a *AnthropicProvider) Model() string { return a.model }

// anthropicStreamReader wraps the Anthropic streaming API.
type anthropicStreamReader struct {
	stream *ssestream.Stream[anthropic.MessageStreamEventUnion]
	mu     sync.Mutex
	buf    string
}

// Next returns the next text delta from the stream.
func (r *anthropicStreamReader) Next() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for r.stream.Next() {
		event := r.stream.Current()
		if event.Type == "content_block_delta" {
			delta := event.AsContentBlockDelta()
			if delta.Delta.Type == "text_delta" {
				text := delta.Delta.AsTextDelta().Text
				r.buf += text
				return text, nil
			}
		}
	}

	if err := r.stream.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}

// Close releases stream resources.
func (r *anthropicStreamReader) Close() error { return r.stream.Close() }

// FullText returns all accumulated text.
func (r *anthropicStreamReader) FullText() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf
}

// extractAnthropicText extracts text from a completed Anthropic message.
func extractAnthropicText(msg *anthropic.Message) string {
	var text string
	for _, block := range msg.Content {
		if block.Type == "text" {
			text += block.AsText().Text
		}
	}
	return text
}

// toAnthropicMessages converts ChatMessage slice to Anthropic message params.
func toAnthropicMessages(msgs []ChatMessage) []anthropic.MessageParam {
	result := make([]anthropic.MessageParam, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case "user":
			result = append(result, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		case "assistant":
			result = append(result, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		}
	}
	return result
}
