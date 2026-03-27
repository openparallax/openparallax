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
	"github.com/openparallax/openparallax/internal/memory"
	"github.com/openparallax/openparallax/internal/parser"
	"github.com/openparallax/openparallax/internal/plog"
	"github.com/openparallax/openparallax/internal/session"
	"github.com/openparallax/openparallax/internal/shield"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	pb "github.com/openparallax/openparallax/internal/types/pb"
	"google.golang.org/grpc"
)

// Engine is the execution engine and gRPC server.
type Engine struct {
	pb.UnimplementedPipelineServiceServer

	cfg       *types.AgentConfig
	llm       llm.Provider
	log       *plog.Logger
	parser    *parser.Parser
	agent     *agent.Agent
	executors *executors.Registry
	shield    *shield.Pipeline
	chronicle *chronicle.Chronicle
	memory    *memory.Manager
	audit     *audit.Logger
	verifier  *Verifier
	checker   *ResponseChecker
	db        *storage.DB

	server   *grpc.Server
	listener net.Listener

	mu       sync.Mutex
	shutdown bool
}

// New creates an Engine from a config file path. When verbose is true,
// diagnostic output for each pipeline stage is written to stderr.
func New(configPath string, verbose bool) (*Engine, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("config load: %w", err)
	}

	log := plog.New(verbose)

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
		Log:              log,
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

	ag := agent.NewAgent(provider, cfg.Workspace, registry.AvailableActions())

	return &Engine{
		cfg:       cfg,
		llm:       provider,
		log:       log,
		parser:    parser.New(provider),
		agent:     ag,
		executors: registry,
		shield:    shieldPipeline,
		chronicle: chron,
		memory:    mem,
		audit:     auditLogger,
		verifier:  NewVerifier(),
		checker:   NewResponseChecker(),
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
// Pipeline: parse → plan → self-eval → [OTR check → Shield → verify → snapshot → execute] → respond.
func (e *Engine) ProcessMessage(req *pb.ProcessMessageRequest, stream pb.PipelineService_ProcessMessageServer) error {
	ctx := stream.Context()
	sid := req.SessionId
	mid := req.MessageId
	isOTR := req.Mode == pb.SessionMode_OTR

	e.storeMessage(sid, mid, "user", req.Content)

	systemPrompt, err := e.agent.Context.Assemble()
	if err != nil {
		return e.sendError(stream, sid, mid, "CONTEXT_FAILED", err.Error())
	}

	history := e.getHistory(sid)

	// Parse intent.
	intent, err := e.parser.Parse(ctx, req.Content)
	if err != nil {
		e.log.Log("parser", "failed: %s, falling back to conversation", err)
		return e.streamResponse(ctx, stream, sid, mid, req.Content, systemPrompt, history, nil)
	}

	e.log.Log("parser", "intent: %s / %s (confidence: %.2f, destructive: %v)",
		intent.Goal, intent.PrimaryAction, intent.Confidence, intent.Destructive)
	e.emitIntentParsed(stream, sid, mid, intent)

	if intent.Goal == types.GoalConversation || intent.PrimaryAction == "conversation" {
		e.log.Log("parser", "conversation mode")
		return e.streamResponse(ctx, stream, sid, mid, req.Content, systemPrompt, history, nil)
	}

	// Plan and build actions.
	actions, err := e.agent.PlanActions(ctx, intent, systemPrompt, history)
	if err != nil || len(actions) == 0 {
		e.log.Log("planner", "no actions produced, falling back to conversation")
		return e.streamResponse(ctx, stream, sid, mid, req.Content, systemPrompt, history, nil)
	}

	e.log.Log("planner", "%d action(s) planned", len(actions))
	e.emitActionsPlanned(stream, sid, mid, len(actions))

	// Self-eval gate.
	passed, reason, _ := e.agent.SelfEval.Evaluate(ctx, actions, req.Content)
	e.emitSelfEval(stream, sid, mid, passed, reason)
	if !passed {
		e.log.Log("selfeval", "FAILED: %s", reason)
		_ = e.audit.Log(audit.Entry{
			EventType: types.AuditSelfProtection, SessionID: sid,
			Details: "self-eval blocked: " + reason,
		})
		return e.sendError(stream, sid, mid, "SELF_EVAL_FAILED", "Safety check failed: "+reason)
	}
	e.log.Log("selfeval", "passed")

	// Execute actions through security pipeline.
	results := e.executeActions(ctx, stream, sid, mid, actions, isOTR)

	// Daily log (Normal mode only).
	if !isOTR && len(actions) > 0 {
		e.memory.LogAction(actions, results)
	}

	e.log.Log("response", "streaming with %d action result(s)", len(results))
	return e.streamResponse(ctx, stream, sid, mid, req.Content, systemPrompt, history, results)
}

// executeActions runs each action through OTR → Shield → verify → snapshot → execute.
// Returns a result for every action (success, blocked, or failed).
func (e *Engine) executeActions(ctx context.Context, stream pb.PipelineService_ProcessMessageServer, sid, mid string, actions []*types.ActionRequest, isOTR bool) []*types.ActionResult {
	var results []*types.ActionResult

	for _, action := range actions {
		result := e.executeOneAction(ctx, stream, sid, mid, action, isOTR)
		results = append(results, result)
	}

	return results
}

// executeOneAction processes a single action through the full security pipeline.
func (e *Engine) executeOneAction(ctx context.Context, stream pb.PipelineService_ProcessMessageServer, sid, mid string, action *types.ActionRequest, isOTR bool) *types.ActionResult {
	// OTR enforcement.
	if isOTR && !session.IsOTRAllowed(action.Type) {
		reason := session.OTRBlockReason(action.Type)
		e.log.Log("otr", "%s -> BLOCKED", action.Type)
		_ = stream.Send(&pb.PipelineEvent{
			SessionId: sid, MessageId: mid,
			EventType:  pb.PipelineEventType_OTR_BLOCKED,
			OtrBlocked: &pb.OTRBlocked{Reason: reason},
		})
		_ = e.audit.Log(audit.Entry{
			EventType: types.AuditActionBlocked, SessionID: sid,
			ActionType: string(action.Type), Details: "OTR: " + reason, OTR: true,
		})
		return &types.ActionResult{
			RequestID: action.RequestID, Success: false,
			Error: reason, Summary: fmt.Sprintf("OTR blocked: %s", action.Type),
		}
	}

	// Audit: proposed.
	_ = e.audit.Log(audit.Entry{
		EventType: types.AuditActionProposed, SessionID: sid,
		ActionType: string(action.Type), Details: "hash: " + action.Hash, OTR: isOTR,
	})

	// Shield evaluation.
	verdict := e.shield.Evaluate(ctx, action)
	e.emitShieldVerdict(stream, sid, mid, verdict)
	_ = e.audit.Log(audit.Entry{
		EventType: types.AuditActionEvaluated, SessionID: sid,
		ActionType: string(action.Type),
		Details:    fmt.Sprintf("%s (tier %d): %s", verdict.Decision, verdict.Tier, verdict.Reasoning),
	})

	if verdict.Decision == types.VerdictBlock {
		e.log.Log("shield", "%s -> BLOCKED: %s", action.Type, verdict.Reasoning)
		_ = e.audit.Log(audit.Entry{
			EventType: types.AuditActionBlocked, SessionID: sid,
			ActionType: string(action.Type), Details: verdict.Reasoning,
		})
		return &types.ActionResult{
			RequestID: action.RequestID, Success: false,
			Error: "blocked by Shield: " + verdict.Reasoning, Summary: fmt.Sprintf("blocked: %s", action.Type),
		}
	}

	if verdict.Decision == types.VerdictEscalate {
		e.log.Log("shield", "%s -> ESCALATE (approval not yet implemented)", action.Type)
		return &types.ActionResult{
			RequestID: action.RequestID, Success: false,
			Error: "requires human approval (not yet implemented)", Summary: fmt.Sprintf("escalated: %s", action.Type),
		}
	}

	// Hash verification (TOCTOU prevention).
	if err := e.verifier.Verify(action); err != nil {
		e.log.Log("verify", "%s -> hash mismatch", action.Type)
		_ = e.audit.Log(audit.Entry{
			EventType: types.AuditIntegrityViolation, SessionID: sid,
			ActionType: string(action.Type), Details: "hash mismatch",
		})
		return &types.ActionResult{
			RequestID: action.RequestID, Success: false,
			Error: "hash verification failed", Summary: "integrity check failed",
		}
	}
	e.log.Log("verify", "hash match: ok")

	// Chronicle snapshot (Normal mode only, non-blocking on failure).
	if !isOTR {
		if _, snapErr := e.chronicle.Snapshot(action); snapErr != nil {
			e.log.Log("chronicle", "snapshot failed: %s (continuing)", snapErr)
		}
	}

	// Execute.
	e.emitActionStarted(stream, sid, mid, action)
	e.log.Log("executor", "%s starting", action.Type)

	start := time.Now()
	result := e.executors.Execute(ctx, action)
	result.DurationMs = time.Since(start).Milliseconds()

	if result.Success {
		e.log.Log("executor", "%s -> success (%s)", action.Type, result.Summary)
		_ = e.audit.Log(audit.Entry{
			EventType: types.AuditActionExecuted, SessionID: sid,
			ActionType: string(action.Type), Details: result.Summary,
		})
	} else {
		e.log.Log("executor", "%s -> failed: %s", action.Type, result.Error)
		_ = e.audit.Log(audit.Entry{
			EventType: types.AuditActionFailed, SessionID: sid,
			ActionType: string(action.Type), Details: result.Error,
		})
	}

	e.emitActionCompleted(stream, sid, mid, result)
	if result.Artifact != nil {
		_ = stream.Send(&pb.PipelineEvent{
			SessionId: sid, MessageId: mid,
			EventType:      pb.PipelineEventType_ACTION_ARTIFACT,
			ActionArtifact: &pb.ActionArtifact{Artifact: toProtoArtifact(result.Artifact)},
		})
	}

	return result
}

// streamResponse generates and streams the LLM response, with secret redaction.
func (e *Engine) streamResponse(ctx context.Context, stream pb.PipelineService_ProcessMessageServer, sid, mid, content, systemPrompt string, history []llm.ChatMessage, results []*types.ActionResult) error {
	reader, err := e.agent.Responder.Generate(ctx, content, systemPrompt, history, results)
	if err != nil {
		return e.sendError(stream, sid, mid, "LLM_FAILED", err.Error())
	}
	defer func() { _ = reader.Close() }()

	tokenCount := 0
	for {
		token, tokenErr := reader.Next()
		if tokenErr == io.EOF {
			break
		}
		if tokenErr != nil {
			return e.sendError(stream, sid, mid, "STREAM_FAILED", tokenErr.Error())
		}
		tokenCount++
		if sendErr := stream.Send(&pb.PipelineEvent{
			SessionId: sid, MessageId: mid,
			EventType: pb.PipelineEventType_LLM_TOKEN,
			LlmToken:  &pb.LLMToken{Text: token},
		}); sendErr != nil {
			return sendErr
		}
	}

	fullText := e.checker.Redact(reader.FullText())
	e.log.Log("response", "streamed %d tokens", tokenCount)
	e.storeMessage(sid, "", "assistant", fullText)

	return stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType:        pb.PipelineEventType_RESPONSE_COMPLETE,
		ResponseComplete: &pb.ResponseComplete{Content: fullText},
	})
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
func (e *Engine) Shutdown(_ context.Context, _ *pb.ShutdownRequest) (*pb.ShutdownResponse, error) {
	go func() {
		time.Sleep(100 * time.Millisecond)
		e.Stop()
	}()
	return &pb.ShutdownResponse{Clean: true}, nil
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

// --- Event emission helpers ---

func (e *Engine) emitIntentParsed(stream pb.PipelineService_ProcessMessageServer, sid, mid string, intent *types.StructuredIntent) {
	_ = stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType: pb.PipelineEventType_INTENT_PARSED,
		IntentParsed: &pb.IntentParsed{
			Goal: goalToProto(intent.Goal), Confidence: intent.Confidence, Destructive: intent.Destructive,
		},
	})
}

func (e *Engine) emitActionsPlanned(stream pb.PipelineService_ProcessMessageServer, sid, mid string, count int) {
	_ = stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType:      pb.PipelineEventType_ACTIONS_PLANNED,
		ActionsPlanned: &pb.ActionsPlanned{Count: int32(count)},
	})
}

func (e *Engine) emitSelfEval(stream pb.PipelineService_ProcessMessageServer, sid, mid string, passed bool, reason string) {
	_ = stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType:      pb.PipelineEventType_SELF_EVAL_PASSED,
		SelfEvalResult: &pb.SelfEvalResult{Passed: passed, Reason: reason},
	})
}

func (e *Engine) emitShieldVerdict(stream pb.PipelineService_ProcessMessageServer, sid, mid string, verdict *types.Verdict) {
	_ = stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType: pb.PipelineEventType_SHIELD_VERDICT,
		ShieldVerdict: &pb.ShieldVerdict{
			Decision: verdictToProto(verdict.Decision), Tier: int32(verdict.Tier),
			Confidence: verdict.Confidence, Reasoning: verdict.Reasoning,
		},
	})
}

func (e *Engine) emitActionStarted(stream pb.PipelineService_ProcessMessageServer, sid, mid string, action *types.ActionRequest) {
	_ = stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType:     pb.PipelineEventType_ACTION_STARTED,
		ActionStarted: &pb.ActionStarted{Summary: formatActionSummary(action)},
	})
}

func (e *Engine) emitActionCompleted(stream pb.PipelineService_ProcessMessageServer, sid, mid string, result *types.ActionResult) {
	_ = stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType:       pb.PipelineEventType_ACTION_COMPLETED,
		ActionCompleted: &pb.ActionCompleted{Success: result.Success, Summary: result.Summary},
	})
}

// --- Conversion helpers ---

func formatActionSummary(a *types.ActionRequest) string {
	switch a.Type {
	case types.ActionReadFile:
		return fmt.Sprintf("Reading %s", a.Payload["path"])
	case types.ActionWriteFile:
		return fmt.Sprintf("Writing %s", a.Payload["path"])
	case types.ActionDeleteFile:
		return fmt.Sprintf("Deleting %s", a.Payload["path"])
	case types.ActionListDir:
		return fmt.Sprintf("Listing %s", a.Payload["path"])
	case types.ActionExecCommand:
		cmd, _ := a.Payload["command"].(string)
		if len(cmd) > 60 {
			cmd = cmd[:60] + "..."
		}
		return fmt.Sprintf("Running: %s", cmd)
	default:
		return string(a.Type)
	}
}

func goalToProto(g types.GoalType) pb.GoalType {
	m := map[types.GoalType]pb.GoalType{
		types.GoalFileManagement:       pb.GoalType_FILE_MANAGEMENT,
		types.GoalCodeExecution:        pb.GoalType_CODE_EXECUTION,
		types.GoalCommunication:        pb.GoalType_COMMUNICATION,
		types.GoalInformationRetrieval: pb.GoalType_INFORMATION_RETRIEVAL,
		types.GoalScheduling:           pb.GoalType_SCHEDULING,
		types.GoalNoteTaking:           pb.GoalType_NOTE_TAKING,
		types.GoalWebBrowsing:          pb.GoalType_WEB_BROWSING,
		types.GoalGitOperations:        pb.GoalType_GIT_OPERATIONS,
		types.GoalTextProcessing:       pb.GoalType_TEXT_PROCESSING,
		types.GoalSystemManagement:     pb.GoalType_SYSTEM_MANAGEMENT,
		types.GoalCreative:             pb.GoalType_CREATIVE,
		types.GoalConversation:         pb.GoalType_CONVERSATION,
		types.GoalCalendar:             pb.GoalType_CALENDAR,
	}
	if v, ok := m[g]; ok {
		return v
	}
	return pb.GoalType_GOAL_TYPE_UNSPECIFIED
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

// readCanaryToken reads or generates the canary token for Shield.
func readCanaryToken(workspace string) string {
	canaryPath := filepath.Join(workspace, ".openparallax", "canary.token")
	data, err := os.ReadFile(canaryPath)
	if err == nil && len(data) > 0 {
		return string(data)
	}
	token, _ := crypto.GenerateCanary()
	return token
}

// resolveFilePath finds a file by trying config dir, workspace, then cwd.
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
