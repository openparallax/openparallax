// Package llm provides a unified interface for interacting with multiple LLM
// providers including Anthropic, OpenAI, Google Gemini, and Ollama.
package llm

import "context"

// Provider is the interface all LLM providers implement.
type Provider interface {
	// Complete sends a prompt and returns the full response.
	// Used for intent extraction, self-eval, compaction, session summarization.
	Complete(ctx context.Context, prompt string, opts ...Option) (string, error)

	// CompleteWithHistory sends a conversation and returns the full response.
	CompleteWithHistory(ctx context.Context, messages []ChatMessage, opts ...Option) (string, error)

	// Stream sends a prompt and returns a StreamReader for progressive token consumption.
	// Used for user-facing response generation.
	Stream(ctx context.Context, prompt string, opts ...Option) (StreamReader, error)

	// StreamWithHistory sends a conversation and returns a StreamReader.
	StreamWithHistory(ctx context.Context, messages []ChatMessage, opts ...Option) (StreamReader, error)

	// EstimateTokens returns an approximate token count for the input text.
	EstimateTokens(text string) int

	// Name returns the provider name (for logging and display).
	Name() string

	// Model returns the model name (for logging and display).
	Model() string
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

// ChatMessage is a single message in a conversation.
type ChatMessage struct {
	// Role is "user", "assistant", or "system".
	Role string `json:"role"`

	// Content is the message text.
	Content string `json:"content"`
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
