package main

// DefaultModels maps each provider to its recommended chat, shield, and
// embedding model names. Centralized here so init.go, tests, and any
// future command reference the same values. Update these when new model
// generations are released -- no other code changes needed.
var DefaultModels = map[string]struct {
	Chat      string
	Shield    string
	Embedding string
}{
	"anthropic": {Chat: "claude-sonnet-4-6", Shield: "claude-haiku-4-5-20251001"},
	"openai":    {Chat: "gpt-5.4", Shield: "gpt-5.4-mini", Embedding: "text-embedding-3-small"},
	"google":    {Chat: "gemini-3.1-pro", Shield: "gemini-3.1-flash-lite", Embedding: "text-embedding-004"},
	"ollama":    {Chat: "llama3.2", Shield: "llama3.2", Embedding: "nomic-embed-text"},
}
