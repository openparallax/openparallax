# LLM Provider Abstraction

The `llm` package provides a unified interface for interacting with multiple LLM providers. All provider-specific API differences -- message formats, streaming protocols, tool-calling conventions, token counting -- are abstracted behind a common `Provider` interface. Application code writes to one interface and the provider is selected at configuration time.

## Why a Provider Abstraction

Every LLM provider has a different SDK, different API semantics, different streaming formats, and different tool-calling conventions. Anthropic uses "Messages API" with content blocks. OpenAI uses "Chat Completions" with function calls. Google uses "GenerateContent" with function declarations. Ollama exposes an OpenAI-compatible HTTP API with no SDK at all.

Without an abstraction layer, every LLM integration point in the codebase would need provider-specific branches. The pipeline, the context assembler, the session summarizer, the Shield Tier 2 evaluator, and the title generator all call the LLM -- that is five places where provider changes would cascade.

The `llm` package eliminates this by normalizing:

- **Message format**: All providers accept the same `ChatMessage` struct with `Role`, `Content`, `ToolCalls`, and `ToolCallID` fields
- **Streaming**: All providers return the same `StreamReader` / `ToolStreamReader` interfaces with identical `Next()` / `SendToolResults()` methods
- **Tool calling**: All providers accept the same `ToolDefinition` and return the same `ToolCall` / `ToolResult` types
- **Options**: System prompts, max tokens, and temperature use the same functional options pattern
- **Token estimation**: All providers expose the same `EstimateTokens()` method for context window management
- **API host**: `APIHost()` returns the `host:port` string each provider needs, so the sandbox can whitelist exactly the right endpoint

## Supported Providers

| Provider | Config Value | SDK | API | Notes |
|----------|-------------|-----|-----|-------|
| **Anthropic Claude** | `anthropic` | `anthropics/anthropic-sdk-go` | Messages API | Native tool use. Prompt caching. Extended thinking. |
| **OpenAI GPT** | `openai` | `openai/openai-go` | Chat Completions | Compatible with Azure OpenAI, DeepSeek, Mistral, LM Studio via `base_url`. |
| **Google Gemini** | `google` | `google/generative-ai-go` | Gemini API | Function calling via `FunctionDeclaration`. |
| **Ollama** | `ollama` | Direct HTTP (no SDK) | OpenAI-compatible | Local inference. No API key required. Default: `http://localhost:11434`. |

### Provider Selection

Each provider is declared as an entry in the `models` pool in
`config.yaml` and selected by name through the `roles` mapping:

```yaml
models:
  # Anthropic Claude
  - name: chat
    provider: anthropic
    model: claude-sonnet-4-6
    api_key_env: ANTHROPIC_API_KEY

  # OpenAI (or any OpenAI-compatible endpoint)
  - name: shield
    provider: openai
    model: gpt-5.4-mini
    api_key_env: OPENAI_API_KEY
    base_url: ""   # Leave empty for OpenAI, set for Azure/DeepSeek/etc.

  # Google Gemini
  - name: embedding
    provider: google
    model: text-embedding-004
    api_key_env: GOOGLE_AI_API_KEY

  # Local Ollama
  - name: local
    provider: ollama
    model: llama3.1:70b
    base_url: http://localhost:11434   # Optional, this is the default

roles:
  chat: chat
  shield: shield
  embedding: embedding
```

## Provider Interface

Every LLM provider implements this interface:

```go
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
```

### Method Usage in OpenParallax

| Method | Used By | Purpose |
|--------|---------|---------|
| `StreamWithTools` | Engine pipeline | Primary conversation loop with tool calling |
| `Complete` | Session summarization, title generation, compaction | One-shot completions without streaming |
| `CompleteWithHistory` | Shield Tier 2 evaluator | Send conversation context for security evaluation |
| `Stream` | Agent context assembly | Streaming responses without tool use |
| `StreamWithHistory` | Web UI streaming | Stream a conversation to the frontend |
| `EstimateTokens` | Context compaction | Decide when to compact conversation history |
| `CheapestModel` | Sub-agent orchestration | Select cheapest model for delegated tasks |

### Complete / CompleteWithHistory

Sends a prompt (or conversation) and returns the full response as a string. Blocks until the entire response is received. Used for utility calls where streaming is not needed:

```go
response, err := provider.Complete(ctx, "Summarize this conversation",
    llm.WithSystem("You are a summarizer."),
    llm.WithMaxTokens(200),
)
```

`CompleteWithHistory` accepts a slice of `ChatMessage` instead of a single prompt string, allowing multi-turn context:

```go
response, err := provider.CompleteWithHistory(ctx, []llm.ChatMessage{
    {Role: "user", Content: "What is the capital of France?"},
    {Role: "assistant", Content: "The capital of France is Paris."},
    {Role: "user", Content: "And its population?"},
}, llm.WithMaxTokens(100))
```

## StreamReader

```go
type StreamReader interface {
    // Next returns the next token. Returns "", io.EOF when the stream is complete.
    Next() (string, error)

    // Close releases resources associated with the stream.
    Close() error

    // FullText returns all tokens accumulated so far, concatenated.
    FullText() string
}
```

StreamReader provides sequential access to streaming LLM tokens. Call `Next()` in a loop until it returns `io.EOF`:

```go
reader, err := provider.Stream(ctx, "Explain quantum computing")
if err != nil {
    log.Fatal(err)
}
defer reader.Close()

for {
    token, err := reader.Next()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    fmt.Print(token) // Print each token as it arrives
}

// After the loop, reader.FullText() contains the complete response
fmt.Println("\n---")
fmt.Println("Complete response:", reader.FullText())
```

## ToolStreamReader

```go
type ToolStreamReader interface {
    // Next returns the next event from the stream.
    // Returns io.EOF when the stream is fully complete.
    Next() (StreamEvent, error)

    // SendToolResults sends the results of tool executions back to the LLM.
    // The LLM will continue generating after receiving these results.
    // Call this after all tool calls in a batch have been processed.
    SendToolResults(results []ToolResult) error

    // SetTools replaces the active tool definitions for the next
    // continuation call. Used when the LLM dynamically loads
    // additional tool groups via load_tools.
    SetTools(tools []ToolDefinition)

    // Close releases resources.
    Close() error

    // FullText returns all text tokens accumulated so far.
    FullText() string

    // Usage returns the token usage metrics from the completed stream.
    Usage() TokenUsage
}
```

ToolStreamReader handles the multi-turn tool-use conversation loop. This is the primary interface used by the Engine pipeline. The caller reads events, processes tool calls, sends results back, and continues reading until the stream is done.

`SetTools` is called after `load_tools` returns new tool definitions. The updated schema must reach the provider before the next LLM continuation request, otherwise the LLM is told "tools loaded" but the function-call schema remains unchanged and the freshly loaded tools are not actually callable.

```go
reader, err := provider.StreamWithTools(ctx, messages, tools,
    llm.WithSystem("You are a helpful assistant."),
    llm.WithMaxTokens(4096),
)
if err != nil {
    log.Fatal(err)
}
defer reader.Close()

for {
    event, err := reader.Next()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }

    switch event.Type {
    case llm.EventTextDelta:
        fmt.Print(event.Text) // Stream text to the user

    case llm.EventToolCallComplete:
        // Execute the tool call
        result := executeMyTool(event.ToolCall)
        // Send the result back to the LLM so it can continue
        err := reader.SendToolResults([]llm.ToolResult{result})
        if err != nil {
            log.Fatal(err)
        }

    case llm.EventDone:
        break
    }
}

// Check token usage
usage := reader.Usage()
fmt.Printf("Input: %d tokens, Output: %d tokens\n", usage.InputTokens, usage.OutputTokens)
```

The conversation loop works as follows:

1. The LLM streams text tokens (`EventTextDelta`) and/or tool call events
2. When a tool call is complete (`EventToolCallComplete`), the caller executes the tool
3. The caller sends the result back via `SendToolResults()`
4. The LLM receives the result and continues generating (more text, more tool calls, or done)
5. When the LLM finishes, `Next()` returns `EventDone` followed by `io.EOF`

Multiple tool calls in a single turn are batched -- collect all `EventToolCallComplete` events before calling `SendToolResults()` with all results at once.

## Key Types

### ChatMessage

```go
type ChatMessage struct {
    Role       string     `json:"role"`          // "user", "assistant", "system", or "tool"
    Content    string     `json:"content"`       // Message text
    ToolCalls  []ToolCall `json:"tool_calls,omitempty"`    // Tool invocations (assistant messages)
    ToolCallID string     `json:"tool_call_id,omitempty"` // Which call this responds to (tool messages)
}
```

| Role | Description |
|---|---|
| `"user"` | User message |
| `"assistant"` | Assistant response (may include `ToolCalls`) |
| `"system"` | System prompt (prefer `WithSystem` option instead) |
| `"tool"` | Tool result (paired with `ToolCallID`) |

### StreamEvent

```go
type StreamEvent struct {
    Type     StreamEventType // Event kind
    Text     string          // Token text (EventTextDelta only)
    ToolCall *ToolCall       // Tool call data (EventToolCallStart/Complete only)
}
```

### StreamEventType

| Constant | Value | Description |
|----------|-------|-------------|
| `EventTextDelta` | 0 | Partial text token from the LLM |
| `EventToolCallStart` | 1 | LLM is beginning a tool call (name known, arguments still streaming) |
| `EventToolCallComplete` | 2 | Tool call fully received with ID, name, and parsed arguments |
| `EventDone` | 3 | Stream is fully complete |
| `EventError` | 4 | Stream error |

### ToolDefinition

```go
type ToolDefinition struct {
    Name        string         `json:"name"`        // Tool identifier (matches executor action types)
    Description string         `json:"description"` // When and how to use this tool
    Parameters  map[string]any `json:"parameters"`  // JSON Schema for the tool's input
}
```

Each provider translates `ToolDefinition` into its native tool format. Anthropic uses `input_schema`, OpenAI uses `function.parameters`, Gemini uses `FunctionDeclaration` -- but you write one definition and it works everywhere:

```go
tool := llm.ToolDefinition{
    Name:        "read_file",
    Description: "Read the contents of a file at the given path",
    Parameters: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "path": map[string]any{
                "type":        "string",
                "description": "The absolute or relative file path to read",
            },
        },
        "required": []string{"path"},
    },
}
```

### ToolCall

```go
type ToolCall struct {
    ID        string         `json:"id"`        // Unique identifier (assigned by the LLM/SDK)
    Name      string         `json:"name"`      // Tool name (maps to ActionType)
    Arguments map[string]any `json:"arguments"` // Parsed JSON arguments
}
```

A tool invocation requested by the LLM. The `ID` is assigned by the LLM SDK and must be included in the corresponding `ToolResult` to match the call with its response.

### ToolResult

```go
type ToolResult struct {
    CallID  string `json:"call_id"` // Matches ToolCall.ID
    Content string `json:"content"` // Result text the LLM will see
    IsError bool   `json:"is_error"` // Whether the tool call failed or was blocked
}
```

Set `IsError` to `true` when the tool call fails or is blocked by Shield. The LLM will see the error message in `Content` and can decide to retry, try a different approach, or inform the user.

### TokenUsage

```go
type TokenUsage struct {
    InputTokens          int // Total input tokens billed
    OutputTokens         int // Total output tokens generated
    CacheCreationTokens  int // Tokens written to cache (Anthropic)
    CacheReadTokens      int // Tokens read from cache (Anthropic, OpenAI)
    ToolDefinitionTokens int // Estimated tokens for tool definitions sent
}
```

Token usage metrics from a completed LLM call. Anthropic and OpenAI report cache tokens separately. `ToolDefinitionTokens` is an estimate based on the serialized size of tool definitions sent with the request.

## Options

Options use the functional options pattern. They can be passed to any `Complete`, `Stream`, or `StreamWithTools` call:

```go
// Set a system prompt
llm.WithSystem("You are a coding assistant.")

// Limit response length
llm.WithMaxTokens(8192)

// Control randomness (0.0 = deterministic, 1.0 = creative)
llm.WithTemperature(0.3)
```

Options can be combined in any order:

```go
response, err := provider.Complete(ctx, prompt,
    llm.WithSystem("Summarize the following conversation."),
    llm.WithMaxTokens(500),
    llm.WithTemperature(0.0),
)
```

### Defaults

| Option | Default | Description |
|--------|---------|-------------|
| `MaxTokens` | 4096 | Maximum tokens in the response |
| `Temperature` | 0.7 | Sampling temperature |
| `SystemPrompt` | (none) | No system prompt unless specified |

## Token Estimation

```go
func (p *Provider) EstimateTokens(text string) int
```

Returns an approximate token count for the input text. Used by the context compaction system to decide when the conversation history exceeds the model's context window and needs to be summarized.

The estimate is provider-specific but generally conservative (slightly over-counts) to avoid accidentally exceeding context limits. A common approximation is 4 characters per token for English text.

## Factory Function

```go
func NewProvider(cfg Config) (Provider, error)
```

Creates the appropriate provider based on the configuration. This is the primary entry point for obtaining a `Provider` instance:

```go
provider, err := llm.NewProvider(llm.Config{
    Provider:  "anthropic",
    Model:     "claude-sonnet-4-6",
    APIKeyEnv: "ANTHROPIC_API_KEY",
})
if err != nil {
    log.Fatal(err)
}
```

The factory:

1. Reads the API key from the environment variable specified by `APIKeyEnv`
2. Returns an error if the environment variable is not set (except for Ollama, which does not require an API key)
3. Instantiates the provider-specific implementation
4. Returns it as the `Provider` interface

### LLMConfig

```go
type LLMConfig struct {
    Provider  string `yaml:"provider"`       // "anthropic", "openai", "google", "ollama"
    Model     string `yaml:"model"`          // Model name
    APIKeyEnv string `yaml:"api_key_env"`    // Env var containing the API key
    BaseURL   string `yaml:"base_url"`       // Custom endpoint (OpenAI-compatible, Ollama)
}
```

## APIHost

```go
func APIHost(cfg Config) string
```

Returns the `host:port` string for the LLM API endpoint. Used by the sandbox to whitelist outbound network connections -- the sandboxed agent process needs to reach the LLM API but should be blocked from all other network access.

| Provider | Default Host |
|----------|-------------|
| `anthropic` | `api.anthropic.com:443` |
| `openai` | `api.openai.com:443` |
| `google` | `generativelanguage.googleapis.com:443` |
| `ollama` | `localhost:11434` |

For OpenAI and Ollama, if `base_url` is set in the config, the host is extracted from the URL. The scheme determines the default port: `https://` defaults to 443, `http://` defaults to 80.

```go
host := llm.APIHost(cfg)
// "api.anthropic.com:443" for Anthropic
// "api.openai.com:443" for OpenAI
// "my-azure-endpoint.openai.azure.com:443" for Azure OpenAI with custom base_url
```

## TestConnection

```go
func TestConnection(cfg Config, apiKey string) error
```

Creates a provider and sends a minimal test prompt (`"Respond with OK"` with `MaxTokens(5)`) with a 15-second timeout. Returns `nil` on success. Used by `openparallax init`, `openparallax doctor`, and the web settings UI to verify that the LLM is reachable and the API key is valid.

```go
err := llm.TestConnection(cfg, os.Getenv("ANTHROPIC_API_KEY"))
if err != nil {
    fmt.Printf("LLM connection failed: %s\n", err)
}
```

The test prompt is minimal by design -- it tests connectivity and authentication without consuming meaningful tokens.

## Provider-Specific Notes

### Anthropic

- Uses the official `anthropics/anthropic-sdk-go`
- Native support for tool use via the Messages API with content blocks
- Prompt caching reduces costs for repeated system prompts and tool definitions
- `CacheCreationTokens` and `CacheReadTokens` are reported in `TokenUsage`
- API host: `api.anthropic.com:443`

### OpenAI

- Uses the official `openai/openai-go` SDK
- Tool calling uses the `function` format in Chat Completions
- Setting `base_url` enables compatibility with:
  - Azure OpenAI
  - DeepSeek
  - Mistral
  - LM Studio
  - OpenRouter
  - Any OpenAI-compatible API
- `CacheReadTokens` reported for models that support cached prompts
- API host: `api.openai.com:443` (or extracted from `base_url`)

### Google Gemini

- Uses the official `google/generative-ai-go` SDK
- Function calling is mapped to/from Gemini's `FunctionDeclaration` format
- API key is read from the `GOOGLE_AI_API_KEY` environment variable
- API host: `generativelanguage.googleapis.com:443`

### Ollama

- No SDK dependency -- communicates directly via HTTP with Ollama's REST API
- No API key required (local inference)
- Default endpoint: `http://localhost:11434`
- Tool calling uses the OpenAI-compatible chat format
- Ideal for development, testing, privacy-sensitive deployments, and air-gapped environments
- API host: `localhost:11434` (or extracted from `base_url`)

## Key Source Files

| File | Purpose |
|---|---|
| `llm/provider.go` | Provider interface, StreamReader, ToolStreamReader, all types, options |
| `llm/factory.go` | NewProvider factory, APIHost, TestConnection |
| `llm/config.go` | Config struct |
| `llm/anthropic.go` | Anthropic Claude implementation |
| `llm/openai.go` | OpenAI (and compatible) implementation |
| `llm/google.go` | Google Gemini implementation |
| `llm/ollama.go` | Ollama implementation |
