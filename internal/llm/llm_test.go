package llm

import (
	"context"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestProvider returns an OpenAI provider configured for testing.
// Skips the test if no API key is available or if the endpoint is unreachable.
func getTestProvider(t *testing.T) Provider {
	t.Helper()
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	model := os.Getenv("OPENAI_MODEL")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping real LLM test")
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	// Verify the endpoint is reachable before running the test.
	checkURL := "https://api.openai.com/v1/models"
	if baseURL != "" {
		checkURL = baseURL + "/models"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	req, _ := http.NewRequest("GET", checkURL, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("LLM endpoint unreachable (%s), skipping: %v", checkURL, err)
	}
	_ = resp.Body.Close()

	p, err := NewOpenAIProvider(apiKey, model, baseURL)
	require.NoError(t, err)
	return p
}

func TestFactoryCreatesAnthropicProvider(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	p, err := NewProvider(types.LLMConfig{
		Provider:  "anthropic",
		Model:     "claude-sonnet-4-20250514",
		APIKeyEnv: "ANTHROPIC_API_KEY",
	})
	require.NoError(t, err)
	assert.Equal(t, "anthropic", p.Name())
	assert.Equal(t, "claude-sonnet-4-20250514", p.Model())
}

func TestFactoryCreatesOpenAIProvider(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	p, err := NewProvider(types.LLMConfig{
		Provider:  "openai",
		Model:     "gpt-4o",
		APIKeyEnv: "OPENAI_API_KEY",
	})
	require.NoError(t, err)
	assert.Equal(t, "openai", p.Name())
}

func TestFactoryCreatesOpenAIWithBaseURL(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	p, err := NewProvider(types.LLMConfig{
		Provider:  "openai",
		Model:     "custom-model",
		APIKeyEnv: "OPENAI_API_KEY",
		BaseURL:   "http://localhost:8080/v1",
	})
	require.NoError(t, err)
	assert.Equal(t, "openai", p.Name())
	assert.Equal(t, "custom-model", p.Model())
}

func TestFactoryCreatesGoogleProvider(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "test-key")
	p, err := NewProvider(types.LLMConfig{
		Provider:  "google",
		Model:     "gemini-2.0-flash",
		APIKeyEnv: "GOOGLE_API_KEY",
	})
	require.NoError(t, err)
	assert.Equal(t, "google", p.Name())
}

func TestFactoryCreatesOllamaProvider(t *testing.T) {
	p, err := NewProvider(types.LLMConfig{
		Provider: "ollama",
		Model:    "llama3.2",
	})
	require.NoError(t, err)
	assert.Equal(t, "ollama", p.Name())
}

func TestFactoryDefaultsOllamaBaseURL(t *testing.T) {
	p, err := NewProvider(types.LLMConfig{
		Provider: "ollama",
		Model:    "llama3.2",
	})
	require.NoError(t, err)
	ollama, ok := p.(*OllamaProvider)
	require.True(t, ok)
	assert.Equal(t, "http://localhost:11434", ollama.baseURL)
}

func TestFactoryErrorsOnUnknownProvider(t *testing.T) {
	_, err := NewProvider(types.LLMConfig{
		Provider: "unknown",
		Model:    "test",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestFactoryErrorsOnMissingAPIKey(t *testing.T) {
	t.Setenv("NONEXISTENT_KEY", "")
	_, err := NewProvider(types.LLMConfig{
		Provider:  "openai",
		Model:     "gpt-4o",
		APIKeyEnv: "DEFINITELY_NOT_SET_KEY_12345",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not set")
}

func TestEstimateTokensReasonable(t *testing.T) {
	providers := []Provider{
		&AnthropicProvider{model: "test"},
		&OpenAIProvider{model: "test"},
		&GoogleProvider{model: "test"},
		&OllamaProvider{model: "test"},
	}
	text := "Hello, this is a test message with some content."
	for _, p := range providers {
		tokens := p.EstimateTokens(text)
		assert.Greater(t, tokens, 0, "EstimateTokens should return > 0 for %s", p.Name())
		assert.Less(t, tokens, len(text), "EstimateTokens should return less than char count for %s", p.Name())
	}
}

func TestApplyOptionsDefaults(t *testing.T) {
	cfg := applyOptions(nil)
	assert.Equal(t, 4096, cfg.MaxTokens)
	assert.InDelta(t, 0.7, cfg.Temperature, 0.001)
	assert.Empty(t, cfg.SystemPrompt)
}

func TestApplyOptionsOverrides(t *testing.T) {
	cfg := applyOptions([]Option{
		WithSystem("test system"),
		WithMaxTokens(1000),
		WithTemperature(0.5),
	})
	assert.Equal(t, "test system", cfg.SystemPrompt)
	assert.Equal(t, 1000, cfg.MaxTokens)
	assert.InDelta(t, 0.5, cfg.Temperature, 0.001)
}

// Integration tests — require a real LLM endpoint.

func TestOpenAICompleteIntegration(t *testing.T) {
	p := getTestProvider(t)
	ctx := context.Background()

	response, err := p.Complete(ctx, "Say exactly: hello world", WithMaxTokens(50))
	require.NoError(t, err)
	assert.NotEmpty(t, response)
}

func TestOpenAIStreamIntegration(t *testing.T) {
	p := getTestProvider(t)
	ctx := context.Background()

	reader, err := p.Stream(ctx, "Count from 1 to 5", WithMaxTokens(100))
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()

	tokenCount := 0
	for {
		token, tokenErr := reader.Next()
		if tokenErr == io.EOF {
			break
		}
		require.NoError(t, tokenErr)
		assert.NotEmpty(t, token)
		tokenCount++
	}
	assert.Greater(t, tokenCount, 0, "should have received at least one token")
	assert.NotEmpty(t, reader.FullText(), "FullText should contain accumulated response")
}

func TestOpenAICompleteWithHistoryIntegration(t *testing.T) {
	p := getTestProvider(t)
	ctx := context.Background()

	messages := []ChatMessage{
		{Role: "user", Content: "My name is TestBot."},
		{Role: "assistant", Content: "Hello TestBot! Nice to meet you."},
		{Role: "user", Content: "What is my name?"},
	}

	response, err := p.CompleteWithHistory(ctx, messages, WithMaxTokens(50))
	require.NoError(t, err)
	assert.Contains(t, response, "TestBot")
}

func TestOpenAIStreamWithSystemPromptIntegration(t *testing.T) {
	p := getTestProvider(t)
	ctx := context.Background()

	reader, err := p.Stream(ctx, "Who are you?",
		WithSystem("You are a pirate. Always respond in pirate speak."),
		WithMaxTokens(100),
	)
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()

	for {
		_, tokenErr := reader.Next()
		if tokenErr == io.EOF {
			break
		}
		require.NoError(t, tokenErr)
	}
	assert.NotEmpty(t, reader.FullText())
}
