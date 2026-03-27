package llm

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/ssestream"
)

// OpenAIProvider implements Provider for OpenAI and OpenAI-compatible endpoints.
// Setting BaseURL enables DeepSeek, Mistral, LM Studio, and other compatible APIs.
type OpenAIProvider struct {
	client *openai.Client
	model  string
}

// NewOpenAIProvider creates an OpenAI provider. Pass an empty baseURL for the default
// OpenAI endpoint, or a custom URL for compatible providers.
func NewOpenAIProvider(apiKey, model, baseURL string) (*OpenAIProvider, error) {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := openai.NewClient(opts...)
	return &OpenAIProvider{client: &client, model: model}, nil
}

// Complete sends a prompt and returns the full response.
func (o *OpenAIProvider) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
	cfg := applyOptions(opts)
	params := o.buildParams(cfg, []ChatMessage{{Role: "user", Content: prompt}})

	resp, err := o.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", nil
	}
	return resp.Choices[0].Message.Content, nil
}

// CompleteWithHistory sends a conversation and returns the full response.
func (o *OpenAIProvider) CompleteWithHistory(ctx context.Context, messages []ChatMessage, opts ...Option) (string, error) {
	cfg := applyOptions(opts)
	params := o.buildParams(cfg, messages)

	resp, err := o.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", nil
	}
	return resp.Choices[0].Message.Content, nil
}

// Stream sends a prompt and returns a StreamReader.
func (o *OpenAIProvider) Stream(ctx context.Context, prompt string, opts ...Option) (StreamReader, error) {
	cfg := applyOptions(opts)
	params := o.buildParams(cfg, []ChatMessage{{Role: "user", Content: prompt}})

	stream := o.client.Chat.Completions.NewStreaming(ctx, params)
	return &openaiStreamReader{stream: stream}, nil
}

// StreamWithHistory sends a conversation and returns a StreamReader.
func (o *OpenAIProvider) StreamWithHistory(ctx context.Context, messages []ChatMessage, opts ...Option) (StreamReader, error) {
	cfg := applyOptions(opts)
	params := o.buildParams(cfg, messages)

	stream := o.client.Chat.Completions.NewStreaming(ctx, params)
	return &openaiStreamReader{stream: stream}, nil
}

// StreamWithTools sends a conversation with tool definitions.
// Full implementation in step 2 of the pipeline revamp.
func (o *OpenAIProvider) StreamWithTools(ctx context.Context, messages []ChatMessage, tools []ToolDefinition, opts ...Option) (ToolStreamReader, error) {
	return nil, fmt.Errorf("StreamWithTools not yet implemented for OpenAI provider")
}

// EstimateTokens returns a rough token estimate (1 token per 4 characters).
func (o *OpenAIProvider) EstimateTokens(text string) int { return len(text) / 4 }

// Name returns "openai".
func (o *OpenAIProvider) Name() string { return "openai" }

// Model returns the model name.
func (o *OpenAIProvider) Model() string { return o.model }

// buildParams constructs the ChatCompletionNewParams from config and messages.
func (o *OpenAIProvider) buildParams(cfg *CompletionConfig, messages []ChatMessage) openai.ChatCompletionNewParams {
	params := openai.ChatCompletionNewParams{
		Model:               o.model,
		MaxCompletionTokens: param.NewOpt(int64(cfg.MaxTokens)),
	}
	if cfg.Temperature > 0 {
		params.Temperature = param.NewOpt(cfg.Temperature)
	}

	var oaiMsgs []openai.ChatCompletionMessageParamUnion
	if cfg.SystemPrompt != "" {
		oaiMsgs = append(oaiMsgs, openai.SystemMessage(cfg.SystemPrompt))
	}
	for _, m := range messages {
		switch m.Role {
		case "system":
			oaiMsgs = append(oaiMsgs, openai.SystemMessage(m.Content))
		case "user":
			oaiMsgs = append(oaiMsgs, openai.UserMessage(m.Content))
		case "assistant":
			oaiMsgs = append(oaiMsgs, openai.AssistantMessage(m.Content))
		}
	}
	params.Messages = oaiMsgs
	return params
}

// openaiStreamReader wraps the OpenAI streaming API.
type openaiStreamReader struct {
	stream *ssestream.Stream[openai.ChatCompletionChunk]
	mu     sync.Mutex
	buf    string
}

// Next returns the next text delta from the stream.
func (r *openaiStreamReader) Next() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for r.stream.Next() {
		chunk := r.stream.Current()
		if len(chunk.Choices) > 0 {
			text := chunk.Choices[0].Delta.Content
			if text != "" {
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
func (r *openaiStreamReader) Close() error { return r.stream.Close() }

// FullText returns all accumulated text.
func (r *openaiStreamReader) FullText() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf
}
