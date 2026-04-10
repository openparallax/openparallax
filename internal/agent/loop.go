package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/openparallax/openparallax/internal/types"
	pb "github.com/openparallax/openparallax/internal/types/pb"
	"github.com/openparallax/openparallax/llm"
)

// LoopConfig configures the reasoning loop.
type LoopConfig struct {
	Provider            llm.Provider
	Agent               *Agent
	MaxRounds           int
	ContextWindow       int
	CompactionThreshold int // percentage (0-100), default 70
	MaxResponseTokens   int // default 4096
}

// ToolProposal is a tool call the LLM wants to make.
type ToolProposal struct {
	CallID        string
	ToolName      string
	ArgumentsJSON string
}

// ToolResult is the engine's response to a tool proposal. When the tool
// being responded to is the load_tools meta-tool, NewTools carries the
// freshly resolved tool definitions so the loop can merge them into the
// active tool slice and the next provider call can actually expose them
// to the LLM. NewTools is nil for ordinary executor tool results.
type ToolResult struct {
	CallID   string
	Content  string
	IsError  bool
	NewTools []llm.ToolDefinition
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
	Usage    *llm.TokenUsage

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
	discoverySummary := ""
	loadedSkills := ""
	if cfg.Agent.Skills != nil {
		discoverySummary = cfg.Agent.Skills.DiscoverySummary()
		loadedSkills = cfg.Agent.Skills.LoadedSkillBodies()
	}
	systemPrompt, err := cfg.Agent.Context.AssembleWithSkills(mode, content, discoverySummary, loadedSkills)
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
	maxResponseTokens := cfg.MaxResponseTokens
	contextBudget := contextWindow - systemTokens - maxResponseTokens
	if contextBudget < maxResponseTokens {
		contextBudget = maxResponseTokens
	}

	historyTokens := 0
	for _, m := range history {
		historyTokens += cfg.Provider.EstimateTokens(m.Content)
	}
	usagePercent := float64(historyTokens) / float64(contextBudget) * 100

	compactionThreshold := cfg.CompactionThreshold
	if usagePercent >= float64(compactionThreshold) {
		// Compact in-memory. The compactor may flush facts — we intercept
		// that by using a memory-flush callback instead of direct writes.
		compacted, compactErr := cfg.Agent.CompactHistory(ctx, history, contextBudget, cfg.CompactionThreshold)
		if compactErr != nil {
			emit(LoopEvent{Type: EventLoopError, ErrorCode: "COMPACTION_FAILED", ErrorMessage: compactErr.Error()})
		}
		history = compacted
	}

	// Build messages.
	messages := make([]llm.ChatMessage, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, llm.ChatMessage{Role: "user", Content: content})

	maxRounds := cfg.MaxRounds

	// Start LLM stream.
	toolStream, err := cfg.Provider.StreamWithTools(ctx, messages, tools,
		llm.WithSystem(systemPrompt), llm.WithMaxTokens(maxResponseTokens))
	if err != nil {
		emit(LoopEvent{Type: EventLoopError, ErrorCode: "LLM_CALL_FAILED", ErrorMessage: err.Error()})
		return
	}
	defer func() { _ = toolStream.Close() }()

	var toolResults []llm.ToolResult
	var thoughts []types.Thought
	var reasoningBuf strings.Builder
	// presentation accumulates the user-facing assistant content: every
	// reasoning fragment in order, separated by blank lines, with the
	// final answer appended last. Persisted as the assistant message
	// content so reload renders the same thing the live stream did.
	var presentation strings.Builder
	rounds := 0

	for rounds < maxRounds {
		event, eventErr := toolStream.Next()
		if eventErr == io.EOF || event.Type == llm.EventDone {
			if len(toolResults) > 0 {
				if reasoningBuf.Len() > 0 {
					fragment := strings.TrimSpace(reasoningBuf.String())
					thoughts = append(thoughts, types.Thought{
						Stage:   "reasoning",
						Summary: fragment,
					})
					appendPresentation(&presentation, fragment)
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
				fragment := strings.TrimSpace(reasoningBuf.String())
				thoughts = append(thoughts, types.Thought{
					Stage:   "reasoning",
					Summary: fragment,
				})
				appendPresentation(&presentation, fragment)
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

				// Wait for tool definitions from Engine.
				select {
				case result, ok := <-resultCh:
					if !ok {
						return
					}
					toolResults = append(toolResults, llm.ToolResult{CallID: tc.ID, Content: result.Content})
					// Merge the freshly resolved tools into the active
					// tool slice and tell the provider stream to use
					// them on the next continuation. Without this, the
					// LLM is told "loaded" but the next request still
					// only ships load_tools, the freshly loaded
					// functions are not actually callable, and the
					// model loops on load_tools forever waiting for
					// something it can call.
					if len(result.NewTools) > 0 {
						tools = mergeToolDefinitions(tools, result.NewTools)
						toolStream.SetTools(tools)
					}
					thoughts = append(thoughts, types.Thought{
						Stage:   "tool_call",
						Summary: fmt.Sprintf("load_tools(%s)", strings.Join(names, ", ")),
						Detail:  map[string]any{"tool_name": "load_tools", "success": true},
					})
				case <-ctx.Done():
					return
				}
				rounds++
				continue
			}

			// Handle load_skills meta-tool locally.
			if tc.Name == "load_skills" && cfg.Agent.Skills != nil {
				skillNames, _ := tc.Arguments["skills"].([]any)
				var loaded []string
				var bodies []string
				for _, s := range skillNames {
					name, ok := s.(string)
					if !ok {
						continue
					}
					body, found := cfg.Agent.Skills.LoadSkill(name)
					if found {
						loaded = append(loaded, name)
						bodies = append(bodies, fmt.Sprintf("# Skill: %s\n\n%s", name, body))
					}
				}
				response := "No skills found."
				if len(bodies) > 0 {
					response = fmt.Sprintf("Loaded %d skill(s):\n\n%s", len(bodies), strings.Join(bodies, "\n\n---\n\n"))
				}
				toolResults = append(toolResults, llm.ToolResult{CallID: tc.ID, Content: response})
				thoughts = append(thoughts, types.Thought{
					Stage:   "tool_call",
					Summary: fmt.Sprintf("load_skills(%s)", strings.Join(loaded, ", ")),
					Detail:  map[string]any{"tool_name": "load_skills", "success": len(loaded) > 0},
				})
				rounds++
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

			// Wait for Engine to evaluate + execute + return result. The
			// comma-ok form on resultCh guards against the upstream pump
			// goroutine closing the channel before delivering a result —
			// without it the loop would receive a zero-value ToolResult
			// and treat a torn-down stream as a phantom successful call.
			select {
			case result, ok := <-resultCh:
				if !ok {
					return
				}
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

			case <-ctx.Done():
				return
			}
		}
	}

	// Flush any trailing text (the "final answer" — text emitted after the
	// last tool call, or the entire response if there were no tool calls).
	// It is appended to presentation but NOT to thoughts: the dropdown
	// holds tool calls only, the bubble holds reasoning + final answer.
	if reasoningBuf.Len() > 0 {
		appendPresentation(&presentation, strings.TrimSpace(reasoningBuf.String()))
		reasoningBuf.Reset()
	}

	fullResponse := strings.TrimSpace(presentation.String())
	if fullResponse == "" {
		// Fallback for edge cases where presentation never received any
		// text (e.g. tool-only turns with no LLM narration). Use the raw
		// stream text so the message body is not empty.
		fullResponse = toolStream.FullText()
	}
	usage := toolStream.Usage()

	emit(LoopEvent{
		Type:     EventComplete,
		Content:  fullResponse,
		Thoughts: thoughts,
		Usage:    &usage,
	})
}

// appendPresentation writes a reasoning fragment (or the final answer) into
// the presentation buffer with a blank-line separator between entries.
// Empty fragments are skipped. Whitespace-only fragments don't get an extra
// separator added.
func appendPresentation(b *strings.Builder, fragment string) {
	if fragment == "" {
		return
	}
	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	b.WriteString(fragment)
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

// mergeToolDefinitions appends tools from add into base, deduplicating by
// name. The existing entry wins on conflict so a freshly loaded group
// cannot silently shadow load_tools or another already-active definition.
func mergeToolDefinitions(base, add []llm.ToolDefinition) []llm.ToolDefinition {
	seen := make(map[string]bool, len(base))
	for _, t := range base {
		seen[t.Name] = true
	}
	merged := append([]llm.ToolDefinition(nil), base...)
	for _, t := range add {
		if seen[t.Name] {
			continue
		}
		seen[t.Name] = true
		merged = append(merged, t)
	}
	return merged
}

// ToolDefsToLLM converts proto tool definitions to LLM tool definitions.
// The engine ships each tool's JSON Schema as the parameters_json field on
// the wire (see engine_pipeline.go llmToolsToProto). It must be unmarshaled
// here so the LLM provider sees the schema — without it the OpenAI SDK
// elides parameters as a zero value, the upstream provider receives a
// function definition with no input_schema, and the request is rejected.
func ToolDefsToLLM(defs []*pb.ToolDef) []llm.ToolDefinition {
	result := make([]llm.ToolDefinition, 0, len(defs))
	for _, d := range defs {
		def := llm.ToolDefinition{
			Name:        d.Name,
			Description: d.Description,
		}
		if d.ParametersJson != "" {
			var params map[string]any
			if err := json.Unmarshal([]byte(d.ParametersJson), &params); err == nil {
				def.Parameters = params
			}
		}
		result = append(result, def)
	}
	return result
}
