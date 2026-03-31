package agent

import (
	"testing"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/stretchr/testify/assert"
)

func TestSummarizeStaleToolResults(t *testing.T) {
	messages := []llm.ChatMessage{
		{Role: "user", Content: "read my file"},                                      // turn 1
		{Role: "tool", Content: "package main\nfunc main() {}\n", ToolCallID: "tc1"}, // stale
		{Role: "assistant", Content: "here it is"},
		{Role: "user", Content: "now edit it"}, // turn 2
		{Role: "tool", Content: "wrote file", ToolCallID: "tc2"},
		{Role: "assistant", Content: "done"},
		{Role: "user", Content: "check status"}, // turn 3
		{Role: "assistant", Content: "all good"},
		{Role: "user", Content: "another question"}, // turn 4
		{Role: "assistant", Content: "sure"},
		{Role: "user", Content: "one more"},                         // turn 5
		{Role: "tool", Content: "recent result", ToolCallID: "tc3"}, // fresh
		{Role: "assistant", Content: "here"},
	}

	result := SummarizeStaleToolResults(messages, 5, 4)

	// Turn 1 tool result (tc1) should be summarized (5-1 >= 4).
	assert.Contains(t, result[1].Content, "[Summary:")
	// Turn 2 tool result (tc2) should be summarized (5-2 >= 4, since turn 2 is 3 turns ago... actually 5-2=3 < 4).
	// tc2 is at turn 2, current is 5, diff=3 < 4, so NOT summarized.
	assert.Equal(t, "wrote file", result[4].Content)
	// Turn 5 tool result (tc3) should be fresh.
	assert.Equal(t, "recent result", result[11].Content)
}

func TestSummarizeAlreadySummarized(t *testing.T) {
	messages := []llm.ChatMessage{
		{Role: "user", Content: "q1"},
		{Role: "tool", Content: "[Summary: already done]", ToolCallID: "tc1"},
		{Role: "assistant", Content: "ok"},
	}

	result := SummarizeStaleToolResults(messages, 10, 1)
	assert.Equal(t, "[Summary: already done]", result[1].Content)
}

func TestSummarizeErrorResult(t *testing.T) {
	m := llm.ChatMessage{Role: "tool", Content: "Error: file not found", ToolCallID: "tc1"}
	summary := summarizeToolResult(m)
	assert.Contains(t, summary, "Error: file not found")
}

func TestSummarizeLoadToolsResult(t *testing.T) {
	m := llm.ChatMessage{Role: "tool", Content: "Loaded tool groups: files, git (13 tools)\n  files: 5 tools\n  git: 8 tools", ToolCallID: "tc1"}
	summary := summarizeToolResult(m)
	assert.Contains(t, summary, "Loaded tool groups")
}

func TestSummarizeGoSourceCode(t *testing.T) {
	m := llm.ChatMessage{Role: "tool", Content: "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n", ToolCallID: "tc1"}
	summary := summarizeToolResult(m)
	assert.Contains(t, summary, "Go source code")
}

func TestInferContentTypeFromPath(t *testing.T) {
	assert.Equal(t, "Go source code", InferContentTypeFromPath("main.go"))
	assert.Equal(t, "Python source code", InferContentTypeFromPath("script.py"))
	assert.Equal(t, "JSON", InferContentTypeFromPath("data.json"))
	assert.Equal(t, "Markdown", InferContentTypeFromPath("README.md"))
	assert.Equal(t, "text content", InferContentTypeFromPath("unknown.xyz"))
}
