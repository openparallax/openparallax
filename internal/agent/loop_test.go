package agent

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	pb "github.com/openparallax/openparallax/internal/types/pb"
	"github.com/openparallax/openparallax/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockToolStream implements llm.ToolStreamReader with pre-programmed events.
// It supports multiple rounds: SendToolResults appends the next batch of events.
type mockToolStream struct {
	mu       sync.Mutex
	events   []llm.StreamEvent
	idx      int
	fullText string
	usage    llm.TokenUsage
	// nextRounds holds event batches queued for after SendToolResults calls.
	nextRounds [][]llm.StreamEvent
}

func (m *mockToolStream) Next() (llm.StreamEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.idx >= len(m.events) {
		return llm.StreamEvent{Type: llm.EventDone}, io.EOF
	}
	ev := m.events[m.idx]
	m.idx++
	if ev.Type == llm.EventTextDelta {
		m.fullText += ev.Text
	}
	return ev, nil
}

func (m *mockToolStream) SendToolResults(_ []llm.ToolResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.nextRounds) > 0 {
		m.events = m.nextRounds[0]
		m.nextRounds = m.nextRounds[1:]
		m.idx = 0
	}
	return nil
}

func (m *mockToolStream) SetTools(_ []llm.ToolDefinition) {}

func (m *mockToolStream) Close() error { return nil }

func (m *mockToolStream) FullText() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.fullText
}

func (m *mockToolStream) Usage() llm.TokenUsage {
	return m.usage
}

// mockProvider implements llm.Provider for testing.
type mockProvider struct {
	streamFn func(ctx context.Context, messages []llm.ChatMessage, tools []llm.ToolDefinition, opts ...llm.Option) (llm.ToolStreamReader, error)
}

func (m *mockProvider) Complete(_ context.Context, _ string, _ ...llm.Option) (string, error) {
	return "summary", nil
}

func (m *mockProvider) CompleteWithHistory(_ context.Context, _ []llm.ChatMessage, _ ...llm.Option) (string, error) {
	return "summary", nil
}

func (m *mockProvider) Stream(_ context.Context, _ string, _ ...llm.Option) (llm.StreamReader, error) {
	return nil, nil
}

func (m *mockProvider) StreamWithHistory(_ context.Context, _ []llm.ChatMessage, _ ...llm.Option) (llm.StreamReader, error) {
	return nil, nil
}

func (m *mockProvider) StreamWithTools(ctx context.Context, messages []llm.ChatMessage, tools []llm.ToolDefinition, opts ...llm.Option) (llm.ToolStreamReader, error) {
	return m.streamFn(ctx, messages, tools, opts...)
}

func (m *mockProvider) EstimateTokens(_ string) int { return 10 }
func (m *mockProvider) Name() string                { return "mock" }
func (m *mockProvider) Model() string               { return "mock-model" }
func (m *mockProvider) CheapestModel() string       { return "mock-cheap" }

// setupTestWorkspace creates a minimal workspace with IDENTITY.md.
func setupTestWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "IDENTITY.md"), []byte("Name: TestAgent"), 0o644))
	return dir
}

func TestRunLoopBasicResponse(t *testing.T) {
	workspace := setupTestWorkspace(t)
	agent := NewAgent(&mockProvider{}, workspace, nil, nil, nil)

	provider := &mockProvider{
		streamFn: func(_ context.Context, _ []llm.ChatMessage, _ []llm.ToolDefinition, _ ...llm.Option) (llm.ToolStreamReader, error) {
			return &mockToolStream{
				events: []llm.StreamEvent{
					{Type: llm.EventTextDelta, Text: "Hello, "},
					{Type: llm.EventTextDelta, Text: "world!"},
					{Type: llm.EventDone},
				},
				usage: llm.TokenUsage{InputTokens: 50, OutputTokens: 10},
			}, nil
		},
	}

	var events []LoopEvent
	var mu sync.Mutex
	emit := func(ev LoopEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}

	resultCh := make(chan ToolResult, 1)
	ctx := context.Background()

	RunLoop(ctx, LoopConfig{
		Provider:      provider,
		Agent:         agent,
		MaxRounds:     5,
		ContextWindow: 128000,
	}, "session-1", "msg-1", "Hi there", types.SessionNormal, nil, nil, emit, resultCh)

	mu.Lock()
	defer mu.Unlock()

	// Collect event types.
	var eventTypes []EventType
	for _, ev := range events {
		eventTypes = append(eventTypes, ev.Type)
	}

	// Must have token events and a complete event.
	assert.Contains(t, eventTypes, EventToken)
	assert.Contains(t, eventTypes, EventComplete)

	// Find the complete event and verify it has content.
	var complete *LoopEvent
	for i := range events {
		if events[i].Type == EventComplete {
			complete = &events[i]
			break
		}
	}
	require.NotNil(t, complete)
	assert.Equal(t, "Hello, world!", complete.Content)
	assert.NotNil(t, complete.Usage)
	assert.Equal(t, 50, complete.Usage.InputTokens)

	// No errors emitted.
	for _, ev := range events {
		assert.NotEqual(t, EventLoopError, ev.Type, "unexpected error: %s", ev.ErrorMessage)
	}
}

func TestRunLoopWithToolCall(t *testing.T) {
	workspace := setupTestWorkspace(t)
	agent := NewAgent(&mockProvider{}, workspace, nil, nil, nil)

	provider := &mockProvider{
		streamFn: func(_ context.Context, _ []llm.ChatMessage, _ []llm.ToolDefinition, _ ...llm.Option) (llm.ToolStreamReader, error) {
			return &mockToolStream{
				events: []llm.StreamEvent{
					{Type: llm.EventToolCallComplete, ToolCall: &llm.ToolCall{
						ID:        "call-1",
						Name:      "load_tools",
						Arguments: map[string]any{"groups": []any{"filesystem"}},
					}},
					{Type: llm.EventDone},
				},
				nextRounds: [][]llm.StreamEvent{
					{
						{Type: llm.EventTextDelta, Text: "Done loading tools."},
						{Type: llm.EventDone},
					},
				},
				usage: llm.TokenUsage{InputTokens: 100, OutputTokens: 20},
			}, nil
		},
	}

	var events []LoopEvent
	var mu sync.Mutex
	emit := func(ev LoopEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}

	resultCh := make(chan ToolResult, 1)
	ctx := context.Background()

	// Run in a goroutine since it will block waiting for resultCh.
	done := make(chan struct{})
	go func() {
		defer close(done)
		RunLoop(ctx, LoopConfig{
			Provider:      provider,
			Agent:         agent,
			MaxRounds:     5,
			ContextWindow: 128000,
		}, "session-1", "msg-1", "Load filesystem tools", types.SessionNormal, nil, nil, emit, resultCh)
	}()

	// Wait for the EventToolDefsRequest, then send back a result.
	waitForEvent := func(typ EventType) LoopEvent {
		t.Helper()
		for {
			mu.Lock()
			for _, ev := range events {
				if ev.Type == typ {
					mu.Unlock()
					return ev
				}
			}
			mu.Unlock()
		}
	}

	toolDefsReq := waitForEvent(EventToolDefsRequest)
	assert.Equal(t, []string{"filesystem"}, toolDefsReq.RequestedGroups)

	// Send the tool definitions result back.
	resultCh <- ToolResult{CallID: "call-1", Content: "Tools loaded: read_file, write_file"}

	// Wait for completion.
	<-done

	mu.Lock()
	defer mu.Unlock()

	var complete *LoopEvent
	for i := range events {
		if events[i].Type == EventComplete {
			complete = &events[i]
			break
		}
	}
	require.NotNil(t, complete)
	assert.Equal(t, "Done loading tools.", complete.Content)
}

func TestRunLoopWithToolProposal(t *testing.T) {
	workspace := setupTestWorkspace(t)
	agent := NewAgent(&mockProvider{}, workspace, nil, nil, nil)

	provider := &mockProvider{
		streamFn: func(_ context.Context, _ []llm.ChatMessage, _ []llm.ToolDefinition, _ ...llm.Option) (llm.ToolStreamReader, error) {
			return &mockToolStream{
				events: []llm.StreamEvent{
					{Type: llm.EventToolCallComplete, ToolCall: &llm.ToolCall{
						ID:        "call-2",
						Name:      "read_file",
						Arguments: map[string]any{"path": "/tmp/test.txt"},
					}},
					{Type: llm.EventDone},
				},
				nextRounds: [][]llm.StreamEvent{
					{
						{Type: llm.EventTextDelta, Text: "File contents read."},
						{Type: llm.EventDone},
					},
				},
				usage: llm.TokenUsage{InputTokens: 80, OutputTokens: 15},
			}, nil
		},
	}

	var events []LoopEvent
	var mu sync.Mutex
	emit := func(ev LoopEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}

	resultCh := make(chan ToolResult, 1)
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		RunLoop(ctx, LoopConfig{
			Provider:      provider,
			Agent:         agent,
			MaxRounds:     5,
			ContextWindow: 128000,
		}, "session-1", "msg-1", "Read the file", types.SessionNormal, nil, nil, emit, resultCh)
	}()

	// Wait for the tool proposal event.
	waitForEvent := func(typ EventType) LoopEvent {
		t.Helper()
		for {
			mu.Lock()
			for _, ev := range events {
				if ev.Type == typ {
					mu.Unlock()
					return ev
				}
			}
			mu.Unlock()
		}
	}

	proposal := waitForEvent(EventToolProposal)
	require.NotNil(t, proposal.Proposal)
	assert.Equal(t, "read_file", proposal.Proposal.ToolName)
	assert.Equal(t, "call-2", proposal.Proposal.CallID)
	assert.Contains(t, proposal.Proposal.ArgumentsJSON, "/tmp/test.txt")

	// Send back the tool execution result.
	resultCh <- ToolResult{CallID: "call-2", Content: "hello world"}

	<-done

	mu.Lock()
	defer mu.Unlock()

	var complete *LoopEvent
	for i := range events {
		if events[i].Type == EventComplete {
			complete = &events[i]
			break
		}
	}
	require.NotNil(t, complete)
	assert.Equal(t, "File contents read.", complete.Content)
}

func TestRunLoopMaxRounds(t *testing.T) {
	workspace := setupTestWorkspace(t)
	agent := NewAgent(&mockProvider{}, workspace, nil, nil, nil)

	// Provider always returns a tool call, never a text-only response.
	// Build enough rounds so MaxRounds=3 is the limiting factor.
	toolCallBatch := []llm.StreamEvent{
		{Type: llm.EventToolCallComplete, ToolCall: &llm.ToolCall{
			ID:        "call-loop",
			Name:      "read_file",
			Arguments: map[string]any{"path": "/tmp/test.txt"},
		}},
		{Type: llm.EventDone},
	}
	provider := &mockProvider{
		streamFn: func(_ context.Context, _ []llm.ChatMessage, _ []llm.ToolDefinition, _ ...llm.Option) (llm.ToolStreamReader, error) {
			return &mockToolStream{
				events: toolCallBatch,
				nextRounds: [][]llm.StreamEvent{
					toolCallBatch, toolCallBatch, toolCallBatch, toolCallBatch,
				},
				usage: llm.TokenUsage{InputTokens: 10, OutputTokens: 5},
			}, nil
		},
	}

	var events []LoopEvent
	var mu sync.Mutex
	emit := func(ev LoopEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}

	resultCh := make(chan ToolResult, 10)
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		RunLoop(ctx, LoopConfig{
			Provider:      provider,
			Agent:         agent,
			MaxRounds:     3,
			ContextWindow: 128000,
		}, "session-1", "msg-1", "Keep going", types.SessionNormal, nil, nil, emit, resultCh)
	}()

	// Feed results for each tool proposal until the loop finishes.
	for {
		select {
		case <-done:
			goto verify
		case resultCh <- ToolResult{CallID: "call-loop", Content: "result"}:
		}
	}

verify:
	mu.Lock()
	defer mu.Unlock()

	// The loop must terminate. Count proposal events.
	proposalCount := 0
	for _, ev := range events {
		if ev.Type == EventToolProposal {
			proposalCount++
		}
	}

	// With MaxRounds=3, we get at most 3 rounds of tool calls.
	assert.LessOrEqual(t, proposalCount, 3, "loop should terminate within MaxRounds")

	// Must still emit EventComplete even after hitting max rounds.
	var hasComplete bool
	for _, ev := range events {
		if ev.Type == EventComplete {
			hasComplete = true
			break
		}
	}
	assert.True(t, hasComplete, "EventComplete must be emitted even after max rounds")
}

func TestRunLoopContextCancellation(t *testing.T) {
	workspace := setupTestWorkspace(t)
	agent := NewAgent(&mockProvider{}, workspace, nil, nil, nil)

	provider := &mockProvider{
		streamFn: func(_ context.Context, _ []llm.ChatMessage, _ []llm.ToolDefinition, _ ...llm.Option) (llm.ToolStreamReader, error) {
			return &mockToolStream{
				events: []llm.StreamEvent{
					{Type: llm.EventToolCallComplete, ToolCall: &llm.ToolCall{
						ID:        "call-cancel",
						Name:      "read_file",
						Arguments: map[string]any{"path": "/tmp/x"},
					}},
					{Type: llm.EventDone},
				},
			}, nil
		},
	}

	var events []LoopEvent
	var mu sync.Mutex
	emit := func(ev LoopEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}

	resultCh := make(chan ToolResult)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		RunLoop(ctx, LoopConfig{
			Provider:      provider,
			Agent:         agent,
			MaxRounds:     10,
			ContextWindow: 128000,
		}, "session-1", "msg-1", "Do something", types.SessionNormal, nil, nil, emit, resultCh)
	}()

	// Wait for the proposal, then cancel context instead of sending a result.
	waitForProposal := func() {
		t.Helper()
		for {
			mu.Lock()
			for _, ev := range events {
				if ev.Type == EventToolProposal {
					mu.Unlock()
					return
				}
			}
			mu.Unlock()
		}
	}

	waitForProposal()
	cancel()

	// The loop must exit promptly.
	<-done
}

func TestRunLoopLLMError(t *testing.T) {
	workspace := setupTestWorkspace(t)
	agent := NewAgent(&mockProvider{}, workspace, nil, nil, nil)

	provider := &mockProvider{
		streamFn: func(_ context.Context, _ []llm.ChatMessage, _ []llm.ToolDefinition, _ ...llm.Option) (llm.ToolStreamReader, error) {
			return nil, assert.AnError
		},
	}

	var events []LoopEvent
	var mu sync.Mutex
	emit := func(ev LoopEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}

	resultCh := make(chan ToolResult, 1)
	ctx := context.Background()

	RunLoop(ctx, LoopConfig{
		Provider:      provider,
		Agent:         agent,
		MaxRounds:     5,
		ContextWindow: 128000,
	}, "session-1", "msg-1", "Hello", types.SessionNormal, nil, nil, emit, resultCh)

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, events, 1)
	assert.Equal(t, EventLoopError, events[0].Type)
	assert.Equal(t, "LLM_CALL_FAILED", events[0].ErrorCode)
	assert.NotEmpty(t, events[0].ErrorMessage)
}

// TestToolDefsToLLM_PreservesParameters guards against the regression where
// the gRPC ToolDef → llm.ToolDefinition conversion silently dropped the
// parameters_json field. Without parameters, every tool reaches the OpenAI
// SDK as a function with no input schema, the marshaler elides it as
// zero-value, and upstream Anthropic / OpenAI-compatible proxies reject the
// request with "input_schema: Field required".
func TestToolDefsToLLM_PreservesParameters(t *testing.T) {
	defs := []*pb.ToolDef{
		{
			Name:           "load_tools",
			Description:    "meta-tool",
			ParametersJson: `{"type":"object","properties":{"groups":{"type":"array","items":{"type":"string"}}},"required":["groups"]}`,
		},
		{
			Name:           "no_args",
			Description:    "tool without parameters json",
			ParametersJson: "",
		},
	}

	tools := ToolDefsToLLM(defs)
	require.Len(t, tools, 2)

	require.NotNil(t, tools[0].Parameters, "parameters_json must be unmarshaled into a non-nil schema map")
	assert.Equal(t, "object", tools[0].Parameters["type"])
	props, ok := tools[0].Parameters["properties"].(map[string]any)
	require.True(t, ok, "properties must round-trip as map[string]any")
	assert.Contains(t, props, "groups")
	required, ok := tools[0].Parameters["required"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"groups"}, required)

	// Empty parameters_json is allowed and produces a tool with no schema
	// (the LLM provider layer can decide whether to omit or substitute).
	assert.Nil(t, tools[1].Parameters)
}
