package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/shared"
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
	params := o.buildParams(cfg, []ChatMessage{{Role: "user", Content: prompt}}, nil)

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
	params := o.buildParams(cfg, messages, nil)

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
	params := o.buildParams(cfg, []ChatMessage{{Role: "user", Content: prompt}}, nil)

	stream := o.client.Chat.Completions.NewStreaming(ctx, params)
	return &openaiStreamReader{stream: stream}, nil
}

// StreamWithHistory sends a conversation and returns a StreamReader.
func (o *OpenAIProvider) StreamWithHistory(ctx context.Context, messages []ChatMessage, opts ...Option) (StreamReader, error) {
	cfg := applyOptions(opts)
	params := o.buildParams(cfg, messages, nil)

	stream := o.client.Chat.Completions.NewStreaming(ctx, params)
	return &openaiStreamReader{stream: stream}, nil
}

// StreamWithTools sends a conversation with tool definitions and returns a
// ToolStreamReader. The LLM can respond with text, tool calls, or both.
func (o *OpenAIProvider) StreamWithTools(ctx context.Context, messages []ChatMessage, tools []ToolDefinition, opts ...Option) (ToolStreamReader, error) {
	cfg := applyOptions(opts)
	params := o.buildParams(cfg, messages, tools)

	stream := o.client.Chat.Completions.NewStreaming(ctx, params)
	return &openaiToolStreamReader{
		provider: o,
		ctx:      ctx,
		cfg:      cfg,
		messages: messages,
		tools:    tools,
		stream:   stream,
		accum:    make(map[int]*toolCallAccum),
	}, nil
}

// EstimateTokens returns a rough token estimate (1 token per 4 characters).
func (o *OpenAIProvider) EstimateTokens(text string) int { return len(text) / 4 }

// Name returns "openai".
func (o *OpenAIProvider) Name() string { return "openai" }

// Model returns the model name.
func (o *OpenAIProvider) Model() string { return o.model }

// CheapestModel returns the cheapest OpenAI model for sub-agent use.
func (o *OpenAIProvider) CheapestModel() string { return "gpt-5.4-mini" }

// buildParams constructs the ChatCompletionNewParams from config, messages, and optional tools.
func (o *OpenAIProvider) buildParams(cfg *CompletionConfig, messages []ChatMessage, tools []ToolDefinition) openai.ChatCompletionNewParams {
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
			if len(m.ToolCalls) > 0 {
				oaiMsgs = append(oaiMsgs, buildAssistantWithToolCalls(m))
			} else {
				oaiMsgs = append(oaiMsgs, openai.AssistantMessage(m.Content))
			}
		case "tool":
			oaiMsgs = append(oaiMsgs, openai.ToolMessage(m.Content, m.ToolCallID))
		}
	}
	params.Messages = oaiMsgs

	if len(tools) > 0 {
		var oaiTools []openai.ChatCompletionToolParam
		for _, t := range tools {
			oaiTools = append(oaiTools, openai.ChatCompletionToolParam{
				Function: shared.FunctionDefinitionParam{
					Name:        t.Name,
					Description: param.NewOpt(t.Description),
					Parameters:  shared.FunctionParameters(t.Parameters),
				},
			})
		}
		params.Tools = oaiTools
	}

	return params
}

// buildAssistantWithToolCalls creates an assistant message that includes tool call references.
func buildAssistantWithToolCalls(m ChatMessage) openai.ChatCompletionMessageParamUnion {
	var toolCalls []openai.ChatCompletionMessageToolCallParam
	for _, tc := range m.ToolCalls {
		argsBytes, _ := json.Marshal(tc.Arguments)
		toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallParam{
			ID: tc.ID,
			Function: openai.ChatCompletionMessageToolCallFunctionParam{
				Name:      tc.Name,
				Arguments: string(argsBytes),
			},
		})
	}
	msg := openai.AssistantMessage(m.Content)
	if msg.OfAssistant != nil {
		msg.OfAssistant.ToolCalls = toolCalls
	}
	return msg
}

// --- openaiStreamReader (text-only, existing) ---

type openaiStreamReader struct {
	stream *ssestream.Stream[openai.ChatCompletionChunk]
	mu     sync.Mutex
	buf    string
}

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

func (r *openaiStreamReader) Close() error { return r.stream.Close() }

func (r *openaiStreamReader) FullText() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf
}

// --- openaiToolStreamReader (tool-use capable) ---

// toolCallAccum tracks a tool call being accumulated across streaming chunks.
type toolCallAccum struct {
	id       string
	name     string
	argsJSON string
}

// openaiToolStreamReader handles streaming with tool call interception.
// It accumulates tool call arguments across chunks (they arrive in fragments),
// emits ToolCallComplete events when a tool call is fully received, and
// supports sending results back to continue the conversation.
type openaiToolStreamReader struct {
	provider  *OpenAIProvider
	ctx       context.Context
	cfg       *CompletionConfig
	messages  []ChatMessage
	tools     []ToolDefinition
	stream    *ssestream.Stream[openai.ChatCompletionChunk]
	accum     map[int]*toolCallAccum
	pending   []ToolCall
	pendingAt int
	mu        sync.Mutex
	buf       string
	done      bool
	usage     TokenUsage
}

// Next returns the next event from the tool-use stream.
func (r *openaiToolStreamReader) Next() (StreamEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// If we have pending ToolCallComplete events to emit, emit them first.
	if r.pendingAt < len(r.pending) {
		tc := r.pending[r.pendingAt]
		r.pendingAt++
		return StreamEvent{Type: EventToolCallComplete, ToolCall: &tc}, nil
	}

	if r.done {
		return StreamEvent{Type: EventDone}, io.EOF
	}

	for r.stream.Next() {
		chunk := r.stream.Current()

		// Capture usage from final chunk (OpenAI sends it when stream_options include_usage is set).
		if chunk.Usage.TotalTokens > 0 {
			r.usage.InputTokens = int(chunk.Usage.PromptTokens)
			r.usage.OutputTokens = int(chunk.Usage.CompletionTokens)
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		// Text delta.
		if choice.Delta.Content != "" {
			r.buf += choice.Delta.Content
			return StreamEvent{Type: EventTextDelta, Text: choice.Delta.Content}, nil
		}

		// Tool call deltas — accumulate by index.
		for _, tc := range choice.Delta.ToolCalls {
			idx := int(tc.Index)
			existing, ok := r.accum[idx]
			if !ok {
				existing = &toolCallAccum{}
				r.accum[idx] = existing
			}
			if tc.ID != "" {
				existing.id = tc.ID
			}
			if tc.Function.Name != "" {
				existing.name = tc.Function.Name
			}
			existing.argsJSON += tc.Function.Arguments

			// Generate an ID if the proxy doesn't provide one.
			if existing.id == "" && existing.name != "" {
				existing.id = fmt.Sprintf("call_%s_%d", existing.name, idx)
			}
		}

		// Check finish reason.
		if choice.FinishReason == "tool_calls" {
			r.pending = nil
			r.pendingAt = 0
			for _, acc := range r.accum {
				if acc == nil {
					continue
				}
				var args map[string]any
				if err := json.Unmarshal([]byte(acc.argsJSON), &args); err != nil {
					args = map[string]any{"raw": acc.argsJSON}
				}
				r.pending = append(r.pending, ToolCall{
					ID: acc.id, Name: acc.name, Arguments: args,
				})
			}
			if len(r.pending) > 0 {
				first := r.pending[0]
				r.pendingAt = 1
				return StreamEvent{Type: EventToolCallComplete, ToolCall: &first}, nil
			}
		}

		// Any finish reason other than "tool_calls" means the stream is complete.
		// OpenAI uses "stop", Anthropic proxies may use "end_turn".
		if choice.FinishReason != "" && choice.FinishReason != "tool_calls" {
			r.done = true
			return StreamEvent{Type: EventDone}, nil
		}
	}

	if err := r.stream.Err(); err != nil {
		return StreamEvent{Type: EventError}, err
	}

	r.done = true
	return StreamEvent{Type: EventDone}, io.EOF
}

// SendToolResults sends tool execution results back to the LLM and starts a
// new streaming call so the LLM can continue generating with the results.
func (r *openaiToolStreamReader) SendToolResults(results []ToolResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Close the current stream.
	_ = r.stream.Close()

	// Build continuation messages:
	// 1. All original messages
	// 2. Assistant message with the tool calls it made
	// 3. Tool result messages (one per result)
	continued := make([]ChatMessage, len(r.messages))
	copy(continued, r.messages)

	// Add the assistant's tool call message.
	assistantToolCalls := append([]ToolCall{}, r.pending...)
	continued = append(continued, ChatMessage{
		Role:      "assistant",
		Content:   r.buf,
		ToolCalls: assistantToolCalls,
	})

	// Add tool result messages.
	for _, result := range results {
		continued = append(continued, ChatMessage{
			Role:       "tool",
			Content:    result.Content,
			ToolCallID: result.CallID,
		})
	}

	// Start a new stream with the continued conversation.
	params := r.provider.buildParams(r.cfg, continued, r.tools)
	r.stream = r.provider.client.Chat.Completions.NewStreaming(r.ctx, params)

	// Reset state for the new stream.
	r.messages = continued
	r.accum = make(map[int]*toolCallAccum)
	r.pending = nil
	r.pendingAt = 0
	r.done = false

	return nil
}

// Close releases stream resources.
func (r *openaiToolStreamReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stream.Close()
}

// FullText returns all text tokens accumulated across all continuation rounds.
func (r *openaiToolStreamReader) FullText() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf
}

// Usage returns the token usage captured from the stream.
func (r *openaiToolStreamReader) Usage() TokenUsage {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.usage
}

// Compile-time check that openaiToolStreamReader satisfies ToolStreamReader.
var _ ToolStreamReader = (*openaiToolStreamReader)(nil)
