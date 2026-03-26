package agent

import (
	"context"
	"os"
	"testing"

	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuilderParsesSingleAction(t *testing.T) {
	b := NewActionBuilder()
	plan := `ACTION: read_file
PARAMS: {"path": "/tmp/test.txt"}
REASONING: User wants to read a file.`

	actions, err := b.Build(plan)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	assert.Equal(t, types.ActionReadFile, actions[0].Type)
	assert.Equal(t, "/tmp/test.txt", actions[0].Payload["path"])
}

func TestBuilderParsesMultipleActions(t *testing.T) {
	b := NewActionBuilder()
	plan := `ACTION: read_file
PARAMS: {"path": "/tmp/a.txt"}
REASONING: Read first file.

ACTION: read_file
PARAMS: {"path": "/tmp/b.txt"}
REASONING: Read second file.`

	actions, err := b.Build(plan)
	require.NoError(t, err)
	assert.Len(t, actions, 2)
}

func TestBuilderHandlesNone(t *testing.T) {
	b := NewActionBuilder()
	plan := `ACTION: none
REASONING: This is a conversation, no tool use needed.`

	actions, err := b.Build(plan)
	require.NoError(t, err)
	assert.Nil(t, actions)
}

func TestBuilderSkipsMalformedBlocks(t *testing.T) {
	b := NewActionBuilder()
	plan := `ACTION: read_file
PARAMS: {"path": "/tmp/valid.txt"}
REASONING: Valid action.

This is just random text with no ACTION marker that should be ignored.

ACTION: write_file
PARAMS: not valid json
REASONING: Has bad params but still parses with raw fallback.`

	actions, err := b.Build(plan)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(actions), 1, "should parse at least the valid action")
}

func TestBuilderAssignsUniqueIDs(t *testing.T) {
	b := NewActionBuilder()
	plan := `ACTION: read_file
PARAMS: {"path": "/tmp/a.txt"}
REASONING: First.

ACTION: read_file
PARAMS: {"path": "/tmp/b.txt"}
REASONING: Second.`

	actions, err := b.Build(plan)
	require.NoError(t, err)
	require.Len(t, actions, 2)
	assert.NotEqual(t, actions[0].RequestID, actions[1].RequestID)
	assert.NotEmpty(t, actions[0].RequestID)
}

func TestBuilderComputesCorrectHash(t *testing.T) {
	b := NewActionBuilder()
	plan := `ACTION: read_file
PARAMS: {"path": "/tmp/test.txt"}
REASONING: Read a file.`

	actions, err := b.Build(plan)
	require.NoError(t, err)
	require.Len(t, actions, 1)

	expected, err := crypto.HashAction("read_file", map[string]any{"path": "/tmp/test.txt"})
	require.NoError(t, err)
	assert.Equal(t, expected, actions[0].Hash)
}

func TestPlannerProducesPlan(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	model := os.Getenv("OPENAI_MODEL")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	provider, err := llm.NewOpenAIProvider(apiKey, model, baseURL)
	require.NoError(t, err)

	p := NewPlanner(provider, []types.ActionType{types.ActionReadFile, types.ActionWriteFile, types.ActionExecCommand})
	intent := &types.StructuredIntent{
		Goal:          types.GoalFileManagement,
		PrimaryAction: types.ActionReadFile,
		RawInput:      "read my SOUL.md",
		Confidence:    0.9,
	}

	plan, err := p.Plan(context.Background(), intent, "You are a helpful assistant.", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, plan)
	assert.Contains(t, plan, "ACTION:")
}

func TestSelfEvaluatorPassesBenign(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	model := os.Getenv("OPENAI_MODEL")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	provider, err := llm.NewOpenAIProvider(apiKey, model, baseURL)
	require.NoError(t, err)

	se := NewSelfEvaluator(provider)
	actions := []*types.ActionRequest{
		{Type: types.ActionReadFile, Payload: map[string]any{"path": "SOUL.md"}},
	}

	passed, _, evalErr := se.Evaluate(context.Background(), actions, "read my SOUL.md")
	require.NoError(t, evalErr)
	assert.True(t, passed)
}
