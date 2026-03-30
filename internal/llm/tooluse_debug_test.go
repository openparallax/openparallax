package llm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToolUseRoundTrip exercises the full tool-use lifecycle:
// 1. Send a message with tool definitions → LLM returns a tool call
// 2. Send tool results back → LLM generates a final response using the results
//
// Set LLM_DEBUG=1 to enable request/response body logging for debugging.
func TestToolUseRoundTrip(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	model := os.Getenv("OPENAI_MODEL")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
	// Verify the key is valid before running the full test.
	checkURL := "https://api.openai.com/v1/models"
	if baseURL != "" {
		checkURL = baseURL + "/models"
	}
	httpClient := &http.Client{Timeout: 3 * time.Second}
	req, _ := http.NewRequest("GET", checkURL, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, httpErr := httpClient.Do(req)
	if httpErr != nil {
		t.Skipf("LLM endpoint unreachable: %v", httpErr)
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Skip("OPENAI_API_KEY is invalid or expired")
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}

	// Enable request/response logging when LLM_DEBUG=1.
	if os.Getenv("LLM_DEBUG") == "1" {
		opts = append(opts, option.WithMiddleware(debugMiddleware(t)))
	}

	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := openai.NewClient(opts...)

	ctx := context.Background()

	tools := []openai.ChatCompletionToolParam{
		{
			Function: shared.FunctionDefinitionParam{
				Name:        "get_weather",
				Description: param.NewOpt("Get the current weather for a city"),
				Parameters: shared.FunctionParameters(map[string]any{
					"type": "object",
					"properties": map[string]any{
						"city": map[string]any{
							"type":        "string",
							"description": "The city name",
						},
					},
					"required": []string{"city"},
				}),
			},
		},
	}

	// --- Step 1: Initial request with tool definitions ---

	stream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model: model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("What is the weather in Paris? Use the get_weather tool."),
		},
		Tools:               tools,
		MaxCompletionTokens: param.NewOpt(int64(500)),
	})

	type accum struct {
		id       string
		name     string
		argsJSON string
	}
	toolCalls := make(map[int]*accum)
	var textContent string
	var finishReason string

	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]

		if choice.Delta.Content != "" {
			textContent += choice.Delta.Content
		}

		for _, tc := range choice.Delta.ToolCalls {
			idx := int(tc.Index)
			if _, ok := toolCalls[idx]; !ok {
				toolCalls[idx] = &accum{}
			}
			if tc.ID != "" {
				toolCalls[idx].id = tc.ID
			}
			if tc.Function.Name != "" {
				toolCalls[idx].name = tc.Function.Name
			}
			toolCalls[idx].argsJSON += tc.Function.Arguments
		}

		if choice.FinishReason != "" {
			finishReason = choice.FinishReason
		}
	}
	require.NoError(t, stream.Err())
	stream.Close()

	require.Equal(t, "tool_calls", finishReason, "LLM should have finished with tool_calls")
	require.NotEmpty(t, toolCalls, "LLM should have made at least one tool call")

	// --- Step 2: Send tool results back ---

	var oaiToolCalls []openai.ChatCompletionMessageToolCallParam
	for _, tc := range toolCalls {
		id := tc.id
		if id == "" {
			id = fmt.Sprintf("call_%s_0", tc.name)
		}
		oaiToolCalls = append(oaiToolCalls, openai.ChatCompletionMessageToolCallParam{
			ID: id,
			Function: openai.ChatCompletionMessageToolCallFunctionParam{
				Name:      tc.name,
				Arguments: tc.argsJSON,
			},
		})
	}

	assistantMsg := openai.AssistantMessage(textContent)
	if assistantMsg.OfAssistant != nil {
		assistantMsg.OfAssistant.ToolCalls = oaiToolCalls
	}

	continuationMsgs := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("What is the weather in Paris? Use the get_weather tool."),
		assistantMsg,
	}
	for _, tc := range toolCalls {
		id := tc.id
		if id == "" {
			id = fmt.Sprintf("call_%s_0", tc.name)
		}
		continuationMsgs = append(continuationMsgs, openai.ToolMessage(`{"temperature": "22°C", "condition": "sunny", "city": "Paris"}`, id))
	}

	stream2 := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model:               model,
		Messages:            continuationMsgs,
		Tools:               tools,
		MaxCompletionTokens: param.NewOpt(int64(500)),
	})

	var responseText string
	for stream2.Next() {
		chunk := stream2.Current()
		if len(chunk.Choices) == 0 {
			continue
		}
		if choice := chunk.Choices[0]; choice.Delta.Content != "" {
			responseText += choice.Delta.Content
		}
	}
	require.NoError(t, stream2.Err())
	stream2.Close()

	assert.NotEmpty(t, responseText, "continuation response should contain the weather result")
	assert.Contains(t, responseText, "Paris", "response should reference Paris")
}

// debugMiddleware returns an HTTP middleware that logs request bodies and
// error responses. Activate with LLM_DEBUG=1.
func debugMiddleware(t *testing.T) option.Middleware {
	return func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		if req.Body != nil {
			bodyBytes, _ := io.ReadAll(req.Body)
			req.Body.Close()
			t.Logf("\n=== REQUEST %s %s ===\n%s\n=== END REQUEST ===", req.Method, req.URL.Path, string(bodyBytes))
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		resp, err := next(req)

		if resp != nil {
			t.Logf("=== RESPONSE STATUS: %d ===", resp.StatusCode)
			if resp.StatusCode >= 400 {
				respBody, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				t.Logf("=== ERROR BODY ===\n%s\n=== END ERROR BODY ===", string(respBody))
				resp.Body = io.NopCloser(bytes.NewReader(respBody))
			}
		}
		return resp, err
	}
}
