package agent

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/openparallax/openparallax/llm"
)

const defaultStalenessThreshold = 4

// SummarizeStaleToolResults replaces full tool results older than
// stalenessTurns with compact summaries. A "turn" is a user→assistant
// cycle. The transformation is view-only — original messages in storage
// are not modified.
func SummarizeStaleToolResults(messages []llm.ChatMessage, currentTurn int, stalenessTurns int) []llm.ChatMessage {
	if stalenessTurns <= 0 {
		stalenessTurns = defaultStalenessThreshold
	}

	// Count turns: each user message increments the counter.
	turnNumber := 0
	turnMap := make(map[int]int) // message index → turn number
	for i, m := range messages {
		if m.Role == "user" {
			turnNumber++
		}
		turnMap[i] = turnNumber
	}

	result := make([]llm.ChatMessage, len(messages))
	for i, m := range messages {
		result[i] = m
		if m.Role != "tool" || m.ToolCallID == "" {
			continue
		}
		msgTurn := turnMap[i]
		if currentTurn-msgTurn < stalenessTurns {
			continue
		}
		// Already summarized — skip.
		if strings.HasPrefix(m.Content, "[Summary:") {
			continue
		}
		result[i].Content = summarizeToolResult(m)
	}
	return result
}

// summarizeToolResult generates a deterministic summary of a tool result.
func summarizeToolResult(m llm.ChatMessage) string {
	content := m.Content
	byteCount := len(content)
	lineCount := strings.Count(content, "\n") + 1

	// Detect tool name from context. Tool results have ToolCallID set
	// but not the tool name. Infer from content patterns.
	contentType := inferContentType(content)

	if strings.HasPrefix(content, "Loaded tool groups:") || strings.HasPrefix(content, "Loaded ") {
		// load_tools result — compact version.
		first := strings.SplitN(content, "\n", 2)[0]
		return fmt.Sprintf("[Summary: %s]", first)
	}

	if strings.HasPrefix(content, "Error:") || strings.HasPrefix(content, "error:") {
		first := strings.SplitN(content, "\n", 2)[0]
		if len(first) > 100 {
			first = first[:100]
		}
		return fmt.Sprintf("[Summary: %s]", first)
	}

	return fmt.Sprintf("[Summary: Returned %d bytes (%d lines) of %s]",
		byteCount, lineCount, contentType)
}

// inferContentType guesses the content type from the tool result.
func inferContentType(content string) string {
	lower := strings.ToLower(content)

	if strings.HasPrefix(content, "<!DOCTYPE") || strings.HasPrefix(content, "<html") {
		return "HTML"
	}
	if strings.Contains(lower, "package ") && strings.Contains(lower, "func ") {
		return "Go source code"
	}
	if strings.Contains(lower, "def ") && strings.Contains(lower, "import ") {
		return "Python source code"
	}
	if strings.HasPrefix(content, "{") || strings.HasPrefix(content, "[") {
		return "JSON data"
	}
	if strings.HasPrefix(content, "# ") || strings.Contains(content, "\n## ") {
		return "Markdown"
	}

	// Check if it looks like command output (exit codes, common patterns).
	if strings.Contains(lower, "exit code") || strings.Contains(lower, "exit status") {
		return "command output"
	}

	return "text content"
}

// InferContentTypeFromPath returns a content type hint based on file extension.
func InferContentTypeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "Go source code"
	case ".py":
		return "Python source code"
	case ".js", ".ts":
		return "JavaScript/TypeScript"
	case ".html", ".htm":
		return "HTML"
	case ".css":
		return "CSS"
	case ".json":
		return "JSON"
	case ".yaml", ".yml":
		return "YAML"
	case ".md":
		return "Markdown"
	case ".sh", ".bash":
		return "shell script"
	case ".sql":
		return "SQL"
	case ".svg":
		return "SVG"
	default:
		return "text content"
	}
}
