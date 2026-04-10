package executors

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubManager is a minimal SubAgentManagerInterface that records the
// last Create call so tests can assert what model was resolved.
type stubManager struct {
	lastReq SubAgentRequest
	name    string
	err     error
}

func (s *stubManager) Create(req SubAgentRequest) (string, error) {
	s.lastReq = req
	if s.err != nil {
		return "", s.err
	}
	if s.name == "" {
		return "agent-1", nil
	}
	return s.name, nil
}
func (s *stubManager) Status(string) (SubAgentInfo, error)          { return SubAgentInfo{}, nil }
func (s *stubManager) Result(string, time.Duration) (string, error) { return "", nil }
func (s *stubManager) SendMessage(string, string) error             { return nil }
func (s *stubManager) Delete(string) error                          { return nil }
func (s *stubManager) List() []SubAgentInfo                         { return nil }

func TestRenderModelMenu(t *testing.T) {
	cases := []struct {
		name   string
		models []types.ModelEntry
		expect []string // substrings the rendered menu must contain
		empty  bool     // when true, the menu must be ""
	}{
		{
			name:   "empty pool yields no menu",
			models: nil,
			empty:  true,
		},
		{
			name: "single entry yields no menu",
			models: []types.ModelEntry{
				{Name: "chat", Model: "claude-sonnet-4-6"},
			},
			empty: true,
		},
		{
			name: "two entries with mixed purpose",
			models: []types.ModelEntry{
				{Name: "fast", Model: "claude-haiku-4-5", Purpose: "fast, cheap, scans"},
				{Name: "smart", Model: "claude-sonnet-4-6"},
			},
			expect: []string{
				"1. claude-haiku-4-5 — fast, cheap, scans",
				"2. claude-sonnet-4-6",
				"you are the judge",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := NewSubAgentExecutor(&stubManager{}, tc.models)
			got := e.renderModelMenu()
			if tc.empty {
				assert.Empty(t, got)
				return
			}
			for _, want := range tc.expect {
				assert.Contains(t, got, want)
			}
			// Entry with no Purpose must not produce a trailing " — ".
			assert.NotContains(t, got, "claude-sonnet-4-6 — ")
		})
	}
}

func TestCreateAgentToolSchemaModelParam(t *testing.T) {
	t.Run("with pool exposes integer index", func(t *testing.T) {
		e := NewSubAgentExecutor(&stubManager{}, []types.ModelEntry{
			{Model: "a"}, {Model: "b"},
		})
		schema := findSchema(t, e.ToolSchemas(), "create_agent")
		props := schema.Parameters["properties"].(map[string]any)
		modelParam := props["model"].(map[string]any)
		assert.Equal(t, "integer", modelParam["type"])
		assert.Contains(t, modelParam["description"], "1-based index")
		assert.Contains(t, schema.Description, "Available sub-agent models")
	})

	t.Run("empty pool tells the LLM to omit", func(t *testing.T) {
		e := NewSubAgentExecutor(&stubManager{}, nil)
		schema := findSchema(t, e.ToolSchemas(), "create_agent")
		props := schema.Parameters["properties"].(map[string]any)
		modelParam := props["model"].(map[string]any)
		assert.Contains(t, modelParam["description"], "omit this field")
		assert.NotContains(t, schema.Description, "Available sub-agent models")
	})
}

func TestExecuteCreateResolvesModelIndex(t *testing.T) {
	models := []types.ModelEntry{
		{Model: "claude-haiku-4-5"},
		{Model: "claude-sonnet-4-6"},
		{Model: "gpt-5.4-mini"},
	}

	t.Run("valid index resolves to model name", func(t *testing.T) {
		mgr := &stubManager{}
		e := NewSubAgentExecutor(mgr, models)
		res := e.Execute(context.Background(), &types.ActionRequest{
			Type: types.ActionCreateAgent,
			Payload: map[string]any{
				"task":  "do the thing",
				"model": float64(2), // JSON numbers decode as float64
			},
		})
		require.True(t, res.Success, "expected success, got error: %s", res.Error)
		assert.Equal(t, "claude-sonnet-4-6", mgr.lastReq.Model)
	})

	t.Run("omitted model leaves request empty", func(t *testing.T) {
		mgr := &stubManager{}
		e := NewSubAgentExecutor(mgr, models)
		res := e.Execute(context.Background(), &types.ActionRequest{
			Type:    types.ActionCreateAgent,
			Payload: map[string]any{"task": "do the thing"},
		})
		require.True(t, res.Success)
		assert.Empty(t, mgr.lastReq.Model, "engine fallback should be used")
	})

	t.Run("out of range returns graceful error", func(t *testing.T) {
		mgr := &stubManager{}
		e := NewSubAgentExecutor(mgr, models)
		res := e.Execute(context.Background(), &types.ActionRequest{
			Type: types.ActionCreateAgent,
			Payload: map[string]any{
				"task":  "do the thing",
				"model": float64(99),
			},
		})
		assert.False(t, res.Success)
		assert.Contains(t, res.Error, "out of range")
		assert.Contains(t, res.Error, "3 entries")
	})

	t.Run("zero index is treated as omit", func(t *testing.T) {
		mgr := &stubManager{}
		e := NewSubAgentExecutor(mgr, models)
		res := e.Execute(context.Background(), &types.ActionRequest{
			Type: types.ActionCreateAgent,
			Payload: map[string]any{
				"task":  "do the thing",
				"model": float64(0),
			},
		})
		require.True(t, res.Success)
		assert.Empty(t, mgr.lastReq.Model)
	})
}

func findSchema(t *testing.T, schemas []ToolSchema, name string) ToolSchema {
	t.Helper()
	for _, s := range schemas {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("schema %q not found", name)
	return ToolSchema{}
}

// Compile-time assertion that stubManager satisfies the interface so
// the test fails to build if the interface drifts.
var _ SubAgentManagerInterface = (*stubManager)(nil)

func TestRenderedMenuStableForGoldenString(t *testing.T) {
	models := []types.ModelEntry{
		{Model: "claude-haiku-4-5", Purpose: "fast, cheap"},
		{Model: "claude-sonnet-4-6", Purpose: "balanced reasoning"},
	}
	e := NewSubAgentExecutor(&stubManager{}, models)
	got := e.renderModelMenu()
	assert.True(t, strings.HasPrefix(got, "Available sub-agent models"))
	assert.Contains(t, got, "1. claude-haiku-4-5 — fast, cheap")
	assert.Contains(t, got, "2. claude-sonnet-4-6 — balanced reasoning")
}
