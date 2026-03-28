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

	cfg       *types.AgentConfig
	llm       llm.Provider
	log       *logging.Logger
	agent     *agent.Agent
	executors *executors.Registry
	shield    *shield.Pipeline
	enricher  *shield.MetadataEnricher
	chronicle *chronicle.Chronicle
	memory    *memory.Manager
	audit     *audit.Logger
	verifier  *Verifier
	db        *storage.DB

	server   *grpc.Server
	listener net.Listener

	mu       sync.Mutex
	shutdown bool
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

	registry := executors.NewRegistry(cfg.Workspace)
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
	ag := agent.NewAgent(provider, cfg.Workspace, registry.AvailableActions())

	return &Engine{
		cfg:       cfg,
		llm:       provider,
		log:       log,
		agent:     ag,
		executors: registry,
		shield:    shieldPipeline,
		enricher:  shield.NewMetadataEnricher(),
		chronicle: chron,
		memory:    mem,
		audit:     auditLogger,
		verifier:  NewVerifier(),
		db:        db,
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
// Single tool-use LLM call with interception loop.
func (e *Engine) ProcessMessage(req *pb.ProcessMessageRequest, stream pb.PipelineService_ProcessMessageServer) error {
	ctx := stream.Context()
	sid := req.SessionId
	mid := req.MessageId
	isOTR := req.Mode == pb.SessionMode_OTR
	mode := types.SessionNormal
	if isOTR {
		mode = types.SessionOTR
	}

	e.storeMessage(sid, mid, "user", req.Content)
	e.log.Info("message_received", "session", sid, "length", len(req.Content))

	// Load history.
	history := e.getHistory(sid)

	// Build system prompt with OTR awareness.
	systemPrompt, err := e.agent.Context.Assemble(mode)
	if err != nil {
		return e.sendError(stream, sid, mid, "CONTEXT_FAILED", err.Error())
	}

	// Compact history if approaching context limits.
	// Reserve 30% for the current turn (system prompt + user message + tool results + response).
	contextBudget := e.llm.EstimateTokens(systemPrompt) + 4096
	history, _ = e.agent.CompactHistory(ctx, history, contextBudget)

	// Load tool definitions (filtered for OTR).
	allTools := agent.GenerateToolDefinitions(e.executors.AllToolSchemas())
	tools := allTools
	if isOTR {
		tools = agent.FilterToolsForOTR(allTools)
	}

	// Build messages: history + current user message.
	messages := make([]llm.ChatMessage, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, llm.ChatMessage{Role: "user", Content: req.Content})

	e.log.Info("llm_call_started", "session", sid, "provider", e.llm.Name(),
		"model", e.llm.Model(), "tools", len(tools), "history", len(messages))

	// Initialize streaming redactor.
	redactor := NewStreamingRedactor(func(text string) {
		_ = stream.Send(&pb.PipelineEvent{
			SessionId: sid, MessageId: mid,
			EventType: pb.PipelineEventType_LLM_TOKEN,
			LlmToken:  &pb.LLMToken{Text: text},
		})
	})

	// Start tool-use stream.
	toolStream, err := e.llm.StreamWithTools(ctx, messages, tools,
		llm.WithSystem(systemPrompt), llm.WithMaxTokens(4096))
	if err != nil {
		e.log.Error("llm_call_failed", "session", sid, "error", err)
		return e.sendError(stream, sid, mid, "LLM_CALL_FAILED", err.Error())
	}
	defer func() { _ = toolStream.Close() }()

	// Main orchestration loop.
	var toolResults []llm.ToolResult
	var executedActions []*types.ActionRequest
	var executedResults []*types.ActionResult
	rounds := 0

	for rounds < maxToolRounds {
		event, eventErr := toolStream.Next()
		if eventErr == io.EOF || event.Type == llm.EventDone {
			// Stream ended. If we have pending tool results, send them
			// back and continue with a new stream.
			if len(toolResults) > 0 {
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

		case llm.EventToolCallComplete:
			redactor.Flush()
			tc := event.ToolCall

			e.log.Info("tool_call_received", "session", sid, "tool", tc.Name, "call_id", tc.ID)

			result := e.processToolCall(ctx, tc, mode, sid, mid, stream)
			e.log.Debug("tool_result", "call_id", result.CallID, "is_error", result.IsError, "content_len", len(result.Content))
			toolResults = append(toolResults, result)

			// Track for daily action log.
			executedActions = append(executedActions, &types.ActionRequest{Type: types.ActionType(tc.Name), Payload: tc.Arguments})
			executedResults = append(executedResults, &types.ActionResult{Success: !result.IsError, Summary: truncateForLog(result.Content)})
		}
	}

	// Flush remaining buffered text.
	redactor.Flush()

	// Get full response text.
	fullResponse := toolStream.FullText()

	// Store assistant response.
	e.storeMessage(sid, "", "assistant", fullResponse)

	// Emit response complete.
	_ = stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType:        pb.PipelineEventType_RESPONSE_COMPLETE,
		ResponseComplete: &pb.ResponseComplete{Content: fullResponse},
	})

	// Daily action log (Normal mode only, if tools were used).
	if !isOTR && len(executedActions) > 0 {
		e.memory.LogAction(executedActions, executedResults)
	}

	e.log.Info("message_complete", "session", sid, "response_length", len(fullResponse), "rounds", rounds)

	return nil
}

// truncateForLog truncates a string for log entries.
func truncateForLog(s string) string {
	if len(s) > 100 {
		return s[:100] + "..."
	}
	return s
}

// processToolCall handles a single tool call through the full security pipeline.
// Returns a ToolResult to send back to the LLM.
func (e *Engine) processToolCall(ctx context.Context, tc *llm.ToolCall, mode types.SessionMode, sid, mid string, stream pb.PipelineService_ProcessMessageServer) llm.ToolResult {
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
		_ = stream.Send(&pb.PipelineEvent{
			SessionId: sid, MessageId: mid,
			EventType:       pb.PipelineEventType_ACTION_COMPLETED,
			ActionCompleted: &pb.ActionCompleted{Success: false, Summary: "Blocked: " + protReason},
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
	_ = stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType: pb.PipelineEventType_ACTION_STARTED,
		ActionStarted: &pb.ActionStarted{
			Summary: formatToolCallSummary(tc),
		},
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
		_ = stream.Send(&pb.PipelineEvent{
			SessionId: sid, MessageId: mid,
			EventType:  pb.PipelineEventType_OTR_BLOCKED,
			OtrBlocked: &pb.OTRBlocked{Reason: reason},
		})
		return llm.ToolResult{CallID: tc.ID, Content: "Blocked: " + reason, IsError: true}
	}

	// Shield evaluation.
	verdict := e.shield.Evaluate(ctx, action)
	_ = stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType: pb.PipelineEventType_SHIELD_VERDICT,
		ShieldVerdict: &pb.ShieldVerdict{
			Decision: verdictToProto(verdict.Decision), Tier: int32(verdict.Tier),
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
		_ = stream.Send(&pb.PipelineEvent{
			SessionId: sid, MessageId: mid,
			EventType:       pb.PipelineEventType_ACTION_COMPLETED,
			ActionCompleted: &pb.ActionCompleted{Success: false, Summary: "Blocked: " + verdict.Reasoning},
		})
		return llm.ToolResult{CallID: tc.ID, Content: "Blocked by security: " + verdict.Reasoning, IsError: true}
	}

	if verdict.Decision == types.VerdictEscalate {
		e.log.Info("shield_escalate", "session", sid, "tool", tc.Name)
		return llm.ToolResult{CallID: tc.ID, Content: "Requires human approval (not yet implemented)", IsError: true}
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

	// Execute.
	e.log.Info("executor_start", "session", sid, "tool", tc.Name)
	start := time.Now()
	result := e.executors.Execute(ctx, action)
	result.DurationMs = time.Since(start).Milliseconds()

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

	_ = stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType:       pb.PipelineEventType_ACTION_COMPLETED,
		ActionCompleted: &pb.ActionCompleted{Success: result.Success, Summary: result.Summary},
	})

	if result.Artifact != nil {
		_ = stream.Send(&pb.PipelineEvent{
			SessionId: sid, MessageId: mid,
			EventType:      pb.PipelineEventType_ACTION_ARTIFACT,
			ActionArtifact: &pb.ActionArtifact{Artifact: toProtoArtifact(result.Artifact)},
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
	_ = e.db.InsertMessage(&types.Message{
		ID: messageID, SessionID: sessionID,
		Role: role, Content: content, Timestamp: time.Now(),
	})
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

func (e *Engine) sendError(stream pb.PipelineService_ProcessMessageServer, sid, mid, code, message string) error {
	return stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType: pb.PipelineEventType_ERROR,
		PipelineError: &pb.PipelineError{
			Code: code, Message: message, Recoverable: true,
		},
	})
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

func verdictToProto(d types.VerdictDecision) pb.VerdictDecision {
	switch d {
	case types.VerdictAllow:
		return pb.VerdictDecision_ALLOW
	case types.VerdictBlock:
		return pb.VerdictDecision_BLOCK
	case types.VerdictEscalate:
		return pb.VerdictDecision_ESCALATE
	default:
		return pb.VerdictDecision_VERDICT_DECISION_UNSPECIFIED
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
