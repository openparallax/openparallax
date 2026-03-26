package llm

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GoogleProvider implements Provider for Google's Gemini models.
type GoogleProvider struct {
	client *genai.Client
	model  string
}

// NewGoogleProvider creates a Google Gemini provider with the given API key and model.
func NewGoogleProvider(apiKey, model string) (*GoogleProvider, error) {
	client, err := genai.NewClient(context.Background(), option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}
	return &GoogleProvider{client: client, model: model}, nil
}

// Complete sends a prompt and returns the full response.
func (g *GoogleProvider) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
	cfg := applyOptions(opts)
	model := g.configureModel(cfg)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	return extractGeminiText(resp), nil
}

// CompleteWithHistory sends a conversation and returns the full response.
func (g *GoogleProvider) CompleteWithHistory(ctx context.Context, messages []ChatMessage, opts ...Option) (string, error) {
	cfg := applyOptions(opts)
	model := g.configureModel(cfg)

	cs := model.StartChat()
	history, lastMsg := splitGeminiHistory(messages)
	cs.History = history

	resp, err := cs.SendMessage(ctx, genai.Text(lastMsg))
	if err != nil {
		return "", err
	}

	return extractGeminiText(resp), nil
}

// Stream sends a prompt and returns a StreamReader.
func (g *GoogleProvider) Stream(ctx context.Context, prompt string, opts ...Option) (StreamReader, error) {
	cfg := applyOptions(opts)
	model := g.configureModel(cfg)

	iter := model.GenerateContentStream(ctx, genai.Text(prompt))
	return &geminiStreamReader{iter: iter}, nil
}

// StreamWithHistory sends a conversation and returns a StreamReader.
func (g *GoogleProvider) StreamWithHistory(ctx context.Context, messages []ChatMessage, opts ...Option) (StreamReader, error) {
	cfg := applyOptions(opts)
	model := g.configureModel(cfg)

	cs := model.StartChat()
	history, lastMsg := splitGeminiHistory(messages)
	cs.History = history

	iter := cs.SendMessageStream(ctx, genai.Text(lastMsg))
	return &geminiStreamReader{iter: iter}, nil
}

// EstimateTokens returns a rough token estimate (1 token per 4 characters).
func (g *GoogleProvider) EstimateTokens(text string) int { return len(text) / 4 }

// Name returns "google".
func (g *GoogleProvider) Name() string { return "google" }

// Model returns the model name.
func (g *GoogleProvider) Model() string { return g.model }

// configureModel creates a GenerativeModel with the given options.
func (g *GoogleProvider) configureModel(cfg *CompletionConfig) *genai.GenerativeModel {
	model := g.client.GenerativeModel(g.model)
	model.SetTemperature(float32(cfg.Temperature))
	model.SetMaxOutputTokens(int32(cfg.MaxTokens))
	if cfg.SystemPrompt != "" {
		model.SystemInstruction = genai.NewUserContent(genai.Text(cfg.SystemPrompt))
	}
	return model
}

// geminiStreamReader wraps the Gemini streaming iterator.
type geminiStreamReader struct {
	iter *genai.GenerateContentResponseIterator
	mu   sync.Mutex
	buf  string
}

// Next returns the next text chunk from the stream.
func (r *geminiStreamReader) Next() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	resp, err := r.iter.Next()
	if err == iterator.Done {
		return "", io.EOF
	}
	if err != nil {
		return "", err
	}

	text := extractGeminiText(resp)
	r.buf += text
	return text, nil
}

// Close is a no-op for Gemini (iterator is consumed by Next).
func (r *geminiStreamReader) Close() error { return nil }

// FullText returns all accumulated text.
func (r *geminiStreamReader) FullText() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf
}

// extractGeminiText extracts text from a Gemini response.
func extractGeminiText(resp *genai.GenerateContentResponse) string {
	if resp == nil || len(resp.Candidates) == 0 {
		return ""
	}
	candidate := resp.Candidates[0]
	if candidate.Content == nil {
		return ""
	}
	var sb strings.Builder
	for _, part := range candidate.Content.Parts {
		if text, ok := part.(genai.Text); ok {
			sb.WriteString(string(text))
		}
	}
	return sb.String()
}

// splitGeminiHistory converts ChatMessages to Gemini Content history + final message.
// Gemini requires alternating user/model turns. The last user message is returned
// separately to be passed to SendMessage.
func splitGeminiHistory(messages []ChatMessage) ([]*genai.Content, string) {
	if len(messages) == 0 {
		return nil, ""
	}

	var history []*genai.Content
	for i := 0; i < len(messages)-1; i++ {
		m := messages[i]
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}
		history = append(history, &genai.Content{
			Parts: []genai.Part{genai.Text(m.Content)},
			Role:  role,
		})
	}

	return history, messages[len(messages)-1].Content
}
