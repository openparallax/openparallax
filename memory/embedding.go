package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// EmbeddingProvider generates vector embeddings for text.
type EmbeddingProvider interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
	ModelID() string
}

// EmbeddingConfig configures the embedding provider.
type EmbeddingConfig struct {
	Provider  string `yaml:"provider" json:"provider"` // "openai", "google", "ollama", "none"
	Model     string `yaml:"model" json:"model"`
	APIKeyEnv string `yaml:"api_key_env,omitempty" json:"api_key_env,omitempty"`
	APIKey    string `yaml:"-" json:"-"`
	BaseURL   string `yaml:"base_url,omitempty" json:"base_url,omitempty"`
}

// NewEmbeddingProvider creates an embedding provider from config.
// Returns nil if provider is "none" or empty.
func NewEmbeddingProvider(cfg EmbeddingConfig) EmbeddingProvider {
	// Resolve API key: explicit > env var from config > default env var per provider.
	resolveKey := func(defaultEnv string) string {
		if cfg.APIKey != "" {
			return cfg.APIKey
		}
		if cfg.APIKeyEnv != "" {
			return resolveEnv(cfg.APIKeyEnv)
		}
		return resolveEnv(defaultEnv)
	}

	switch cfg.Provider {
	case "openai":
		if cfg.Model == "" {
			cfg.Model = "text-embedding-3-small"
		}
		apiKey := resolveKey("OPENAI_API_KEY")
		if apiKey == "" {
			return nil
		}
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		return &openAIEmbedder{apiKey: apiKey, model: cfg.Model, baseURL: baseURL}

	case "google":
		if cfg.Model == "" {
			cfg.Model = "text-embedding-004"
		}
		apiKey := resolveKey("GOOGLE_API_KEY")
		if apiKey == "" {
			return nil
		}
		return &googleEmbedder{apiKey: apiKey, model: cfg.Model}

	case "ollama":
		if cfg.Model == "" {
			cfg.Model = "nomic-embed-text"
		}
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return &ollamaEmbedder{model: cfg.Model, baseURL: baseURL}

	default:
		return nil
	}
}

// --- OpenAI embedding provider ---

type openAIEmbedder struct {
	apiKey  string
	model   string
	baseURL string
}

func (e *openAIEmbedder) Dimension() int {
	switch e.model {
	case "text-embedding-3-large":
		return 3072
	case "text-embedding-3-small":
		return 1536
	case "text-embedding-ada-002":
		return 1536
	default:
		return 1536
	}
}

func (e *openAIEmbedder) ModelID() string { return e.model }

func (e *openAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	body := map[string]any{
		"input": texts,
		"model": e.model,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	embeddings := make([][]float32, len(result.Data))
	for i, d := range result.Data {
		embeddings[i] = d.Embedding
	}
	return embeddings, nil
}

// --- Ollama embedding provider ---

type ollamaEmbedder struct {
	model   string
	baseURL string
}

func (e *ollamaEmbedder) Dimension() int  { return 768 }
func (e *ollamaEmbedder) ModelID() string { return e.model }

func (e *ollamaEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	var embeddings [][]float32
	for _, text := range texts {
		body := map[string]any{
			"model":  e.model,
			"prompt": text,
		}
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/api/embeddings", bytes.NewReader(jsonBody))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}

		var result struct {
			Embedding []float32 `json:"embedding"`
		}
		if decErr := json.NewDecoder(resp.Body).Decode(&result); decErr != nil {
			_ = resp.Body.Close()
			return nil, decErr
		}
		_ = resp.Body.Close()
		embeddings = append(embeddings, result.Embedding)
	}
	return embeddings, nil
}

// --- Google embedding provider ---

type googleEmbedder struct {
	apiKey string
	model  string
}

func (e *googleEmbedder) Dimension() int {
	switch e.model {
	case "text-embedding-004":
		return 768
	default:
		return 768
	}
}

func (e *googleEmbedder) ModelID() string { return e.model }

func (e *googleEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	var embeddings [][]float32
	for _, text := range texts {
		body := map[string]any{
			"model": fmt.Sprintf("models/%s", e.model),
			"content": map[string]any{
				"parts": []map[string]any{
					{"text": text},
				},
			},
		}
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}

		url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:embedContent?key=%s",
			e.model, e.apiKey)
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return nil, fmt.Errorf("google embedding API error %d: %s", resp.StatusCode, string(respBody))
		}

		var result struct {
			Embedding struct {
				Values []float32 `json:"values"`
			} `json:"embedding"`
		}
		if decErr := json.NewDecoder(resp.Body).Decode(&result); decErr != nil {
			_ = resp.Body.Close()
			return nil, decErr
		}
		_ = resp.Body.Close()
		embeddings = append(embeddings, result.Embedding.Values)
	}
	return embeddings, nil
}

// resolveEnv reads an environment variable, falling back to an OP_-prefixed
// version if the original is not set.
func resolveEnv(name string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return os.Getenv("OP_" + name)
}
