// Package engine implements the privileged execution engine that evaluates
// and executes agent-proposed actions. The engine runs as a separate OS process
// and communicates with the agent via gRPC.
package engine

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/openparallax/openparallax/internal/agent"
	"github.com/openparallax/openparallax/internal/audit"
	"github.com/openparallax/openparallax/internal/chronicle"
	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/engine/executors"
	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/mcp"
	"github.com/openparallax/openparallax/internal/memory"
	"github.com/openparallax/openparallax/internal/session"
	"github.com/openparallax/openparallax/internal/shield"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	pb "github.com/openparallax/openparallax/internal/types/pb"
	"google.golang.org/grpc"
)

// maxToolRounds limits the number of tool-call round-trips per message
// to prevent infinite loops.
const maxToolRounds = 25

// Engine is the execution engine and gRPC server.
type Engine struct {
	pb.UnimplementedPipelineServiceServer

	cfg        *types.AgentConfig
	llm        llm.Provider
	log        *logging.Logger
	agent      *agent.Agent
	executors  *executors.Registry
	shield     *shield.Pipeline
	enricher   *shield.MetadataEnricher
	chronicle  *chronicle.Chronicle
	memory     *memory.Manager
	audit      *audit.Logger
	verifier   *Verifier
	db         *storage.DB
	mcpManager *mcp.Manager

	server   *grpc.Server
	listener net.Listener

	sandboxStatus sandboxInfo

	mu       sync.Mutex
	shutdown bool
}

// sandboxInfo holds the kernel sandbox state for API reporting.
type sandboxInfo struct {
	Active     bool
	Mode       string
	Version    int
	Filesystem bool
	Network    bool
	Reason     string
}

// New creates an Engine from a config file path. When verbose is true,
// diagnostic output for each pipeline stage is written to engine.log.
func New(configPath string, verbose bool) (*Engine, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("config load: %w", err)
	}

	logLevel := logging.LevelInfo
	if verbose {
		logLevel = logging.LevelDebug
	}
	logPath := filepath.Join(cfg.Workspace, ".openparallax", "engine.log")
	log, err := logging.New(logPath, logLevel)
	if err != nil {
		log = logging.Nop()
	}

	provider, err := llm.NewProvider(cfg.LLM)
	if err != nil {
		return nil, fmt.Errorf("llm provider: %w", err)
	}

	dbPath := filepath.Join(cfg.Workspace, ".openparallax", "openparallax.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	db.RepairSessions()
	registry := executors.NewRegistry(cfg.Workspace, cfg, log)
	canaryToken := readCanaryToken(cfg.Workspace)

	configDir := filepath.Dir(configPath)
	policyFile := resolveFilePath(cfg.Shield.PolicyFile, configDir, cfg.Workspace)
	promptPath := resolveFilePath("prompts/evaluator-v1.md", configDir, cfg.Workspace)

	shieldPipeline, err := shield.NewPipeline(shield.Config{
		PolicyFile:       policyFile,
		OnnxThreshold:    cfg.Shield.OnnxThreshold,
		HeuristicEnabled: cfg.Shield.HeuristicEnabled,
		ClassifierAddr:   cfg.Shield.ClassifierAddr,
		Evaluator:        &cfg.Shield.Evaluator,
		CanaryToken:      canaryToken,
		PromptPath:       promptPath,
		FailClosed:       cfg.General.FailClosed,
		RateLimit:        cfg.General.RateLimit,
		VerdictTTL:       cfg.General.VerdictTTLSeconds,
		DailyBudget:      cfg.General.DailyBudget,
		Log:              nil, // Shield uses its own logging internally
	})
	if err != nil {
		return nil, fmt.Errorf("shield init: %w", err)
	}

	auditPath := filepath.Join(cfg.Workspace, ".openparallax", "audit.jsonl")
	auditLogger, err := audit.NewLogger(auditPath)
	if err != nil {
		return nil, fmt.Errorf("audit logger: %w", err)
	}
	auditLogger.SetDB(db)

	chron, err := chronicle.New(cfg.Workspace, cfg.Chronicle, db)
	if err != nil {
		return nil, fmt.Errorf("chronicle: %w", err)
	}

	mem := memory.NewManager(cfg.Workspace, db, provider)
	registry.RegisterMemory(mem)
	ag := agent.NewAgent(provider, cfg.Workspace, mem)

	// MCP manager (optional — only if servers are configured).
	var mcpMgr *mcp.Manager
	if len(cfg.MCP.Servers) > 0 {
		mcpMgr = mcp.NewManager(cfg.MCP.Servers, log)
	}

	return &Engine{
		cfg:        cfg,
		llm:        provider,
		log:        log,
		agent:      ag,
		executors:  registry,
		shield:     shieldPipeline,
		enricher:   shield.NewMetadataEnricher(),
		chronicle:  chron,
		memory:     mem,
		audit:      auditLogger,
		verifier:   NewVerifier(),
		db:         db,
		mcpManager: mcpMgr,
	}, nil
}

// Start begins the gRPC server on a dynamic port.
func (e *Engine) Start() (int, error) {
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, fmt.Errorf("failed to listen: %w", err)
	}
	e.listener = lis

	e.server = grpc.NewServer()
	pb.RegisterPipelineServiceServer(e.server, e)

	go func() {
		_ = e.server.Serve(lis)
	}()

	tcpAddr, ok := lis.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("listener address is not TCP")
	}
	e.log.Info("grpc_server_started", "port", tcpAddr.Port)
	return tcpAddr.Port, nil
}

// Stop gracefully shuts down the engine.
func (e *Engine) Stop() {
	e.mu.Lock()
	e.shutdown = true
	e.mu.Unlock()

	if e.mcpManager != nil {
		e.mcpManager.ShutdownAll()
	}
	if e.server != nil {
		e.server.GracefulStop()
	}
	if e.audit != nil {
		_ = e.audit.Close()
	}
	if e.db != nil {
		_ = e.db.Close()
	}
	e.log.Info("grpc_server_shutdown")
}

// Port returns the port the engine is listening on, or 0 if not started.
func (e *Engine) Port() int {
	if e.listener == nil {
		return 0
	}
	tcpAddr, ok := e.listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0
	}
	return tcpAddr.Port
}

// ProcessMessage implements the PipelineService gRPC method.
// Thin wrapper around processMessageCore using a gRPC event sender.
func (e *Engine) ProcessMessage(req *pb.ProcessMessageRequest, stream pb.PipelineService_ProcessMessageServer) error {
	mode := types.SessionNormal
	if req.Mode == pb.SessionMode_OTR {
		mode = types.SessionOTR
	}
	sender := newGRPCEventSender(stream)
	return e.processMessageCore(stream.Context(), sender, req.SessionId, req.MessageId, req.Content, mode)
}

// ProcessMessageForWeb is the public entry point for the web server.
// It uses a transport-neutral EventSender to deliver pipeline events.
func (e *Engine) ProcessMessageForWeb(ctx context.Context, sender EventSender, sid, mid, content string, mode types.SessionMode) error {
	return e.processMessageCore(ctx, sender, sid, mid, content, mode)
}

// processMessageCore is the shared pipeline logic for both gRPC and WebSocket.
func (e *Engine) processMessageCore(ctx context.Context, sender EventSender, sid, mid, content string, mode types.SessionMode) error {
	isOTR := mode == types.SessionOTR

	e.storeMessage(sid, mid, "user", content)
	e.log.Info("message_received", "session", sid, "length", len(content))

	// Load history.
	history := e.getHistory(sid)

	// Build system prompt with OTR awareness and skills.
	skillSummary := ""
	activeSkills := ""
	if e.agent.Skills != nil {
		skillSummary = e.agent.Skills.LightSummary()
		activeSkills = e.agent.Skills.ActiveSkillBodies()
	}
	systemPrompt, err := e.agent.Context.AssembleWithSkills(mode, skillSummary, activeSkills)
	if err != nil {
		return e.sendErrorEvent(sender, sid, mid, "CONTEXT_FAILED", err.Error())
	}

	// Compact history if approaching context limits.
	contextBudget := e.llm.EstimateTokens(systemPrompt) + 4096
	history, _ = e.agent.CompactHistory(ctx, history, contextBudget)

	// Load tool definitions (filtered for OTR).
	allTools := agent.GenerateToolDefinitions(e.executors.AllToolSchemas())
	if e.mcpManager != nil {
		mcpTools := e.mcpManager.DiscoverTools(ctx)
		allTools = append(allTools, mcpTools...)
	}
	tools := allTools
	if isOTR {
		tools = agent.FilterToolsForOTR(allTools)
	}

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
	var executedActions []*types.ActionRequest
	var executedResults []*types.ActionResult
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

			executedActions = append(executedActions, &types.ActionRequest{Type: types.ActionType(tc.Name), Payload: tc.Arguments})
			executedResults = append(executedResults, &types.ActionResult{Success: !result.IsError, Summary: executors.Truncate(result.Content, 100)})

			if e.agent.Skills != nil {
				e.agent.Skills.MatchSkills([]string{tc.Name})
			}
		}
	}

	redactor.Flush()
	fullResponse := toolStream.FullText()

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

	if !isOTR && len(executedActions) > 0 {
		e.memory.LogAction(executedActions, executedResults)
	}

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

	e.log.Info("message_complete", "session", sid, "response_length", len(fullResponse), "rounds", rounds)
	return nil
}

func truncateForLog(s string) string {
	if len(s) > 100 {
		return s[:100] + "..."
	}
	return s
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
	_ = e.audit.Log(audit.Entry{
		EventType: types.AuditActionProposed, SessionID: sid,
		ActionType: string(action.Type), Details: "hash: " + action.Hash, OTR: isOTR,
	})

	// OTR check (defense in depth — primary enforcement is tool filtering).
	if isOTR && !session.IsOTRAllowed(action.Type) {
		reason := session.OTRBlockReason(action.Type)
		e.log.Info("otr_blocked", "session", sid, "tool", tc.Name)
		_ = e.audit.Log(audit.Entry{
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
	verdict := e.shield.Evaluate(ctx, action)
	_ = sender.SendEvent(&PipelineEvent{
		SessionID: sid, MessageID: mid, Type: EventShieldVerdict,
		ShieldVerdict: &ShieldVerdictEvent{
			ToolName: tc.Name, Decision: string(verdict.Decision), Tier: verdict.Tier,
			Confidence: verdict.Confidence, Reasoning: verdict.Reasoning,
		},
	})
	_ = e.audit.Log(audit.Entry{
		EventType: types.AuditActionEvaluated, SessionID: sid,
		ActionType: string(action.Type),
		Details:    fmt.Sprintf("%s (tier %d): %s", verdict.Decision, verdict.Tier, verdict.Reasoning),
	})

	if verdict.Decision == types.VerdictBlock {
		e.log.Info("shield_blocked", "session", sid, "tool", tc.Name, "reason", verdict.Reasoning)
		_ = e.audit.Log(audit.Entry{
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
		e.log.Info("shield_escalate", "session", sid, "tool", tc.Name)
		return llm.ToolResult{CallID: tc.ID, Content: "Action requires human approval — escalation is not available in this session", IsError: true}
	}

	// Hash verification.
	if verifyErr := e.verifier.Verify(action); verifyErr != nil {
		e.log.Error("hash_verify_failed", "session", sid, "tool", tc.Name)
		return llm.ToolResult{CallID: tc.ID, Content: "Integrity check failed", IsError: true}
	}

	// Chronicle snapshot (Normal mode only).
	if !isOTR {
		if _, snapErr := e.chronicle.Snapshot(action); snapErr != nil {
			e.log.Warn("chronicle_snapshot_failed", "session", sid, "error", snapErr)
		}
	}

	// IFC check: if the action sends data externally and we've seen sensitive
	// data in this session, block the flow.
	if action.DataClassification != nil && !shield.IsFlowAllowed(action.DataClassification, action.Type) {
		reason := "IFC violation: sensitive data cannot flow to this destination"
		e.log.Info("ifc_blocked", "session", sid, "tool", tc.Name,
			"sensitivity", action.DataClassification.Sensitivity, "source", action.DataClassification.SourcePath)
		_ = e.audit.Log(audit.Entry{
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

	if result.Success {
		e.log.Info("executor_complete", "session", sid, "tool", tc.Name, "success", true, "ms", result.DurationMs)
		_ = e.audit.Log(audit.Entry{
			EventType: types.AuditActionExecuted, SessionID: sid,
			ActionType: string(action.Type), Details: result.Summary,
		})
	} else {
		e.log.Info("executor_complete", "session", sid, "tool", tc.Name, "success", false, "error", result.Error)
		_ = e.audit.Log(audit.Entry{
			EventType: types.AuditActionFailed, SessionID: sid,
			ActionType: string(action.Type), Details: result.Error,
		})
	}

	_ = sender.SendEvent(&PipelineEvent{
		SessionID: sid, MessageID: mid, Type: EventActionCompleted,
		ActionCompleted: &ActionCompletedEvent{ToolName: tc.Name, Success: result.Success, Summary: result.Summary},
	})

	if result.Artifact != nil {
		_ = sender.SendEvent(&PipelineEvent{
			SessionID: sid, MessageID: mid, Type: EventActionArtifact,
			ActionArtifact: &ActionArtifactEvent{Artifact: result.Artifact},
		})
	}

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

// --- gRPC RPC implementations ---

// GetStatus implements the PipelineService gRPC method.
func (e *Engine) GetStatus(_ context.Context, _ *pb.StatusRequest) (*pb.StatusResponse, error) {
	sessionCount, _ := e.db.SessionCount()
	agentName := e.cfg.Identity.Name
	if agentName == "" {
		agentName = types.DefaultIdentity.Name
	}
	return &pb.StatusResponse{
		AgentName:    agentName,
		Model:        e.llm.Model(),
		SessionCount: int32(sessionCount),
	}, nil
}

// Shutdown implements the PipelineService gRPC method.
func (e *Engine) Shutdown(ctx context.Context, _ *pb.ShutdownRequest) (*pb.ShutdownResponse, error) {
	// Summarize active sessions before shutdown.
	e.summarizeActiveSessions(ctx)

	go func() {
		time.Sleep(100 * time.Millisecond)
		e.Stop()
	}()
	return &pb.ShutdownResponse{Clean: true}, nil
}

// summarizeActiveSessions generates summaries for sessions with sufficient history.
func (e *Engine) summarizeActiveSessions(ctx context.Context) {
	sessions, err := e.db.ListSessions()
	if err != nil {
		e.log.Warn("summarize_sessions_failed", "error", err)
		return
	}
	for _, sess := range sessions {
		history := e.getHistory(sess.ID)
		if len(history) < 4 {
			continue
		}
		if err := e.memory.SummarizeSession(ctx, "", history); err != nil {
			e.log.Warn("session_summarize_failed", "session", sess.ID, "error", err)
		} else {
			e.log.Info("session_summarized", "session", sess.ID)
		}
	}
}

// ReadMemory implements the PipelineService gRPC method.
func (e *Engine) ReadMemory(_ context.Context, req *pb.MemoryReadRequest) (*pb.MemoryReadResponse, error) {
	content, err := e.memory.Read(types.MemoryFileType(req.FileType))
	if err != nil {
		return nil, err
	}
	return &pb.MemoryReadResponse{
		Content: content,
		Path:    filepath.Join(e.cfg.Workspace, req.FileType),
	}, nil
}

// SearchMemory implements the PipelineService gRPC method.
func (e *Engine) SearchMemory(_ context.Context, req *pb.MemorySearchRequest) (*pb.MemorySearchResponse, error) {
	results, err := e.memory.Search(req.Query, int(req.Limit))
	if err != nil {
		return nil, err
	}
	var pbResults []*pb.MemorySearchResult
	for _, r := range results {
		pbResults = append(pbResults, &pb.MemorySearchResult{
			Path: r.Path, Section: r.Section, Snippet: r.Snippet, Score: r.Score,
		})
	}
	return &pb.MemorySearchResponse{Results: pbResults}, nil
}

// --- Internal helpers ---

func (e *Engine) storeMessage(sessionID, messageID, role, content string) {
	if messageID == "" {
		messageID = fmt.Sprintf("msg-%d", time.Now().UnixNano())
	}
	e.ensureSession(sessionID)
	_ = e.db.InsertMessage(&types.Message{
		ID: messageID, SessionID: sessionID,
		Role: role, Content: content, Timestamp: time.Now(),
	})
}

// storeAssistantMessage saves an assistant message with thoughts (reasoning + tool calls).
func (e *Engine) storeAssistantMessage(sessionID string, msg *types.Message) {
	if msg.ID == "" {
		msg.ID = fmt.Sprintf("msg-%d", time.Now().UnixNano())
	}
	msg.SessionID = sessionID
	e.ensureSession(sessionID)
	_ = e.db.InsertMessage(msg)
}

// ensureSession creates the session if it doesn't exist.
func (e *Engine) ensureSession(sessionID string) {
	if _, err := e.db.GetSession(sessionID); err != nil {
		_ = e.db.InsertSession(&types.Session{
			ID:        sessionID,
			Mode:      types.SessionNormal,
			CreatedAt: time.Now(),
		})
	}
}

// generateSessionTitle asks the LLM for a short headline summarizing the conversation.
func (e *Engine) generateSessionTitle(sessionID string, history []llm.ChatMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var summary strings.Builder
	for _, m := range history {
		if len(summary.String()) > 400 {
			break
		}
		fmt.Fprintf(&summary, "%s: %s\n", m.Role, truncateForLog(m.Content))
	}

	prompt := fmt.Sprintf(
		"Generate a short title (max 6 words) summarizing this conversation's topic:\n\n%s\nRespond with ONLY the title, no quotes, no punctuation at the end.",
		summary.String(),
	)

	title, err := e.llm.Complete(ctx, prompt)
	if err != nil {
		e.log.Debug("session_title_generation_failed", "session", sessionID, "error", err)
		return
	}

	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'")
	if len(title) > 60 {
		title = title[:60]
	}
	if title == "" {
		return
	}

	_ = e.db.UpdateSessionTitle(sessionID, title)
	e.log.Debug("session_titled", "session", sessionID, "title", title)
}

func (e *Engine) getHistory(sessionID string) []llm.ChatMessage {
	messages, err := e.db.GetMessages(sessionID)
	if err != nil {
		return nil
	}
	result := make([]llm.ChatMessage, 0, len(messages))
	for _, m := range messages {
		result = append(result, llm.ChatMessage{Role: m.Role, Content: m.Content})
	}
	return result
}

func (e *Engine) sendErrorEvent(sender EventSender, sid, mid, code, message string) error {
	return sender.SendEvent(&PipelineEvent{
		SessionID: sid, MessageID: mid, Type: EventError,
		Error: &PipelineErrorEvent{Code: code, Message: message, Recoverable: true},
	})
}

// --- Accessors for the web server ---

// DB returns the storage database.
func (e *Engine) DB() *storage.DB { return e.db }

// Memory returns the memory manager.
func (e *Engine) Memory() *memory.Manager { return e.memory }

// Config returns the agent configuration.
func (e *Engine) Config() *types.AgentConfig { return e.cfg }

// Log returns the logger.
func (e *Engine) Log() *logging.Logger { return e.log }

// LLMModel returns the configured LLM model name.
func (e *Engine) LLMModel() string { return e.llm.Model() }

// ToolList returns all available tools grouped by name.
func (e *Engine) ToolList() []map[string]string {
	schemas := e.executors.AllToolSchemas()
	tools := make([]map[string]string, 0, len(schemas))
	for _, s := range schemas {
		tools = append(tools, map[string]string{
			"name":        s.Name,
			"description": s.Description,
		})
	}
	return tools
}

// ShieldStatus returns the current Shield operational state.
func (e *Engine) ShieldStatus() map[string]any {
	s := e.shield.Status()
	return map[string]any{
		"active":        s.Active,
		"tier2_used":    s.Tier2Used,
		"tier2_budget":  s.Tier2Budget,
		"tier2_enabled": s.Tier2Enabled,
	}
}

// SandboxStatus returns the current kernel sandbox state for the Agent process.
func (e *Engine) SandboxStatus() map[string]any {
	s := e.sandboxStatus
	return map[string]any{
		"active":     s.Active,
		"mode":       s.Mode,
		"version":    s.Version,
		"filesystem": s.Filesystem,
		"network":    s.Network,
		"reason":     s.Reason,
	}
}

// SetSandboxStatus stores the sandbox probe result for API reporting.
func (e *Engine) SetSandboxStatus(active bool, mode string, version int, filesystem, network bool, reason string) {
	e.sandboxStatus = sandboxInfo{
		Active:     active,
		Mode:       mode,
		Version:    version,
		Filesystem: filesystem,
		Network:    network,
		Reason:     reason,
	}
}

// ConfigPath returns the path to the config.yaml file.
func (e *Engine) ConfigPath() string {
	return filepath.Join(e.cfg.Workspace, "config.yaml")
}

// MCPServerStatus returns the status of all configured MCP servers.
func (e *Engine) MCPServerStatus() []map[string]any {
	if e.mcpManager == nil {
		return nil
	}
	return e.mcpManager.ServerStatus()
}

// UpdateIdentity applies new agent identity settings in-memory.
func (e *Engine) UpdateIdentity(name, avatar string) {
	if name != "" {
		e.cfg.Identity.Name = name
	}
	if avatar != "" {
		e.cfg.Identity.Avatar = avatar
	}
}

// UpdateShieldBudget changes the daily Tier 2 budget in-memory.
func (e *Engine) UpdateShieldBudget(budget int) {
	e.cfg.General.DailyBudget = budget
	e.shield.UpdateBudget(budget)
}

// LogPath returns the path to the engine log file.
func (e *Engine) LogPath() string {
	return filepath.Join(e.cfg.Workspace, ".openparallax", "engine.log")
}

// AuditPath returns the path to the audit JSONL file.
func (e *Engine) AuditPath() string {
	return filepath.Join(e.cfg.Workspace, ".openparallax", "audit.jsonl")
}

// --- Conversion helpers ---

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

func toProtoArtifact(a *types.Artifact) *pb.Artifact {
	return &pb.Artifact{
		Id: a.ID, Type: a.Type, Title: a.Title, Path: a.Path,
		Content: a.Content, Language: a.Language,
		SizeBytes: a.SizeBytes, PreviewType: a.PreviewType,
	}
}

func readCanaryToken(workspace string) string {
	canaryPath := filepath.Join(workspace, ".openparallax", "canary.token")
	data, err := os.ReadFile(canaryPath)
	if err == nil && len(data) > 0 {
		return string(data)
	}
	token, _ := crypto.GenerateCanary()
	return token
}

func resolveFilePath(path, configDir, workspace string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	for _, base := range []string{configDir, workspace, "."} {
		candidate := filepath.Join(base, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join(configDir, path)
}
