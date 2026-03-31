// Package llm provides a unified interface for interacting with multiple LLM
// providers including Anthropic, OpenAI, Google Gemini, and Ollama.
package llm

import "context"

// Provider is the interface all LLM providers implement.
type Provider interface {
	// Complete sends a prompt and returns the full response.
	// Used for session summarization, compaction, and utility calls.
	Complete(ctx context.Context, prompt string, opts ...Option) (string, error)

	// CompleteWithHistory sends a conversation and returns the full response.
	CompleteWithHistory(ctx context.Context, messages []ChatMessage, opts ...Option) (string, error)

	// Stream sends a prompt and returns a StreamReader for progressive token consumption.
	Stream(ctx context.Context, prompt string, opts ...Option) (StreamReader, error)

	// StreamWithHistory sends a conversation and returns a StreamReader.
	StreamWithHistory(ctx context.Context, messages []ChatMessage, opts ...Option) (StreamReader, error)

	// StreamWithTools sends a conversation with tool definitions and returns a
	// ToolStreamReader. This is the primary pipeline call. The LLM can respond
	// with text, tool calls, or both. The caller processes tool calls, sends
	// results back via SendToolResults, and the LLM continues.
	StreamWithTools(ctx context.Context, messages []ChatMessage, tools []ToolDefinition, opts ...Option) (ToolStreamReader, error)

	// EstimateTokens returns an approximate token count for the input text.
	EstimateTokens(text string) int

	// Name returns the provider name (for logging and display).
	Name() string

	// Model returns the model name (for logging and display).
	Model() string

	// CheapestModel returns the cheapest/fastest model available on this provider.
	// Used as the default model for sub-agents to optimize cost.
	CheapestModel() string
}

// StreamReader provides sequential access to streaming LLM tokens.
type StreamReader interface {
	// Next returns the next token. Returns "", io.EOF when the stream is complete.
	Next() (string, error)

	// Close releases resources associated with the stream.
	Close() error

	// FullText returns all tokens accumulated so far, concatenated.
	FullText() string
}

// ToolStreamReader handles the tool-use conversation loop.
// The caller reads events, processes tool calls, sends results back,
// and continues reading until the stream is done.
type ToolStreamReader interface {
	// Next returns the next event from the stream.
	// Returns io.EOF when the stream is fully complete.
	Next() (StreamEvent, error)

	// SendToolResults sends the results of tool executions back to the LLM.
	// The LLM will continue generating after receiving these results.
	// Call this after all tool calls in a batch have been processed.
	SendToolResults(results []ToolResult) error

	// Close releases resources.
	Close() error

	// FullText returns all text tokens accumulated so far.
	FullText() string

	// Usage returns the token usage metrics from the completed stream.
	Usage() TokenUsage
}

// StreamEvent is a single event from the tool-use stream.
type StreamEvent struct {
	// Type identifies the kind of event.
	Type StreamEventType

	// Text carries the delta for TextDelta events.
	Text string

	// ToolCall carries the tool call for ToolCallStart and ToolCallComplete events.
	ToolCall *ToolCall
}

// StreamEventType identifies the kind of stream event.
type StreamEventType int

const (
	// EventTextDelta carries a partial text token from the LLM.
	EventTextDelta StreamEventType = iota
	// EventToolCallStart signals the LLM is beginning a tool call.
	EventToolCallStart
	// EventToolCallComplete signals a tool call is fully received with ID, name, and arguments.
	EventToolCallComplete
	// EventDone signals the stream is fully complete.
	EventDone
	// EventError signals a stream error.
	EventError
)

// ToolDefinition describes a tool the LLM can invoke.
type ToolDefinition struct {
	// Name is the tool identifier (matches executor action types).
	Name string `json:"name"`

	// Description tells the LLM when and how to use this tool.
	Description string `json:"description"`

	// Parameters is the JSON Schema for the tool's input.
	Parameters map[string]any `json:"parameters"`
}

// ToolCall is a tool invocation requested by the LLM.
type ToolCall struct {
	// ID is the unique identifier for this tool call (assigned by the LLM/SDK).
	ID string `json:"id"`

	// Name is the tool name (maps to ActionType).
	Name string `json:"name"`

	// Arguments is the parsed JSON arguments.
	Arguments map[string]any `json:"arguments"`
}

// ToolResult is the result of executing a tool call, sent back to the LLM.
type ToolResult struct {
	// CallID matches the ToolCall.ID.
	CallID string `json:"call_id"`

	// Content is the result content the LLM will see.
	Content string `json:"content"`

	// IsError indicates the tool call failed or was blocked.
	IsError bool `json:"is_error"`
}

// TokenUsage holds token consumption metrics from an LLM call.
type TokenUsage struct {
	// InputTokens is the total input tokens billed.
	InputTokens int

	// OutputTokens is the total output tokens generated.
	OutputTokens int

	// CacheCreationTokens is the tokens written to cache (Anthropic).
	CacheCreationTokens int

	// CacheReadTokens is the tokens read from cache (Anthropic, OpenAI).
	CacheReadTokens int

	// ToolDefinitionTokens is the estimated tokens for tool definitions sent.
	ToolDefinitionTokens int
}

// ChatMessage is a single message in a conversation.
type ChatMessage struct {
	// Role is "user", "assistant", "system", or "tool".
	Role string `json:"role"`

	// Content is the message text.
	Content string `json:"content"`

	// ToolCalls is set on assistant messages that request tool invocations.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// ToolCallID is set on tool result messages to identify which call this responds to.
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// Option configures a completion request.
type Option func(*CompletionConfig)

// CompletionConfig holds optional parameters for a completion.
type CompletionConfig struct {
	// SystemPrompt is prepended as a system message.
	SystemPrompt string

	// MaxTokens is the maximum number of tokens in the response.
	MaxTokens int

	// Temperature controls randomness (0.0 = deterministic, 1.0 = creative).
	Temperature float64
}

// WithSystem sets the system prompt.
func WithSystem(prompt string) Option {
	return func(c *CompletionConfig) { c.SystemPrompt = prompt }
}

// WithMaxTokens sets the maximum response tokens.
func WithMaxTokens(n int) Option {
	return func(c *CompletionConfig) { c.MaxTokens = n }
}

// WithTemperature sets the sampling temperature.
func WithTemperature(t float64) Option {
	return func(c *CompletionConfig) { c.Temperature = t }
}

// applyOptions merges option functions into a CompletionConfig with defaults.
func applyOptions(opts []Option) *CompletionConfig {
	cfg := &CompletionConfig{
		MaxTokens:   4096,
		Temperature: 0.7,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}
