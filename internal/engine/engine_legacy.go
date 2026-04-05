package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/openparallax/openparallax/audit"
	"github.com/openparallax/openparallax/chronicle"
	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/ifc"
	"github.com/openparallax/openparallax/internal/agent"
	"github.com/openparallax/openparallax/internal/session"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/llm"
)

// ProcessMessageForWeb is the public entry point for the web server.
// It subscribes the WebSocket sender for events and forwards the message
// to the Agent for LLM processing.
func (e *Engine) ProcessMessageForWeb(ctx context.Context, sender EventSender, sid, mid, content string, mode types.SessionMode) error {
	// OTR sessions never persist to the database.
	if mode != types.SessionOTR {
		e.storeMessage(sid, mid, "user", content)
	}

	// Subscribe this WS connection for events on this session.
	// Use a stable client ID derived from the sender pointer so that
	// multiple messages on the same connection reuse (replace) the same
	// subscription instead of stacking duplicates.
	clientID := fmt.Sprintf("ws-%p", sender)
	e.broadcaster.Subscribe(clientID, sid, sender)

	// Try forwarding to Agent (new architecture).
	e.mu.Lock()
	hasAgent := e.agentStream != nil
	e.mu.Unlock()

	e.log.Info("process_web_message", "session", sid, "has_agent", hasAgent, "content_len", len(content))

	if hasAgent {
		if err := e.forwardToAgent(sid, mid, content, mode, "web"); err != nil {
			e.log.Error("forward_to_agent_failed", "session", sid, "error", err)
			return e.sendErrorEvent(sender, sid, mid, "AGENT_UNAVAILABLE", err.Error())
		}
		e.log.Info("forwarded_to_agent", "session", sid)
		// Wait for completion or disconnect.
		<-ctx.Done()
		e.log.Info("web_ctx_done", "session", sid)
		return nil
	}

	e.log.Info("fallback_in_process", "session", sid)
	// Fallback: run the old in-process pipeline during migration.
	return e.processMessageCore(ctx, sender, sid, mid, content, mode)
}

// processMessageCore is the shared pipeline logic for both gRPC and WebSocket.
func (e *Engine) processMessageCore(ctx context.Context, sender EventSender, sid, mid, content string, mode types.SessionMode) error {
	start := time.Now()
	isOTR := mode == types.SessionOTR

	e.storeMessage(sid, mid, "user", content)
	e.log.Info("message_received", "session", sid, "length", len(content))

	// Load history.
	history := e.getHistory(sid)

	// Build system prompt with OTR awareness and skills.
	discoverySummary := ""
	loadedSkills := ""
	if e.agent.Skills != nil {
		discoverySummary = e.agent.Skills.DiscoverySummary()
		loadedSkills = e.agent.Skills.LoadedSkillBodies()
	}
	systemPrompt, err := e.agent.Context.AssembleWithSkills(mode, content, discoverySummary, loadedSkills)
	if err != nil {
		return e.sendErrorEvent(sender, sid, mid, "CONTEXT_FAILED", err.Error())
	}

	// Summarize stale tool results before compaction check.
	turnCount := 0
	for _, m := range history {
		if m.Role == "user" {
			turnCount++
		}
	}
	history = agent.SummarizeStaleToolResults(history, turnCount, 4)

	// Compact history if approaching context limits.
	// Budget = model context window minus system prompt and tool definition overhead.
	systemTokens := e.llm.EstimateTokens(systemPrompt)
	contextWindow := 128000 // conservative default; most models support this
	contextBudget := contextWindow - systemTokens - 4096
	if contextBudget < 4096 {
		contextBudget = 4096
	}

	historyTokens := 0
	for _, m := range history {
		historyTokens += e.llm.EstimateTokens(m.Content)
	}
	usagePercent := float64(historyTokens) / float64(contextBudget) * 100

	e.log.Debug("compaction_check",
		"history_tokens", historyTokens, "context_budget", contextBudget,
		"usage_percent", int(usagePercent), "threshold_percent", 70,
		"triggered", usagePercent >= 70, "history_msgs", len(history))

	if usagePercent >= 70 {
		before := len(history)
		history, _ = e.agent.CompactHistory(ctx, history, contextBudget)
		after := len(history)
		if after < before {
			afterTokens := 0
			for _, m := range history {
				afterTokens += e.llm.EstimateTokens(m.Content)
			}
			e.log.Info("compaction_executed",
				"messages_before", before, "messages_after", after,
				"tokens_before", historyTokens, "tokens_after", afterTokens)
		}
	}

	// Start with only load_tools. The LLM requests groups as needed.
	loadToolsDef := e.executors.Groups.LoadToolsDefinition()
	tools := []llm.ToolDefinition{loadToolsDef}
	var loadedGroupTools []llm.ToolDefinition

	// Build messages: history + current user message.
	messages := make([]llm.ChatMessage, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, llm.ChatMessage{Role: "user", Content: content})

	e.log.Info("llm_call_started", "session", sid, "provider", e.llm.Name(),
		"model", e.llm.Model(), "tools", len(tools), "history", len(messages))

	// Initialize streaming redactor.
	redactor := NewStreamingRedactor(func(text string) {
		_ = sender.SendEvent(&PipelineEvent{
			SessionID: sid, MessageID: mid,
			Type:     EventLLMToken,
			LLMToken: &LLMTokenEvent{Text: text},
		})
	})

	// Start tool-use stream.
	toolStream, err := e.llm.StreamWithTools(ctx, messages, tools,
		llm.WithSystem(systemPrompt), llm.WithMaxTokens(4096))
	if err != nil {
		e.log.Error("llm_call_failed", "session", sid, "error", err)
		return e.sendErrorEvent(sender, sid, mid, "LLM_CALL_FAILED", err.Error())
	}
	defer func() { _ = toolStream.Close() }()

	// Main orchestration loop.
	var toolResults []llm.ToolResult
	var toolsExecuted int
	var thoughts []types.Thought
	var reasoningBuf strings.Builder
	rounds := 0

	for rounds < maxToolRounds {
		event, eventErr := toolStream.Next()
		if eventErr == io.EOF || event.Type == llm.EventDone {
			if len(toolResults) > 0 {
				// Capture any reasoning text that preceded this batch of tool calls.
				if reasoningBuf.Len() > 0 {
					thoughts = append(thoughts, types.Thought{
						Stage:   "reasoning",
						Summary: strings.TrimSpace(reasoningBuf.String()),
					})
					reasoningBuf.Reset()
				}
				redactor.Flush()
				e.log.Info("sending_tool_results", "session", sid, "count", len(toolResults))

				if sendErr := toolStream.SendToolResults(toolResults); sendErr != nil {
					e.log.Error("tool_result_send_failed", "session", sid, "error", sendErr)
					break
				}
				toolResults = nil
				rounds++
				continue
			}
			break
		}
		if eventErr != nil {
			e.log.Error("stream_error", "session", sid, "error", eventErr)
			break
		}

		switch event.Type {
		case llm.EventTextDelta:
			redactor.Write(event.Text)
			reasoningBuf.WriteString(event.Text)

		case llm.EventToolCallComplete:
			// Capture reasoning that preceded this tool call.
			if reasoningBuf.Len() > 0 {
				thoughts = append(thoughts, types.Thought{
					Stage:   "reasoning",
					Summary: strings.TrimSpace(reasoningBuf.String()),
				})
				reasoningBuf.Reset()
			}
			redactor.Flush()
			tc := event.ToolCall

			e.log.Info("tool_call_received", "session", sid, "tool", tc.Name, "call_id", tc.ID)

			// Handle load_tools meta-tool: expand tool set for the current turn.
			if tc.Name == "load_tools" {
				groupNames, _ := tc.Arguments["groups"].([]any)
				names := make([]string, 0, len(groupNames))
				for _, g := range groupNames {
					if s, ok := g.(string); ok {
						names = append(names, s)
					}
				}
				newTools, summary := e.executors.Groups.ResolveGroups(names, isOTR)
				loadedGroupTools = append(loadedGroupTools, newTools...)
				tools = append([]llm.ToolDefinition{loadToolsDef}, loadedGroupTools...)
				e.log.Info("tools_loaded", "groups", strings.Join(names, ","),
					"tools_count", len(newTools), "tool_def_tokens", len(summary)/4)
				toolResults = append(toolResults, llm.ToolResult{CallID: tc.ID, Content: summary})
				thoughts = append(thoughts, types.Thought{
					Stage:   "tool_call",
					Summary: fmt.Sprintf("load_tools(%s)", strings.Join(names, ", ")),
					Detail:  map[string]any{"tool_name": "load_tools", "success": true},
				})
				continue
			}

			result := e.processToolCall(ctx, tc, mode, sid, mid, sender)
			e.log.Debug("tool_result", "call_id", result.CallID, "is_error", result.IsError, "content_len", len(result.Content))
			toolResults = append(toolResults, result)

			// Record tool call as a thought for persistence.
			tcDetail := map[string]any{
				"tool_name": tc.Name,
				"success":   !result.IsError,
			}
			thoughts = append(thoughts, types.Thought{
				Stage:   "tool_call",
				Summary: formatToolCallSummary(tc),
				Detail:  tcDetail,
			})

			toolsExecuted++
		}
	}

	redactor.Flush()
	fullResponse := toolStream.FullText()

	e.log.Info("response_complete", "session", sid, "rounds", rounds,
		"response_len", len(fullResponse), "thoughts", len(thoughts),
		"tools_executed", toolsExecuted)

	// Store assistant message with thoughts (reasoning + tool calls).
	assistantMsg := &types.Message{
		SessionID: sid,
		Role:      "assistant",
		Content:   fullResponse,
		Timestamp: time.Now(),
		Thoughts:  thoughts,
	}
	e.storeAssistantMessage(sid, assistantMsg)

	_ = sender.SendEvent(&PipelineEvent{
		SessionID: sid, MessageID: mid,
		Type:             EventResponseComplete,
		ResponseComplete: &ResponseCompleteEvent{Content: fullResponse, Thoughts: thoughts},
	})

	// Generate a session title once there's enough context (3+ exchanges).
	// Runs once — after that the title sticks.
	if !isOTR {
		if sess, err := e.db.GetSession(sid); err == nil && sess.Title == "" {
			history := e.getHistory(sid)
			if len(history) >= 6 {
				go e.generateSessionTitle(sid, history)
			}
		}
	}

	// Capture token usage from the provider (real metrics when available).
	usage := toolStream.Usage()
	if usage.InputTokens == 0 {
		// Fallback to estimation if provider didn't report.
		for _, m := range messages {
			usage.InputTokens += e.llm.EstimateTokens(m.Content)
		}
	}
	if usage.OutputTokens == 0 {
		usage.OutputTokens = e.llm.EstimateTokens(fullResponse)
	}
	// Estimate tool definition tokens from serialized size.
	toolJSON, _ := json.Marshal(tools)
	usage.ToolDefinitionTokens = len(toolJSON) / 4

	e.log.Info("message_complete", "session", sid,
		"response_length", len(fullResponse), "rounds", rounds,
		"input_tokens", usage.InputTokens, "output_tokens", usage.OutputTokens,
		"cache_creation_tokens", usage.CacheCreationTokens,
		"cache_read_tokens", usage.CacheReadTokens,
		"tool_def_tokens", usage.ToolDefinitionTokens,
		"history_msgs", len(messages), "tools_sent", len(tools))

	// Persist token usage metrics.
	_ = e.db.InsertLLMUsage(storage.LLMUsageEntry{
		SessionID:           sid,
		MessageID:           mid,
		Provider:            e.llm.Name(),
		Model:               e.llm.Model(),
		InputTokens:         usage.InputTokens,
		OutputTokens:        usage.OutputTokens,
		CacheReadTokens:     usage.CacheReadTokens,
		CacheCreationTokens: usage.CacheCreationTokens,
		ToolDefTokens:       usage.ToolDefinitionTokens,
		Rounds:              rounds,
		DurationMs:          time.Since(start).Milliseconds(),
	})
	e.db.IncrementDailyMetric("llm_calls", 1)
	e.db.IncrementDailyMetric("tokens_input", usage.InputTokens)
	e.db.IncrementDailyMetric("tokens_output", usage.OutputTokens)
	e.db.IncrementDailyMetric("messages_processed", 1)

	return nil
}

// processToolCall handles a single tool call through the full security pipeline.
func (e *Engine) processToolCall(ctx context.Context, tc *llm.ToolCall, mode types.SessionMode, sid, mid string, sender EventSender) llm.ToolResult {
	// Convert tool call to ActionRequest.
	action := &types.ActionRequest{
		RequestID: crypto.NewID(),
		Type:      types.ActionType(tc.Name),
		Payload:   tc.Arguments,
		Timestamp: time.Now(),
	}
	hash, _ := crypto.HashAction(tc.Name, tc.Arguments)
	action.Hash = hash

	// Metadata enrichment.
	e.enricher.Enrich(action)

	// Hardcoded protection check — before OTR, before Shield, before audit.
	allowed, protection, protReason := CheckProtection(action, e.cfg.Workspace)
	if !allowed {
		e.log.Warn("protection_blocked", "session", sid, "tool", tc.Name, "reason", protReason)
		e.auditLog(audit.Entry{
			EventType: types.AuditSelfProtection, SessionID: sid,
			ActionType: string(action.Type), Details: protReason,
		})
		_ = sender.SendEvent(&PipelineEvent{
			SessionID: sid, MessageID: mid, Type: EventActionCompleted,
			ActionCompleted: &ActionCompletedEvent{ToolName: tc.Name, Success: false, Summary: "Blocked: " + protReason},
		})
		return llm.ToolResult{CallID: tc.ID, Content: "Blocked: " + protReason, IsError: true}
	}
	switch protection {
	case EscalateTier2:
		action.MinTier = 2
	case WriteTier1Min:
		action.MinTier = 1
	}

	isOTR := mode == types.SessionOTR

	// Emit tool call started.
	_ = sender.SendEvent(&PipelineEvent{
		SessionID: sid, MessageID: mid, Type: EventActionStarted,
		ActionStarted: &ActionStartedEvent{ToolName: tc.Name, Summary: formatToolCallSummary(tc)},
	})

	// Audit: proposed.
	e.auditLog(audit.Entry{
		EventType: types.AuditActionProposed, SessionID: sid,
		ActionType: string(action.Type), Details: "hash: " + action.Hash, OTR: isOTR,
	})

	// OTR check (defense in depth — primary enforcement is tool filtering).
	if isOTR && !session.IsOTRAllowed(action.Type) {
		reason := session.OTRBlockReason(action.Type)
		e.log.Info("otr_blocked", "session", sid, "tool", tc.Name)
		e.auditLog(audit.Entry{
			EventType: types.AuditActionBlocked, SessionID: sid,
			ActionType: string(action.Type), Details: "OTR: " + reason, OTR: true,
		})
		_ = sender.SendEvent(&PipelineEvent{
			SessionID: sid, MessageID: mid, Type: EventOTRBlocked,
			OTRBlocked: &OTRBlockedEvent{Reason: reason},
		})
		return llm.ToolResult{CallID: tc.ID, Content: "Blocked: " + reason, IsError: true}
	}

	// Shield evaluation.
	shieldStart := time.Now()
	verdict := e.shield.Evaluate(ctx, action)
	shieldMs := time.Since(shieldStart).Milliseconds()

	e.log.Info("shield_verdict", "session", sid, "tool", tc.Name,
		"decision", verdict.Decision, "tier", verdict.Tier,
		"confidence", verdict.Confidence, "ms", shieldMs,
		"reasoning", truncateForLog(verdict.Reasoning))

	_ = sender.SendEvent(&PipelineEvent{
		SessionID: sid, MessageID: mid, Type: EventShieldVerdict,
		ShieldVerdict: &ShieldVerdictEvent{
			ToolName: tc.Name, Decision: string(verdict.Decision), Tier: verdict.Tier,
			Confidence: verdict.Confidence, Reasoning: verdict.Reasoning,
		},
	})
	e.auditLog(audit.Entry{
		EventType: types.AuditActionEvaluated, SessionID: sid,
		ActionType: string(action.Type),
		Details:    fmt.Sprintf("%s (tier %d, %.0f%%): %s", verdict.Decision, verdict.Tier, verdict.Confidence*100, verdict.Reasoning),
	})

	// Track shield metrics.
	e.db.IncrementDailyMetric("shield_"+strings.ToLower(string(verdict.Decision)), 1)
	e.db.IncrementDailyMetric(fmt.Sprintf("shield_t%d", verdict.Tier), 1)

	// Audit rate limit and budget exhaustion specifically.
	if strings.Contains(verdict.Reasoning, "rate limit") {
		e.auditLog(audit.Entry{
			EventType: types.AuditRateLimitHit, SessionID: sid,
			ActionType: string(action.Type), Details: verdict.Reasoning,
		})
	}
	if strings.Contains(verdict.Reasoning, "budget exhausted") {
		e.auditLog(audit.Entry{
			EventType: types.AuditBudgetExhausted, SessionID: sid,
			ActionType: string(action.Type), Details: verdict.Reasoning,
		})
	}

	if verdict.Decision == types.VerdictBlock {
		e.auditLog(audit.Entry{
			EventType: types.AuditActionBlocked, SessionID: sid,
			ActionType: string(action.Type), Details: verdict.Reasoning,
		})
		_ = sender.SendEvent(&PipelineEvent{
			SessionID: sid, MessageID: mid, Type: EventActionCompleted,
			ActionCompleted: &ActionCompletedEvent{ToolName: tc.Name, Success: false, Summary: "Blocked: " + verdict.Reasoning},
		})
		return llm.ToolResult{CallID: tc.ID, Content: "Blocked by security: " + verdict.Reasoning, IsError: true}
	}

	if verdict.Decision == types.VerdictEscalate {
		approved, approvalErr := e.requestTier3Approval(ctx, sid, mid, tc.Name, action, verdict.Reasoning)
		if approvalErr != nil || !approved {
			reason := "denied by human review"
			if approvalErr != nil {
				reason = "approval timeout or error"
			}
			_ = sender.SendEvent(&PipelineEvent{
				SessionID: sid, MessageID: mid, Type: EventActionCompleted,
				ActionCompleted: &ActionCompletedEvent{ToolName: tc.Name, Success: false, Summary: "Escalated: " + reason},
			})
			return llm.ToolResult{CallID: tc.ID, Content: reason, IsError: true}
		}
	}

	// Hash verification.
	if verifyErr := e.verifier.Verify(action); verifyErr != nil {
		e.log.Error("hash_verify_failed", "session", sid, "tool", tc.Name, "error", verifyErr)
		return llm.ToolResult{CallID: tc.ID, Content: "Integrity check failed", IsError: true}
	}
	e.log.Debug("hash_verified", "session", sid, "tool", tc.Name, "hash", action.Hash[:16])

	// Chronicle snapshot (Normal mode only).
	if !isOTR {
		if snapMeta, snapErr := e.chronicle.Snapshot(&chronicle.ActionRequest{Type: string(action.Type), Payload: action.Payload}); snapErr != nil {
			e.log.Warn("chronicle_snapshot_failed", "session", sid, "error", snapErr)
		} else if snapMeta != nil {
			e.log.Debug("chronicle_snapshot", "session", sid, "tool", tc.Name, "snapshot_id", snapMeta.ID)
		}
	}

	// IFC check: if the action sends data externally and we've seen sensitive
	// data in this session, block the flow.
	if action.DataClassification != nil && !ifc.IsFlowAllowed(action.DataClassification, action.Type) {
		reason := "IFC violation: sensitive data cannot flow to this destination"
		e.log.Warn("ifc_blocked", "session", sid, "tool", tc.Name,
			"sensitivity", action.DataClassification.Sensitivity, "source", action.DataClassification.SourcePath)
		e.auditLog(audit.Entry{
			EventType: types.AuditActionBlocked, SessionID: sid,
			ActionType: string(action.Type), Details: reason,
		})
		return llm.ToolResult{CallID: tc.ID, Content: "Blocked: " + reason, IsError: true}
	}

	// Route: MCP tools or built-in executors.
	e.log.Info("executor_start", "session", sid, "tool", tc.Name)
	start := time.Now()
	var result *types.ActionResult

	if e.mcpManager != nil {
		if client, toolName, isMCP := e.mcpManager.Route(tc.Name); isMCP {
			mcpResult, mcpErr := client.CallTool(ctx, toolName, tc.Arguments)
			if mcpErr != nil {
				result = &types.ActionResult{
					RequestID: action.RequestID, Success: false,
					Error: mcpErr.Error(), Summary: "MCP call failed: " + mcpErr.Error(),
				}
			} else {
				result = &types.ActionResult{
					RequestID: action.RequestID, Success: true,
					Output: mcpResult, Summary: "MCP call completed",
				}
			}
			result.DurationMs = time.Since(start).Milliseconds()
		}
	}

	// Built-in executor — only if MCP didn't handle it.
	if result == nil {
		result = e.executors.Execute(ctx, action)
		result.DurationMs = time.Since(start).Milliseconds()
	}

	e.db.IncrementDailyMetric("tool_calls", 1)
	e.db.IncrementDailyMetric("tool:"+tc.Name, 1)
	if result.Success {
		e.db.IncrementDailyMetric("tool_success", 1)
		e.log.Info("executor_complete", "session", sid, "tool", tc.Name, "success", true, "ms", result.DurationMs)
		e.auditLog(audit.Entry{
			EventType: types.AuditActionExecuted, SessionID: sid,
			ActionType: string(action.Type), Details: result.Summary,
		})
	} else {
		e.db.IncrementDailyMetric("tool_failed", 1)
		e.log.Info("executor_complete", "session", sid, "tool", tc.Name, "success", false, "error", result.Error)
		e.auditLog(audit.Entry{
			EventType: types.AuditActionFailed, SessionID: sid,
			ActionType: string(action.Type), Details: result.Error,
		})
	}

	_ = sender.SendEvent(&PipelineEvent{
		SessionID: sid, MessageID: mid, Type: EventActionCompleted,
		ActionCompleted: &ActionCompletedEvent{ToolName: tc.Name, Success: result.Success, Summary: result.Summary},
	})

	// Format result for the LLM. On failure, prefer the full output (which
	// includes stderr) so the LLM knows *why* the command failed, not just
	// "exit status 1".
	content := result.Summary
	if result.Output != "" {
		content = result.Output
	}
	if !result.Success && result.Output == "" {
		content = "Error: " + result.Error
	}

	return llm.ToolResult{CallID: tc.ID, Content: content, IsError: !result.Success}
}

func formatToolCallSummary(tc *llm.ToolCall) string {
	switch tc.Name {
	case "read_file":
		return fmt.Sprintf("Reading %s", tc.Arguments["path"])
	case "write_file":
		return fmt.Sprintf("Writing %s", tc.Arguments["path"])
	case "delete_file":
		return fmt.Sprintf("Deleting %s", tc.Arguments["path"])
	case "list_directory":
		return fmt.Sprintf("Listing %s", tc.Arguments["path"])
	case "execute_command":
		cmd, _ := tc.Arguments["command"].(string)
		if len(cmd) > 60 {
			cmd = cmd[:60] + "..."
		}
		return fmt.Sprintf("Running: %s", cmd)
	default:
		return tc.Name
	}
}
