//go:build e2e

package e2e

import "strings"

// registerDefaults adds the standard mock response patterns.
//
// Pattern lifecycle for a typical tool-call test:
//
//  1. User sends message → assistantCount==0 → load_tools fires
//  2. load_tools result arrives → assistantCount==1, hasToolResult → action pattern fires
//  3. Action result arrives → assistantCount==2, hasToolResult → "Done" fallback fires
//
// Action patterns only match on the second turn (assistantCount==1, i.e. right
// after load_tools). This prevents infinite loops where the mock keeps returning
// the same tool call after every error/success result.
func (m *MockLLMServer) registerDefaults() {
	// First message with tools available: call load_tools.
	m.patterns = append(m.patterns, mockPattern{
		match: func(messages []chatMessage, hasTools bool) bool {
			if !hasTools {
				return false
			}
			msg := lastUserMessage(messages)
			return countAssistant(messages) == 0 && msg != ""
		},
		response: MockResponse{ToolCalls: []MockToolCall{
			{Name: "load_tools", Arguments: map[string]any{"groups": []string{"files", "shell", "git", "memory"}}},
		}},
	})

	// Sensitive file read (blocked by Shield policy).
	// Matches on second turn (after load_tools).
	m.patterns = append(m.patterns, mockPattern{
		match: func(messages []chatMessage, _ bool) bool {
			if countAssistant(messages) != 1 {
				return false
			}
			msg := strings.ToLower(lastUserMessage(messages))
			return strings.Contains(msg, "/etc/shadow")
		},
		response: MockResponse{ToolCalls: []MockToolCall{
			{Name: "read_file", Arguments: map[string]any{"path": "/etc/shadow"}},
		}},
	})

	// File read.
	m.patterns = append(m.patterns, mockPattern{
		match: func(messages []chatMessage, _ bool) bool {
			if countAssistant(messages) != 1 {
				return false
			}
			msg := strings.ToLower(lastUserMessage(messages))
			return strings.Contains(msg, "read") && strings.Contains(msg, "file")
		},
		response: MockResponse{ToolCalls: []MockToolCall{
			{Name: "read_file", Arguments: map[string]any{"path": "e2e-readme.md"}},
		}},
	})

	// File write.
	m.patterns = append(m.patterns, mockPattern{
		match: func(messages []chatMessage, _ bool) bool {
			if countAssistant(messages) != 1 {
				return false
			}
			msg := strings.ToLower(lastUserMessage(messages))
			return strings.Contains(msg, "write") && strings.Contains(msg, "test.txt")
		},
		response: MockResponse{ToolCalls: []MockToolCall{
			{Name: "write_file", Arguments: map[string]any{"path": "test.txt", "content": "hello from e2e test"}},
		}},
	})

	// Memory write.
	m.patterns = append(m.patterns, mockPattern{
		match: func(messages []chatMessage, _ bool) bool {
			if countAssistant(messages) != 1 {
				return false
			}
			msg := strings.ToLower(lastUserMessage(messages))
			return strings.Contains(msg, "write to memory") || strings.Contains(msg, "remember")
		},
		response: MockResponse{ToolCalls: []MockToolCall{
			{Name: "memory_write", Arguments: map[string]any{"content": "E2E test fact: the sky is blue", "category": "fact"}},
		}},
	})

	// Sub-agent delegation.
	m.patterns = append(m.patterns, mockPattern{
		match: func(messages []chatMessage, _ bool) bool {
			if countAssistant(messages) != 1 {
				return false
			}
			msg := strings.ToLower(lastUserMessage(messages))
			return strings.Contains(msg, "delegate") || strings.Contains(msg, "sub-agent") || strings.Contains(msg, "create a sub")
		},
		response: MockResponse{ToolCalls: []MockToolCall{
			{Name: "create_agent", Arguments: map[string]any{"task": "research the project structure", "tools": []string{"files"}}},
		}},
	})

	// List files / directory.
	m.patterns = append(m.patterns, mockPattern{
		match: func(messages []chatMessage, _ bool) bool {
			if countAssistant(messages) != 1 {
				return false
			}
			msg := strings.ToLower(lastUserMessage(messages))
			return strings.Contains(msg, "list files") || strings.Contains(msg, "directory")
		},
		response: MockResponse{ToolCalls: []MockToolCall{
			{Name: "list_directory", Arguments: map[string]any{"path": "."}},
		}},
	})

	// Fallback: after any tool result, return a summary.
	m.patterns = append(m.patterns, mockPattern{
		match: func(messages []chatMessage, _ bool) bool {
			return hasToolResult(messages)
		},
		response: MockResponse{Text: "Done. The tool call completed successfully."},
	})
}

// countAssistant returns the number of assistant messages in the conversation.
func countAssistant(messages []chatMessage) int {
	n := 0
	for _, m := range messages {
		if m.Role == "assistant" {
			n++
		}
	}
	return n
}
