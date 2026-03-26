package agent

import (
	"context"

	"github.com/openparallax/openparallax/internal/llm"
)

// Responder generates user-facing responses using the LLM.
type Responder struct {
	llm llm.Provider
}

// NewResponder creates a Responder with the given LLM provider.
func NewResponder(provider llm.Provider) *Responder {
	return &Responder{llm: provider}
}

// Generate creates a streaming response for the user's message in the context
// of conversation history and the assembled system prompt.
// Returns a StreamReader so tokens can be relayed to the channel adapter.
func (r *Responder) Generate(ctx context.Context, userMessage string, systemPrompt string, history []llm.ChatMessage) (llm.StreamReader, error) {
	messages := make([]llm.ChatMessage, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, llm.ChatMessage{Role: "user", Content: userMessage})

	return r.llm.StreamWithHistory(ctx, messages, llm.WithSystem(systemPrompt))
}
