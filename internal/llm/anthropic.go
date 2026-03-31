package llm

import (
	"context"
	"encoding/json"
	"io"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	anthropicParam "github.com/anthropics/anthropic-sdk-go/packages/param"
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
	params := a.buildParams(cfg, []ChatMessage{{Role: "user", Content: prompt}})

	msg, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return "", err
	}

	return extractAnthropicText(msg), nil
}

// CompleteWithHistory sends a conversation and returns the full response.
func (a *AnthropicProvider) CompleteWithHistory(ctx context.Context, messages []ChatMessage, opts ...Option) (string, error) {
	cfg := applyOptions(opts)
	params := a.buildParams(cfg, messages)

	msg, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return "", err
	}

	return extractAnthropicText(msg), nil
}

// Stream sends a prompt and returns a StreamReader.
func (a *AnthropicProvider) Stream(ctx context.Context, prompt string, opts ...Option) (StreamReader, error) {
	cfg := applyOptions(opts)
	params := a.buildParams(cfg, []ChatMessage{{Role: "user", Content: prompt}})

	stream := a.client.Messages.NewStreaming(ctx, params)
	return &anthropicStreamReader{stream: stream}, nil
}

// StreamWithHistory sends a conversation and returns a StreamReader.
func (a *AnthropicProvider) StreamWithHistory(ctx context.Context, messages []ChatMessage, opts ...Option) (StreamReader, error) {
	cfg := applyOptions(opts)
	params := a.buildParams(cfg, messages)

	stream := a.client.Messages.NewStreaming(ctx, params)
	return &anthropicStreamReader{stream: stream}, nil
}

// StreamWithTools sends a conversation with tool definitions and returns a ToolStreamReader.
func (a *AnthropicProvider) StreamWithTools(ctx context.Context, messages []ChatMessage, tools []ToolDefinition, opts ...Option) (ToolStreamReader, error) {
	cfg := applyOptions(opts)
	params := a.buildParams(cfg, messages)

	// Add tools.
	for _, t := range tools {
		props := t.Parameters["properties"]
		required, _ := t.Parameters["required"].([]any)
		var reqStrings []string
		for _, r := range required {
			if s, ok := r.(string); ok {
				reqStrings = append(reqStrings, s)
			}
		}
		params.Tools = append(params.Tools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropicParam.NewOpt(t.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: props,
					Required:   reqStrings,
				},
			},
		})
	}

	stream := a.client.Messages.NewStreaming(ctx, params)
	return &anthropicToolStreamReader{
		provider: a,
		ctx:      ctx,
		cfg:      cfg,
		messages: messages,
		tools:    tools,
		stream:   stream,
		accum:    make(map[int]*toolCallAccum),
	}, nil
}

// EstimateTokens returns a rough token estimate (1 token per 4 characters).
func (a *AnthropicProvider) EstimateTokens(text string) int { return len(text) / 4 }

// Name returns "anthropic".
func (a *AnthropicProvider) Name() string { return "anthropic" }

// Model returns the model name.
func (a *AnthropicProvider) Model() string { return a.model }

// buildParams constructs Anthropic MessageNewParams.
func (a *AnthropicProvider) buildParams(cfg *CompletionConfig, messages []ChatMessage) anthropic.MessageNewParams {
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
	return params
}

// --- anthropicStreamReader (text-only) ---

type anthropicStreamReader struct {
	stream *ssestream.Stream[anthropic.MessageStreamEventUnion]
	mu     sync.Mutex
	buf    string
}

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

func (r *anthropicStreamReader) Close() error { return r.stream.Close() }

func (r *anthropicStreamReader) FullText() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf
}

// --- anthropicToolStreamReader (tool-use capable) ---

type anthropicToolStreamReader struct {
	provider  *AnthropicProvider
	ctx       context.Context
	cfg       *CompletionConfig
	messages  []ChatMessage
	tools     []ToolDefinition
	stream    *ssestream.Stream[anthropic.MessageStreamEventUnion]
	accum     map[int]*toolCallAccum
	pending   []ToolCall
	pendingAt int
	mu        sync.Mutex
	buf       string
	done      bool
	usage     TokenUsage
}

func (r *anthropicToolStreamReader) Next() (StreamEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.pendingAt < len(r.pending) {
		tc := r.pending[r.pendingAt]
		r.pendingAt++
		return StreamEvent{Type: EventToolCallComplete, ToolCall: &tc}, nil
	}

	if r.done {
		return StreamEvent{Type: EventDone}, io.EOF
	}

	for r.stream.Next() {
		event := r.stream.Current()

		switch event.Type {
		case "message_start":
			msg := event.AsMessageStart()
			r.usage.InputTokens = int(msg.Message.Usage.InputTokens)
			r.usage.OutputTokens = int(msg.Message.Usage.OutputTokens)
			r.usage.CacheCreationTokens = int(msg.Message.Usage.CacheCreationInputTokens)
			r.usage.CacheReadTokens = int(msg.Message.Usage.CacheReadInputTokens)
			continue

		case "content_block_start":
			block := event.AsContentBlockStart()
			if block.ContentBlock.Type == "tool_use" {
				tb := block.ContentBlock.AsToolUse()
				r.accum[int(block.Index)] = &toolCallAccum{
					id:   tb.ID,
					name: tb.Name,
				}
			}

		case "content_block_delta":
			delta := event.AsContentBlockDelta()
			if delta.Delta.Type == "text_delta" {
				text := delta.Delta.AsTextDelta().Text
				r.buf += text
				return StreamEvent{Type: EventTextDelta, Text: text}, nil
			}
			if delta.Delta.Type == "input_json_delta" {
				idx := int(delta.Index)
				if acc, ok := r.accum[idx]; ok {
					acc.argsJSON += delta.Delta.AsInputJSONDelta().PartialJSON
				}
			}

		case "content_block_stop":
			idx := int(event.AsContentBlockStop().Index)
			if acc, ok := r.accum[idx]; ok && acc.id != "" {
				var args map[string]any
				if err := json.Unmarshal([]byte(acc.argsJSON), &args); err != nil {
					args = map[string]any{"raw": acc.argsJSON}
				}
				tc := ToolCall{ID: acc.id, Name: acc.name, Arguments: args}
				delete(r.accum, idx)
				return StreamEvent{Type: EventToolCallComplete, ToolCall: &tc}, nil
			}

		case "message_delta":
			md := event.AsMessageDelta()
			if md.Usage.OutputTokens > 0 {
				r.usage.OutputTokens = int(md.Usage.OutputTokens)
			}
			if md.Delta.StopReason == "end_turn" || md.Delta.StopReason == "stop_sequence" {
				r.done = true
				return StreamEvent{Type: EventDone}, nil
			}
			if md.Delta.StopReason == "tool_use" {
				// More tool calls may have been emitted via content_block_stop already.
				// If any remaining accum entries, emit them.
				for idx, acc := range r.accum {
					if acc.id != "" {
						var args map[string]any
						_ = json.Unmarshal([]byte(acc.argsJSON), &args)
						r.pending = append(r.pending, ToolCall{ID: acc.id, Name: acc.name, Arguments: args})
						delete(r.accum, idx)
					}
				}
				if len(r.pending) > 0 {
					first := r.pending[0]
					r.pendingAt = 1
					return StreamEvent{Type: EventToolCallComplete, ToolCall: &first}, nil
				}
			}

		case "message_stop":
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

func (r *anthropicToolStreamReader) SendToolResults(results []ToolResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_ = r.stream.Close()

	// Build continuation: all prior messages + assistant with tool_use + user with tool_result.
	continued := make([]ChatMessage, len(r.messages))
	copy(continued, r.messages)

	// Assistant message with tool calls.
	toolCalls := append([]ToolCall{}, r.pending...)
	continued = append(continued, ChatMessage{
		Role:      "assistant",
		Content:   r.buf,
		ToolCalls: toolCalls,
	})

	// Anthropic sends tool results as a user message with tool_result content blocks.
	// We model this as individual tool messages.
	for _, result := range results {
		continued = append(continued, ChatMessage{
			Role:       "tool",
			Content:    result.Content,
			ToolCallID: result.CallID,
		})
	}

	params := r.provider.buildParams(r.cfg, continued)
	for _, t := range r.tools {
		props := t.Parameters["properties"]
		required, _ := t.Parameters["required"].([]any)
		var reqStrings []string
		for _, rv := range required {
			if s, ok := rv.(string); ok {
				reqStrings = append(reqStrings, s)
			}
		}
		params.Tools = append(params.Tools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropicParam.NewOpt(t.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: props,
					Required:   reqStrings,
				},
			},
		})
	}

	r.stream = r.provider.client.Messages.NewStreaming(r.ctx, params)
	r.messages = continued
	r.accum = make(map[int]*toolCallAccum)
	r.pending = nil
	r.pendingAt = 0
	r.done = false

	return nil
}

func (r *anthropicToolStreamReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stream.Close()
}

func (r *anthropicToolStreamReader) FullText() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf
}

// Usage returns the token usage captured from the stream.
func (r *anthropicToolStreamReader) Usage() TokenUsage {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.usage
}

// --- Helpers ---

func extractAnthropicText(msg *anthropic.Message) string {
	var text string
	for _, block := range msg.Content {
		if block.Type == "text" {
			text += block.AsText().Text
		}
	}
	return text
}

func toAnthropicMessages(msgs []ChatMessage) []anthropic.MessageParam {
	result := make([]anthropic.MessageParam, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case "user":
			result = append(result, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		case "assistant":
			if len(m.ToolCalls) > 0 {
				var blocks []anthropic.ContentBlockParamUnion
				if m.Content != "" {
					blocks = append(blocks, anthropic.NewTextBlock(m.Content))
				}
				for _, tc := range m.ToolCalls {
					blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, tc.Arguments, tc.Name))
				}
				result = append(result, anthropic.NewAssistantMessage(blocks...))
			} else {
				result = append(result, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
			}
		case "tool":
			result = append(result, anthropic.NewUserMessage(
				anthropic.NewToolResultBlock(m.ToolCallID, m.Content, false),
			))
		}
	}
	return result
}

// Compile-time interface checks.
var (
	_ ToolStreamReader = (*anthropicToolStreamReader)(nil)
)
