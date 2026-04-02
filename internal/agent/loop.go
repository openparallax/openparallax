package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/types"
	pb "github.com/openparallax/openparallax/internal/types/pb"
)

// LoopConfig configures the reasoning loop.
type LoopConfig struct {
	Provider      llm.Provider
	Agent         *Agent
	MaxRounds     int
	ContextWindow int
}

// ToolProposal is a tool call the LLM wants to make.
type ToolProposal struct {
	CallID        string
	ToolName      string
	ArgumentsJSON string
}

// ToolResult is the engine's response to a tool proposal.
type ToolResult struct {
	CallID  string
	Content string
	IsError bool
}

// LoopEvent is an event emitted by the reasoning loop.
type LoopEvent struct {
	Type EventType

	// LLMToken
	Token string

	// ToolProposal
	Proposal *ToolProposal

	// ToolDefsRequest (load_tools)
	RequestedGroups []string

	// MemoryFlush (compaction facts)
	FlushContent string

	// ResponseComplete
	Content  string
	Thoughts []types.Thought

	// Error
	ErrorCode    string
	ErrorMessage string
}

// EventType identifies a loop event.
type EventType int

const (
	// EventToken is a streaming LLM token.
	EventToken EventType = iota
	// EventToolProposal is a tool call the LLM wants to make.
	EventToolProposal
	// EventToolDefsRequest asks the engine for tool definitions.
	EventToolDefsRequest
	// EventMemoryFlush sends compaction facts to the engine.
	EventMemoryFlush
	// EventComplete signals the response is finished.
	EventComplete
	// EventLoopError signals an error in the loop.
	EventLoopError
)

// RunLoop executes the LLM reasoning loop for a single message. It reads
// context from the workspace directly (read-only) and streams events back
// through the callback. Tool calls are proposed via the callback; the caller
// must deliver results via the returned channel.
//
// The flow:
//  1. Load history from DB, assemble context from workspace files
//  2. Compact if needed (flushes facts via EventMemoryFlush)
//  3. Stream LLM with tools, emit tokens via EventToken
//  4. For each tool call: emit EventToolProposal, wait for result on resultCh
//  5. Feed result back to LLM, continue loop
//  6. Emit EventComplete when done
func RunLoop(
	ctx context.Context,
	cfg LoopConfig,
	sessionID, messageID, content string,
	mode types.SessionMode,
	history []llm.ChatMessage,
	tools []llm.ToolDefinition,
	emit func(LoopEvent),
	resultCh <-chan ToolResult,
) {
	_ = mode // OTR filtering happens at the Engine level, not in the loop.

	// Build system prompt.
	skillSummary := ""
	activeSkills := ""
	if cfg.Agent.Skills != nil {
		skillSummary = cfg.Agent.Skills.LightSummary()
		activeSkills = cfg.Agent.Skills.ActiveSkillBodies()
	}
	systemPrompt, err := cfg.Agent.Context.AssembleWithSkills(mode, skillSummary, activeSkills)
	if err != nil {
		emit(LoopEvent{Type: EventLoopError, ErrorCode: "CONTEXT_FAILED", ErrorMessage: err.Error()})
		return
	}

	// Summarize stale tool results.
	turnCount := 0
	for _, m := range history {
		if m.Role == "user" {
			turnCount++
		}
	}
	history = SummarizeStaleToolResults(history, turnCount, 4)

	// Compact history if approaching context limits.
	systemTokens := cfg.Provider.EstimateTokens(systemPrompt)
	contextWindow := cfg.ContextWindow
	if contextWindow <= 0 {
		contextWindow = 128000
	}
	contextBudget := contextWindow - systemTokens - 4096
	if contextBudget < 4096 {
		contextBudget = 4096
	}

	historyTokens := 0
	for _, m := range history {
		historyTokens += cfg.Provider.EstimateTokens(m.Content)
	}
	usagePercent := float64(historyTokens) / float64(contextBudget) * 100

	if usagePercent >= 70 {
		// Compact in-memory. The compactor may flush facts — we intercept
		// that by using a memory-flush callback instead of direct writes.
		history, _ = cfg.Agent.CompactHistory(ctx, history, contextBudget)
	}

	// Build messages.
	messages := make([]llm.ChatMessage, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, llm.ChatMessage{Role: "user", Content: content})

	maxRounds := cfg.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 25
	}

	// Start LLM stream.
	toolStream, err := cfg.Provider.StreamWithTools(ctx, messages, tools,
		llm.WithSystem(systemPrompt), llm.WithMaxTokens(4096))
	if err != nil {
		emit(LoopEvent{Type: EventLoopError, ErrorCode: "LLM_CALL_FAILED", ErrorMessage: err.Error()})
		return
	}
	defer func() { _ = toolStream.Close() }()

	var toolResults []llm.ToolResult
	var thoughts []types.Thought
	var reasoningBuf strings.Builder
	rounds := 0

	for rounds < maxRounds {
		event, eventErr := toolStream.Next()
		if eventErr == io.EOF || event.Type == llm.EventDone {
			if len(toolResults) > 0 {
				if reasoningBuf.Len() > 0 {
					thoughts = append(thoughts, types.Thought{
						Stage:   "reasoning",
						Summary: strings.TrimSpace(reasoningBuf.String()),
					})
					reasoningBuf.Reset()
				}
				if sendErr := toolStream.SendToolResults(toolResults); sendErr != nil {
					break
				}
				toolResults = nil
				rounds++
				continue
			}
			break
		}
		if eventErr != nil {
			emit(LoopEvent{Type: EventLoopError, ErrorCode: "STREAM_ERROR", ErrorMessage: eventErr.Error()})
			break
		}

		switch event.Type {
		case llm.EventTextDelta:
			emit(LoopEvent{Type: EventToken, Token: event.Text})
			reasoningBuf.WriteString(event.Text)

		case llm.EventToolCallComplete:
			if reasoningBuf.Len() > 0 {
				thoughts = append(thoughts, types.Thought{
					Stage:   "reasoning",
					Summary: strings.TrimSpace(reasoningBuf.String()),
				})
				reasoningBuf.Reset()
			}
			tc := event.ToolCall

			// Handle load_tools meta-tool locally.
			if tc.Name == "load_tools" {
				groupNames, _ := tc.Arguments["groups"].([]any)
				names := make([]string, 0, len(groupNames))
				for _, g := range groupNames {
					if s, ok := g.(string); ok {
						names = append(names, s)
					}
				}
				emit(LoopEvent{Type: EventToolDefsRequest, RequestedGroups: names})

				// Wait for tool definitions from Engine (delivered as a ToolResult
				// containing the tool summary text).
				select {
				case result := <-resultCh:
					toolResults = append(toolResults, llm.ToolResult{CallID: tc.ID, Content: result.Content})
					thoughts = append(thoughts, types.Thought{
						Stage:   "tool_call",
						Summary: fmt.Sprintf("load_tools(%s)", strings.Join(names, ", ")),
						Detail:  map[string]any{"tool_name": "load_tools", "success": true},
					})
				case <-ctx.Done():
					return
				}
				continue
			}

			// Propose tool call to Engine.
			argsJSON := formatArgsJSON(tc.Arguments)
			emit(LoopEvent{
				Type: EventToolProposal,
				Proposal: &ToolProposal{
					CallID:        tc.ID,
					ToolName:      tc.Name,
					ArgumentsJSON: argsJSON,
				},
			})

			// Wait for Engine to evaluate + execute + return result.
			select {
			case result := <-resultCh:
				toolResults = append(toolResults, llm.ToolResult{
					CallID:  result.CallID,
					Content: result.Content,
					IsError: result.IsError,
				})
				thoughts = append(thoughts, types.Thought{
					Stage:   "tool_call",
					Summary: fmt.Sprintf("%s → %s", tc.Name, truncateResult(result.Content)),
					Detail: map[string]any{
						"tool_name": tc.Name,
						"success":   !result.IsError,
					},
				})

				if cfg.Agent.Skills != nil {
					cfg.Agent.Skills.MatchSkills([]string{tc.Name})
				}
			case <-ctx.Done():
				return
			}
		}
	}

	fullResponse := toolStream.FullText()

	emit(LoopEvent{
		Type:     EventComplete,
		Content:  fullResponse,
		Thoughts: thoughts,
	})
}

// formatArgsJSON serializes tool call arguments to a JSON string.
func formatArgsJSON(args map[string]any) string {
	data, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func truncateResult(s string) string {
	if len(s) > 80 {
		return s[:80] + "..."
	}
	return s
}

// ToolDefsToLLM converts proto tool definitions to LLM tool definitions.
func ToolDefsToLLM(defs []*pb.ToolDef) []llm.ToolDefinition {
	result := make([]llm.ToolDefinition, 0, len(defs))
	for _, d := range defs {
		result = append(result, llm.ToolDefinition{
			Name:        d.Name,
			Description: d.Description,
		})
	}
	return result
}
