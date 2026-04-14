package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openparallax/openparallax/audit"
	"github.com/openparallax/openparallax/chronicle"
	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/ifc"
	"github.com/openparallax/openparallax/internal/session"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/llm"
)

// ProcessMessageForWeb is the public entry point for the web server and
// channel adapters. It subscribes the sender for events and forwards the
// message to the sandboxed Agent via the gRPC stream.
func (e *Engine) ProcessMessageForWeb(ctx context.Context, sender EventSender, sid, mid, content string, mode types.SessionMode) error {
	if mode != types.SessionOTR {
		e.storeMessage(sid, mid, "user", content)
	}

	clientID := fmt.Sprintf("ws-%p", sender)
	e.broadcaster.Subscribe(clientID, sid, sender)

	if err := e.forwardToAgent(sid, mid, content, mode, "web"); err != nil {
		e.log.Error("forward_to_agent_failed", "session", sid, "error", err)
		return e.sendErrorEvent(sender, sid, mid, "AGENT_UNAVAILABLE", err.Error())
	}

	<-ctx.Done()
	return nil
}

// processToolCall handles a single tool call through the full security pipeline.
// Used by the agent stream (handleToolProposal) and sub-agent tool execution.
func (e *Engine) processToolCall(ctx context.Context, tc *llm.ToolCall, mode types.SessionMode, sid, mid string, sender EventSender) llm.ToolResult {
	action := &types.ActionRequest{
		RequestID: crypto.NewID(),
		Type:      types.ActionType(tc.Name),
		Payload:   tc.Arguments,
		Timestamp: time.Now(),
	}
	hash, _ := crypto.HashAction(tc.Name, tc.Arguments)
	action.Hash = hash

	isOTR := mode == types.SessionOTR

	// IFC classification — policy-driven.
	action.DataClassification = e.ifcPolicy.ClassifyWithActivity(actionPath(action), e.db.LookupIFCClassification)
	if !isOTR && action.DataClassification != nil {
		e.auditLog(audit.Entry{
			EventType: types.AuditIFCClassified, SessionID: sid,
			ActionType: string(action.Type),
			Details: fmt.Sprintf("sensitivity=%s source=%s",
				action.DataClassification.Sensitivity,
				action.DataClassification.SourcePath),
		})
		e.db.IncrementDailyMetric("ifc_classification_"+action.DataClassification.Sensitivity.String(), 1)
	}

	// Session taint: record from classified data, apply to unclassified actions.
	if action.DataClassification != nil {
		e.recordSessionTaint(sid, action.DataClassification.Sensitivity)
	} else {
		sessionSens := e.getSessionTaint(sid)
		if sessionSens > ifc.SensitivityPublic {
			action.DataClassification = &ifc.DataClassification{
				Sensitivity: sessionSens,
				SourcePath:  "(session taint)",
			}
		}
	}

	allowed, protection, protReason := CheckProtection(action, e.cfg.Workspace)
	if !allowed {
		e.log.Warn("protection_blocked", "session", sid, "tool", tc.Name, "reason", protReason)
		e.db.IncrementDailyMetric("protection_layer_blocks", 1)
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

	_ = sender.SendEvent(&PipelineEvent{
		SessionID: sid, MessageID: mid, Type: EventActionStarted,
		ActionStarted: &ActionStartedEvent{ToolName: tc.Name, Summary: formatToolCallSummary(tc)},
	})

	e.auditLog(audit.Entry{
		EventType: types.AuditActionProposed, SessionID: sid,
		ActionType: string(action.Type), Details: "hash: " + action.Hash, OTR: isOTR,
	})

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

	shieldStart := time.Now()
	verdict := e.shield.Evaluate(ctx, action)
	shieldMs := time.Since(shieldStart).Milliseconds()
	e.db.AddLatencySample(fmt.Sprintf("shield_t%d", verdict.Tier), shieldMs)

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

	e.db.IncrementDailyMetric("shield_"+strings.ToLower(string(verdict.Decision)), 1)
	e.db.IncrementDailyMetric(fmt.Sprintf("shield_t%d", verdict.Tier), 1)

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

	if verifyErr := e.verifier.Verify(action); verifyErr != nil {
		e.log.Error("hash_verify_failed", "session", sid, "tool", tc.Name, "error", verifyErr)
		return llm.ToolResult{CallID: tc.ID, Content: "Integrity check failed", IsError: true}
	}
	e.log.Debug("hash_verified", "session", sid, "tool", tc.Name, "hash", action.Hash[:16])

	if !isOTR {
		if snapMeta, snapErr := e.chronicle.Snapshot(&chronicle.ActionRequest{Type: string(action.Type), Payload: action.Payload}); snapErr != nil {
			e.log.Warn("chronicle_snapshot_failed", "session", sid, "error", snapErr)
			e.auditLog(audit.Entry{
				EventType: types.AuditChronicleSnapshotFailed, SessionID: sid,
				ActionType: string(action.Type), Details: snapErr.Error(),
			})
		} else if snapMeta != nil {
			e.log.Debug("chronicle_snapshot", "session", sid, "tool", tc.Name, "snapshot_id", snapMeta.ID)
			e.auditLog(audit.Entry{
				EventType: types.AuditChronicleSnapshot, SessionID: sid,
				ActionType: string(action.Type),
				Details:    "snapshot_id=" + snapMeta.ID,
			})
		}
	}

	// IFC flow check — policy-driven.
	if action.DataClassification != nil {
		ifcDecision := e.ifcPolicy.Decide(action.DataClassification, action.Type)
		if ifcDecision == ifc.DecisionBlock {
			reason := fmt.Sprintf("IFC violation: %s data cannot flow to %s", action.DataClassification.Sensitivity, action.Type)
			if e.ifcPolicy.Mode == ifc.ModeAudit {
				e.log.Info("ifc_audit_would_block", "session", sid, "tool", tc.Name,
					"sensitivity", action.DataClassification.Sensitivity, "source", action.DataClassification.SourcePath)
				e.auditLog(audit.Entry{
					EventType: types.AuditIFCAuditWouldBlock, SessionID: sid,
					ActionType: string(action.Type), Details: reason,
				})
			} else {
				e.log.Warn("ifc_blocked", "session", sid, "tool", tc.Name,
					"sensitivity", action.DataClassification.Sensitivity, "source", action.DataClassification.SourcePath)
				e.auditLog(audit.Entry{
					EventType: types.AuditIFCBlocked, SessionID: sid,
					ActionType: string(action.Type), Details: reason,
				})
				_ = sender.SendEvent(&PipelineEvent{
					SessionID: sid, MessageID: mid, Type: EventActionCompleted,
					ActionCompleted: &ActionCompletedEvent{ToolName: tc.Name, Success: false, Summary: "Blocked: " + reason},
				})
				return llm.ToolResult{CallID: tc.ID, Content: "Blocked: " + reason, IsError: true}
			}
		} else if ifcDecision == ifc.DecisionEscalate && action.MinTier < 2 {
			action.MinTier = 2
		}
	}

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
		// Record IFC activity for successful file writes with classified data.
		if action.DataClassification != nil && isTrackedWriteAction(action.Type) {
			if destPath := actionPath(action); destPath != "" {
				e.db.RecordIFCWrite(destPath,
					int(action.DataClassification.Sensitivity),
					action.DataClassification.SourcePath, sid)
			}
		}
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

	content := result.Summary
	if result.Output != "" {
		content = result.Output
	}
	if !result.Success && result.Output == "" {
		content = "Error: " + result.Error
	}
	content = e.sanitizeToolOutput(tc.Name, content)

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
