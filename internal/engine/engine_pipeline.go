package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openparallax/openparallax/audit"
	"github.com/openparallax/openparallax/chronicle"
	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/ifc"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	pb "github.com/openparallax/openparallax/internal/types/pb"
	"github.com/openparallax/openparallax/memory"
)

// SendMessage implements ClientService.SendMessage — the entry point for TUI
// and other gRPC clients. Stores the user message, subscribes the client for
// events, and forwards the request to the Agent for LLM processing.
func (e *Engine) SendMessage(req *pb.ClientMessageRequest, stream pb.ClientService_SendMessageServer) error {
	sid := req.SessionId
	mid := crypto.NewID()
	mode := types.SessionNormal
	if req.Mode == pb.SessionMode_OTR {
		mode = types.SessionOTR
	}

	// Store user message.
	e.storeMessage(sid, mid, "user", req.Content)

	// Subscribe this client stream for events on this session.
	clientID := "grpc-" + mid
	sender := newGRPCEventSender(stream)
	e.broadcaster.Subscribe(clientID, sid, sender)
	defer e.broadcaster.Unsubscribe(clientID)

	// Forward to Agent.
	if err := e.forwardToAgent(sid, mid, req.Content, mode, req.Source); err != nil {
		return e.sendErrorEvent(sender, sid, mid, "AGENT_UNAVAILABLE", err.Error())
	}

	// Wait for ResponseComplete or context cancellation. The broadcaster
	// delivers events to this client as they arrive from the Agent.
	<-stream.Context().Done()
	return nil
}

// forwardToAgent sends a ProcessRequest to the connected Agent.
func (e *Engine) forwardToAgent(sid, mid, content string, mode types.SessionMode, source string) error {
	e.mu.Lock()
	agentStream := e.agentStream
	e.currentMsgOTR = mode == types.SessionOTR
	e.mu.Unlock()

	if agentStream == nil {
		// Fall back to old processMessageCore for backward compatibility
		// during the migration period.
		return nil
	}

	pbMode := pb.SessionMode_NORMAL
	if mode == types.SessionOTR {
		pbMode = pb.SessionMode_OTR
	}

	return agentStream.Send(&pb.EngineDirective{
		Directive: &pb.EngineDirective_Process{
			Process: &pb.ProcessRequest{
				SessionId: sid,
				MessageId: mid,
				Content:   content,
				Mode:      pbMode,
				Source:    source,
			},
		},
	})
}

// RunSession implements AgentService.RunSession — the bidirectional stream
// between the Engine and the sandboxed Agent process. The Agent sends events
// (tokens, tool proposals, completions); the Engine evaluates tool calls
// through Shield, executes them, and broadcasts events to all clients.
func (e *Engine) RunSession(stream pb.AgentService_RunSessionServer) error {
	ctx := stream.Context()

	// Wait for AgentReady.
	firstEvent, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("agent stream: %w", err)
	}
	ready := firstEvent.GetReady()
	if ready == nil {
		return fmt.Errorf("expected AgentReady, got %T", firstEvent.Event)
	}

	// Validate agent auth token.
	agentID := ready.AgentId
	e.mu.Lock()
	expectedToken := e.agentAuthToken
	e.mu.Unlock()
	if expectedToken != "" {
		parts := strings.SplitN(agentID, ":", 2)
		if len(parts) != 2 || parts[1] != expectedToken {
			e.log.Error("agent_auth_failed", "id", agentID)
			return fmt.Errorf("agent authentication failed: invalid token")
		}
		agentID = parts[0]
	}
	e.log.Info("agent_connected", "id", agentID)

	// Store the agent stream for forwarding messages.
	e.mu.Lock()
	e.agentStream = stream
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.agentStream = nil
		e.mu.Unlock()
		e.log.Info("agent_disconnected", "id", agentID)
	}()

	// Track tool calls and timing for the current message.
	var pendingThoughts []types.Thought
	var msgStartTime time.Time
	msgRounds := 0

	// Read agent events in a loop.
	for {
		event, recvErr := stream.Recv()
		if recvErr != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("agent stream recv: %w", recvErr)
		}

		switch ev := event.Event.(type) {
		case *pb.AgentEvent_LlmTokenEmitted:
			if msgStartTime.IsZero() {
				msgStartTime = time.Now()
			}
			e.broadcaster.Broadcast(&PipelineEvent{
				SessionID: ev.LlmTokenEmitted.SessionId,
				MessageID: ev.LlmTokenEmitted.MessageId,
				Type:      EventLLMToken,
				LLMToken:  &LLMTokenEvent{Text: ev.LlmTokenEmitted.Text},
			})

		case *pb.AgentEvent_ToolProposal:
			msgRounds++
			tp := ev.ToolProposal
			result := e.handleToolProposal(ctx, tp)

			// Track tool call as a thought for persistence.
			summary := tp.ToolName
			if result.Content != "" && len(result.Content) < 80 {
				summary = tp.ToolName + " " + result.Content
			}
			detail := map[string]any{
				"tool_name": tp.ToolName,
				"success":   !result.IsError,
			}
			if strings.HasPrefix(result.Content, "Blocked") {
				detail["shield"] = "BLOCK"
			}
			pendingThoughts = append(pendingThoughts, types.Thought{
				Stage:   "tool_call",
				Summary: summary,
				Detail:  detail,
			})

			// Send result back to Agent.
			if sendErr := stream.Send(&pb.EngineDirective{
				Directive: &pb.EngineDirective_ToolResult{
					ToolResult: &pb.ToolResultDelivery{
						CallId:  tp.CallId,
						Content: result.Content,
						IsError: result.IsError,
					},
				},
			}); sendErr != nil {
				e.log.Error("tool_result_send_failed", "error", sendErr)
			}

		case *pb.AgentEvent_ToolDefsRequest:
			groups := ev.ToolDefsRequest.Groups
			e.mu.Lock()
			isOTR := e.currentMsgOTR
			e.mu.Unlock()
			newTools, summary := e.executors.Groups.ResolveGroups(groups, isOTR)
			_ = newTools

			// Send tool definitions back as a ToolResult (the Agent
			// is waiting on resultCh for load_tools response).
			var defs []*pb.ToolDef
			for _, t := range newTools {
				paramJSON, _ := json.Marshal(t.Parameters)
				defs = append(defs, &pb.ToolDef{
					Name:           t.Name,
					Description:    t.Description,
					ParametersJson: string(paramJSON),
				})
			}
			if sendErr := stream.Send(&pb.EngineDirective{
				Directive: &pb.EngineDirective_ToolDefs{
					ToolDefs: &pb.ToolDefsDelivery{Tools: defs},
				},
			}); sendErr != nil {
				e.log.Error("tool_defs_send_failed", "error", sendErr)
			}

			e.log.Info("tools_loaded", "groups", strings.Join(groups, ","),
				"tools_count", len(defs), "summary_len", len(summary))

			pendingThoughts = append(pendingThoughts, types.Thought{
				Stage:   "tool_call",
				Summary: fmt.Sprintf("load_tools(%s)", strings.Join(groups, ", ")),
				Detail:  map[string]any{"tool_name": "load_tools", "success": true},
			})
			e.db.IncrementDailyMetric("tool_calls", 1)
			e.db.IncrementDailyMetric("tool_success", 1)
			e.db.IncrementDailyMetric("tool:load_tools", 1)

		case *pb.AgentEvent_MemoryFlush:
			if ev.MemoryFlush.Content != "" {
				e.log.Debug("memory_flush", "content_len", len(ev.MemoryFlush.Content))
				if memErr := e.memory.Append(memory.MemoryMain, ev.MemoryFlush.Content); memErr != nil {
					e.log.Warn("memory_append_failed", "error", memErr, "content_len", len(ev.MemoryFlush.Content))
				}
			}

		case *pb.AgentEvent_ResponseComplete:
			rc := ev.ResponseComplete
			sid := rc.SessionId
			mid := rc.MessageId
			e.log.Info("agent_response_complete", "session", sid,
				"content_len", len(rc.Content), "thoughts", len(rc.Thoughts))

			// Convert thoughts from agent. If agent sent none but the
			// engine tracked tool calls, use engine-side thoughts.
			var thoughts []types.Thought
			if len(rc.Thoughts) > 0 {
				for _, t := range rc.Thoughts {
					thoughts = append(thoughts, types.Thought{
						Stage:   t.Stage,
						Summary: t.Summary,
					})
				}
			} else if len(pendingThoughts) > 0 {
				thoughts = pendingThoughts
			}
			pendingThoughts = nil

			e.mu.Lock()
			isOTR := e.currentMsgOTR
			e.mu.Unlock()

			// Token usage always persists (cost tracking).
			var durationMs int64
			if !msgStartTime.IsZero() {
				durationMs = time.Since(msgStartTime).Milliseconds()
			}
			if rc.Usage != nil {
				_ = e.db.InsertLLMUsage(storage.LLMUsageEntry{
					SessionID:           sid,
					MessageID:           mid,
					Provider:            e.llm.Name(),
					Model:               e.llm.Model(),
					InputTokens:         int(rc.Usage.InputTokens),
					OutputTokens:        int(rc.Usage.OutputTokens),
					CacheReadTokens:     int(rc.Usage.CacheReadTokens),
					CacheCreationTokens: int(rc.Usage.CacheWriteTokens),
					Rounds:              msgRounds,
					DurationMs:          durationMs,
				})
				e.db.IncrementDailyMetric("llm_calls", 1)
				e.db.IncrementDailyMetric("tokens_input", int(rc.Usage.InputTokens))
				e.db.IncrementDailyMetric("tokens_output", int(rc.Usage.OutputTokens))
			}

			// Reset per-message state for the next message.
			msgStartTime = time.Time{}
			msgRounds = 0

			// OTR: broadcast only, no DB writes.
			if !isOTR {
				msg := &types.Message{
					SessionID: sid,
					Role:      "assistant",
					Content:   rc.Content,
					Timestamp: time.Now(),
					Thoughts:  thoughts,
				}
				e.storeAssistantMessage(sid, msg)
			}

			// Broadcast completion.
			e.broadcaster.Broadcast(&PipelineEvent{
				SessionID: sid, MessageID: mid,
				Type:             EventResponseComplete,
				ResponseComplete: &ResponseCompleteEvent{Content: rc.Content, Thoughts: thoughts},
			})

			// Generate session title (not for OTR).
			if !isOTR {
				if sess, titleErr := e.db.GetSession(sid); titleErr == nil && sess.Title == "" {
					history := e.getHistory(sid)
					if len(history) >= 6 {
						go e.generateSessionTitle(sid, history)
					}
				}
			}

		case *pb.AgentEvent_AgentError:
			ae := ev.AgentError
			e.log.Error("agent_error", "session", ae.SessionId, "code", ae.Code, "message", ae.Message)
			e.auditLog(audit.Entry{
				EventType:  types.AuditShieldError,
				SessionID:  ae.SessionId,
				ActionType: ae.Code,
				Details:    ae.Message,
			})
			e.broadcaster.Broadcast(&PipelineEvent{
				SessionID: ae.SessionId, MessageID: ae.MessageId,
				Type:  EventError,
				Error: &PipelineErrorEvent{Code: ae.Code, Message: ae.Message, Recoverable: ae.Recoverable},
			})
		}
	}
}

// handleToolProposal processes a tool call proposed by the Agent through the
// full security pipeline: protection → Shield → execution.
func (e *Engine) handleToolProposal(ctx context.Context, tp *pb.ToolCallProposed) *pb.ToolResultDelivery {
	sid := tp.SessionId
	mid := tp.MessageId

	e.mu.Lock()
	isOTRAction := e.currentMsgOTR
	e.mu.Unlock()

	// Parse arguments.
	var args map[string]any
	if err := json.Unmarshal([]byte(tp.ArgumentsJson), &args); err != nil {
		args = map[string]any{"raw": tp.ArgumentsJson}
	}
	// Build ActionRequest.
	action := &types.ActionRequest{
		RequestID: crypto.NewID(),
		Type:      types.ActionType(tp.ToolName),
		Payload:   args,
		Timestamp: time.Now(),
	}
	hash, _ := crypto.HashAction(tp.ToolName, args)
	action.Hash = hash

	// Metadata enrichment.
	e.enricher.Enrich(action)

	// Hardcoded protection check.
	allowed, protection, protReason := CheckProtection(action, e.cfg.Workspace)
	if !allowed {
		e.log.Warn("protection_blocked", "session", sid, "tool", tp.ToolName, "reason", protReason)
		e.broadcaster.Broadcast(&PipelineEvent{
			SessionID: sid, MessageID: mid, Type: EventActionCompleted,
			ActionCompleted: &ActionCompletedEvent{ToolName: tp.ToolName, Success: false, Summary: "Blocked: " + protReason},
		})
		return &pb.ToolResultDelivery{CallId: tp.CallId, Content: "Blocked: " + protReason, IsError: true}
	}
	switch protection {
	case EscalateTier2:
		action.MinTier = 2
	case WriteTier1Min:
		action.MinTier = 1
	}

	// Emit ActionStarted.
	e.broadcaster.Broadcast(&PipelineEvent{
		SessionID: sid, MessageID: mid, Type: EventActionStarted,
		ActionStarted: &ActionStartedEvent{ToolName: tp.ToolName, Summary: tp.ToolName + ": " + truncateForLog(tp.ArgumentsJson)},
	})

	// Audit: proposed (skip for OTR).
	if !isOTRAction {
		e.auditLog(audit.Entry{
			EventType: types.AuditActionProposed, SessionID: sid,
			ActionType: string(action.Type), Details: "hash: " + action.Hash,
		})
	}

	// Shield evaluation.
	verdict := e.shield.Evaluate(ctx, action)
	e.broadcaster.Broadcast(&PipelineEvent{
		SessionID: sid, MessageID: mid, Type: EventShieldVerdict,
		ShieldVerdict: &ShieldVerdictEvent{
			ToolName: tp.ToolName, Decision: string(verdict.Decision), Tier: verdict.Tier,
			Confidence: verdict.Confidence, Reasoning: verdict.Reasoning,
		},
	})
	if !isOTRAction {
		e.auditLog(audit.Entry{
			EventType: types.AuditActionEvaluated, SessionID: sid,
			ActionType: string(action.Type),
			Details:    fmt.Sprintf("%s (tier %d): %s", verdict.Decision, verdict.Tier, verdict.Reasoning),
		})
	}

	// Track shield decisions in daily metrics.
	switch verdict.Decision {
	case types.VerdictAllow:
		e.db.IncrementDailyMetric("shield_allow", 1)
	case types.VerdictBlock:
		e.db.IncrementDailyMetric("shield_block", 1)
	case types.VerdictEscalate:
		e.db.IncrementDailyMetric("shield_escalate", 1)
	}
	e.db.IncrementDailyMetric(fmt.Sprintf("shield_t%d", verdict.Tier), 1)

	if verdict.Decision == types.VerdictBlock {
		e.db.IncrementDailyMetric("tool_calls", 1)
		e.db.IncrementDailyMetric("tool_failed", 1)
		e.db.IncrementDailyMetric("tool:"+tp.ToolName, 1)
		if !isOTRAction {
			e.auditLog(audit.Entry{
				EventType:  types.AuditActionBlocked,
				SessionID:  sid,
				ActionType: string(action.Type),
				Details:    verdict.Reasoning,
			})
		}
		e.broadcaster.Broadcast(&PipelineEvent{
			SessionID: sid, MessageID: mid, Type: EventActionCompleted,
			ActionCompleted: &ActionCompletedEvent{ToolName: tp.ToolName, Success: false, Summary: "Blocked: " + verdict.Reasoning},
		})
		return &pb.ToolResultDelivery{CallId: tp.CallId, Content: "Blocked by security: " + verdict.Reasoning, IsError: true}
	}

	if verdict.Decision == types.VerdictEscalate {
		approved, approvalErr := e.requestTier3Approval(ctx, sid, mid, tp.ToolName, action, verdict.Reasoning)
		if approvalErr != nil || !approved {
			reason := "denied by human review"
			if approvalErr != nil {
				reason = "approval timeout or error"
			}
			e.broadcaster.Broadcast(&PipelineEvent{
				SessionID: sid, MessageID: mid, Type: EventActionCompleted,
				ActionCompleted: &ActionCompletedEvent{ToolName: tp.ToolName, Success: false, Summary: "Escalated: " + reason},
			})
			return &pb.ToolResultDelivery{CallId: tp.CallId, Content: reason, IsError: true}
		}
	}

	// Hash verification.
	if verifyErr := e.verifier.Verify(action); verifyErr != nil {
		return &pb.ToolResultDelivery{CallId: tp.CallId, Content: "Integrity check failed", IsError: true}
	}

	// Chronicle snapshot.
	if _, snapErr := e.chronicle.Snapshot(&chronicle.ActionRequest{Type: string(action.Type), Payload: action.Payload}); snapErr != nil {
		e.log.Warn("chronicle_snapshot_failed", "error", snapErr)
	}

	// IFC check.
	if action.DataClassification != nil && !ifc.IsFlowAllowed(action.DataClassification, action.Type) {
		return &pb.ToolResultDelivery{CallId: tp.CallId, Content: "Blocked: IFC violation", IsError: true}
	}

	// Execute.
	start := time.Now()
	var result *types.ActionResult

	if e.mcpManager != nil {
		if client, toolName, isMCP := e.mcpManager.Route(tp.ToolName); isMCP {
			mcpResult, mcpErr := client.CallTool(ctx, toolName, args)
			if mcpErr != nil {
				result = &types.ActionResult{RequestID: action.RequestID, Success: false, Error: mcpErr.Error(), Summary: "MCP call failed"}
			} else {
				result = &types.ActionResult{RequestID: action.RequestID, Success: true, Output: mcpResult, Summary: "MCP call completed"}
			}
			result.DurationMs = time.Since(start).Milliseconds()
		}
	}

	if result == nil {
		result = e.executors.Execute(ctx, action)
		result.DurationMs = time.Since(start).Milliseconds()
	}

	// Audit and metrics (skip for OTR except metrics).
	if !isOTRAction {
		if result.Success {
			e.auditLog(audit.Entry{EventType: types.AuditActionExecuted, SessionID: sid, ActionType: string(action.Type), Details: result.Summary})
		} else {
			e.auditLog(audit.Entry{EventType: types.AuditActionFailed, SessionID: sid, ActionType: string(action.Type), Details: result.Error})
		}
	}
	if result.Success {
		e.db.IncrementDailyMetric("tool_success", 1)
	} else {
		e.db.IncrementDailyMetric("tool_failed", 1)
	}
	e.db.IncrementDailyMetric("tool_calls", 1)
	e.db.IncrementDailyMetric("tool:"+tp.ToolName, 1)

	e.broadcaster.Broadcast(&PipelineEvent{
		SessionID: sid, MessageID: mid, Type: EventActionCompleted,
		ActionCompleted: &ActionCompletedEvent{ToolName: tp.ToolName, Success: result.Success, Summary: result.Summary},
	})

	content := result.Output
	if !result.Success {
		content = result.Error
	}
	return &pb.ToolResultDelivery{CallId: tp.CallId, Content: content, IsError: !result.Success}
}

func truncateForLog(s string) string {
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}
