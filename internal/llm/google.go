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

// StreamWithTools sends a conversation with tool definitions.
func (g *GoogleProvider) StreamWithTools(ctx context.Context, messages []ChatMessage, tools []ToolDefinition, opts ...Option) (ToolStreamReader, error) {
	cfg := applyOptions(opts)
	model := g.configureModel(cfg)

	// Convert tool definitions to Gemini format.
	var funcs []*genai.FunctionDeclaration
	for _, t := range tools {
		fd := &genai.FunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
		}
		if t.Parameters != nil {
			fd.Parameters = toGeminiSchema(t.Parameters)
		}
		funcs = append(funcs, fd)
	}
	if len(funcs) > 0 {
		model.Tools = []*genai.Tool{{FunctionDeclarations: funcs}}
	}

	cs := model.StartChat()
	history, lastMsg := splitGeminiHistory(messages)
	cs.History = history

	iter := cs.SendMessageStream(ctx, genai.Text(lastMsg))
	return &geminiToolStreamReader{
		provider: g,
		ctx:      ctx,
		cfg:      cfg,
		cs:       cs,
		tools:    tools,
		iter:     iter,
	}, nil
}

// toGeminiSchema converts a JSON Schema map to a genai.Schema.
func toGeminiSchema(params map[string]any) *genai.Schema {
	schema := &genai.Schema{Type: genai.TypeObject}
	if props, ok := params["properties"].(map[string]any); ok {
		schema.Properties = make(map[string]*genai.Schema)
		for name, prop := range props {
			if propMap, ok := prop.(map[string]any); ok {
				s := &genai.Schema{}
				if t, ok := propMap["type"].(string); ok {
					switch t {
					case "string":
						s.Type = genai.TypeString
					case "number":
						s.Type = genai.TypeNumber
					case "integer":
						s.Type = genai.TypeInteger
					case "boolean":
						s.Type = genai.TypeBoolean
					}
				}
				if desc, ok := propMap["description"].(string); ok {
					s.Description = desc
				}
				schema.Properties[name] = s
			}
		}
	}
	if req, ok := params["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				schema.Required = append(schema.Required, s)
			}
		}
	}
	return schema
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

// --- geminiToolStreamReader (tool-use capable) ---

type geminiToolStreamReader struct {
	provider  *GoogleProvider
	ctx       context.Context
	cfg       *CompletionConfig
	cs        *genai.ChatSession
	tools     []ToolDefinition
	iter      *genai.GenerateContentResponseIterator
	pending   []ToolCall
	pendingAt int
	mu        sync.Mutex
	buf       string
	done      bool
}

func (r *geminiToolStreamReader) Next() (StreamEvent, error) {
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

	resp, err := r.iter.Next()
	if err == iterator.Done {
		r.done = true
		return StreamEvent{Type: EventDone}, nil
	}
	if err != nil {
		return StreamEvent{Type: EventError}, err
	}

	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return StreamEvent{Type: EventDone}, nil
	}

	for _, part := range resp.Candidates[0].Content.Parts {
		switch p := part.(type) {
		case genai.Text:
			text := string(p)
			r.buf += text
			return StreamEvent{Type: EventTextDelta, Text: text}, nil
		case genai.FunctionCall:
			args := make(map[string]any)
			for k, v := range p.Args {
				args[k] = v
			}
			tc := ToolCall{ID: p.Name, Name: p.Name, Arguments: args}
			return StreamEvent{Type: EventToolCallComplete, ToolCall: &tc}, nil
		}
	}

	return StreamEvent{Type: EventDone}, nil
}

func (r *geminiToolStreamReader) SendToolResults(results []ToolResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var parts []genai.Part
	for _, result := range results {
		parts = append(parts, genai.FunctionResponse{
			Name:     result.CallID,
			Response: map[string]any{"content": result.Content, "is_error": result.IsError},
		})
	}

	r.iter = r.cs.SendMessageStream(r.ctx, parts...)
	r.pending = nil
	r.pendingAt = 0
	r.done = false
	return nil
}

func (r *geminiToolStreamReader) Close() error { return nil }

func (r *geminiToolStreamReader) FullText() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf
}

var _ ToolStreamReader = (*geminiToolStreamReader)(nil)

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
