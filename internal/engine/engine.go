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
	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/engine/executors"
	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/parser"
	"github.com/openparallax/openparallax/internal/plog"
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
	planner   *agent.Planner
	builder   *agent.ActionBuilder
	selfEval  *agent.SelfEvaluator
	context   *agent.ContextAssembler
	responder *agent.Responder
	executors *executors.Registry
	shield    *shield.Pipeline
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

	// Read canary token.
	canaryToken := ""
	canaryPath := filepath.Join(cfg.Workspace, ".openparallax", "canary.token")
	if data, readErr := readFileQuiet(canaryPath); readErr == nil {
		canaryToken = string(data)
	}
	if canaryToken == "" {
		var genErr error
		canaryToken, genErr = crypto.GenerateCanary()
		if genErr != nil {
			return nil, fmt.Errorf("canary generation: %w", genErr)
		}
	}

	// Resolve policy and prompt paths relative to the config file's directory.
	configDir := filepath.Dir(configPath)
	policyFile := cfg.Shield.PolicyFile
	if policyFile != "" && !filepath.IsAbs(policyFile) {
		policyFile = filepath.Join(configDir, policyFile)
	}
	promptPath := filepath.Join(configDir, "prompts", "evaluator-v1.md")

	// Initialize Shield pipeline.
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

	// Initialize audit logger with SQLite indexing.
	auditPath := filepath.Join(cfg.Workspace, ".openparallax", "audit.jsonl")
	auditLogger, err := audit.NewLogger(auditPath)
	if err != nil {
		return nil, fmt.Errorf("audit logger: %w", err)
	}
	auditLogger.SetDB(db)

	return &Engine{
		cfg:       cfg,
		llm:       provider,
		log:       log,
		parser:    parser.New(provider),
		planner:   agent.NewPlanner(provider, registry.AvailableActions()),
		builder:   agent.NewActionBuilder(),
		selfEval:  agent.NewSelfEvaluator(provider),
		context:   agent.NewContextAssembler(cfg.Workspace),
		responder: agent.NewResponder(provider),
		executors: registry,
		shield:    shieldPipeline,
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
// Full secured pipeline: parse -> plan -> self-eval -> shield -> verify -> execute -> respond.
func (e *Engine) ProcessMessage(req *pb.ProcessMessageRequest, stream pb.PipelineService_ProcessMessageServer) error {
	ctx := stream.Context()
	sid := req.SessionId
	mid := req.MessageId

	e.storeMessage(sid, mid, "user", req.Content)

	systemPrompt, err := e.context.Assemble()
	if err != nil {
		return e.sendError(stream, sid, mid, "CONTEXT_FAILED", err.Error())
	}

	history := e.getHistory(sid)

	// Step 1: Parse intent.
	intent, err := e.parser.Parse(ctx, req.Content)
	if err != nil {
		e.log.Log("parser", "failed: %s, falling back to conversation", err)
		return e.streamConversation(ctx, stream, sid, mid, req.Content, systemPrompt, history)
	}

	e.log.Log("parser", "intent: %s / %s (confidence: %.2f, destructive: %v)",
		intent.Goal, intent.PrimaryAction, intent.Confidence, intent.Destructive)

	_ = stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType: pb.PipelineEventType_INTENT_PARSED,
		IntentParsed: &pb.IntentParsed{
			Goal:        goalToProto(intent.Goal),
			Confidence:  intent.Confidence,
			Destructive: intent.Destructive,
		},
	})

	if intent.Goal == types.GoalConversation || intent.PrimaryAction == "conversation" {
		e.log.Log("parser", "conversation mode, skipping action pipeline")
		return e.streamConversation(ctx, stream, sid, mid, req.Content, systemPrompt, history)
	}

	// Step 2: Plan actions.
	rawPlan, err := e.planner.Plan(ctx, intent, systemPrompt, history)
	if err != nil {
		e.log.Log("planner", "failed: %s, falling back to conversation", err)
		return e.streamConversation(ctx, stream, sid, mid, req.Content, systemPrompt, history)
	}

	actions, err := e.builder.Build(rawPlan)
	if err != nil || len(actions) == 0 {
		e.log.Log("builder", "no actions produced, falling back to conversation")
		return e.streamConversation(ctx, stream, sid, mid, req.Content, systemPrompt, history)
	}

	e.log.Log("planner", "raw plan: %d ACTION block(s) parsed", len(actions))

	_ = stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType:      pb.PipelineEventType_ACTIONS_PLANNED,
		ActionsPlanned: &pb.ActionsPlanned{Count: int32(len(actions))},
	})

	// Step 3: Self-eval (Layer 0) — enforcing gate.
	passed, reason, _ := e.selfEval.Evaluate(ctx, actions, req.Content)
	if passed {
		e.log.Log("selfeval", "passed")
	} else {
		e.log.Log("selfeval", "FAILED: %s", reason)
	}
	_ = stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType:      pb.PipelineEventType_SELF_EVAL_PASSED,
		SelfEvalResult: &pb.SelfEvalResult{Passed: passed, Reason: reason},
	})
	if !passed {
		_ = e.audit.Log(audit.Entry{
			EventType: types.AuditSelfProtection, SessionID: sid,
			Details: fmt.Sprintf("self-eval blocked: %s", reason),
		})
		return e.sendError(stream, sid, mid, "SELF_EVAL_FAILED",
			fmt.Sprintf("Safety check failed: %s", reason))
	}

	// Step 4: Evaluate and execute each action.
	var results []*types.ActionResult
	for _, action := range actions {
		// Audit: action proposed.
		_ = e.audit.Log(audit.Entry{
			EventType: types.AuditActionProposed, SessionID: sid,
			ActionType: string(action.Type),
			Details:    fmt.Sprintf("hash: %s", action.Hash),
		})

		// Shield evaluation.
		verdict := e.shield.Evaluate(ctx, action)

		_ = stream.Send(&pb.PipelineEvent{
			SessionId: sid, MessageId: mid,
			EventType: pb.PipelineEventType_SHIELD_VERDICT,
			ShieldVerdict: &pb.ShieldVerdict{
				Decision:   verdictToProto(verdict.Decision),
				Tier:       int32(verdict.Tier),
				Confidence: verdict.Confidence,
				Reasoning:  verdict.Reasoning,
			},
		})

		_ = e.audit.Log(audit.Entry{
			EventType: types.AuditActionEvaluated, SessionID: sid,
			ActionType: string(action.Type),
			Details:    fmt.Sprintf("%s (tier %d): %s", verdict.Decision, verdict.Tier, verdict.Reasoning),
		})

		if verdict.Decision == types.VerdictBlock {
			e.log.Log("shield", "%s -> BLOCKED: %s", action.Type, verdict.Reasoning)
			_ = e.audit.Log(audit.Entry{
				EventType: types.AuditActionBlocked, SessionID: sid,
				ActionType: string(action.Type),
				Details:    verdict.Reasoning,
			})
			results = append(results, &types.ActionResult{
				RequestID: action.RequestID, Success: false,
				Error:   fmt.Sprintf("blocked by Shield: %s", verdict.Reasoning),
				Summary: fmt.Sprintf("blocked: %s", action.Type),
			})
			continue
		}

		if verdict.Decision == types.VerdictEscalate {
			e.log.Log("shield", "%s -> ESCALATE (approval not yet implemented)", action.Type)
			results = append(results, &types.ActionResult{
				RequestID: action.RequestID, Success: false,
				Error:   "requires human approval (not yet implemented)",
				Summary: fmt.Sprintf("escalated: %s", action.Type),
			})
			continue
		}

		// Hash verification.
		if verifyErr := e.verifier.Verify(action); verifyErr != nil {
			e.log.Log("verify", "%s -> hash mismatch!", action.Type)
			_ = e.audit.Log(audit.Entry{
				EventType: types.AuditIntegrityViolation, SessionID: sid,
				ActionType: string(action.Type),
				Details:    "hash mismatch between evaluation and execution",
			})
			results = append(results, &types.ActionResult{
				RequestID: action.RequestID, Success: false,
				Error: "hash verification failed", Summary: "integrity check failed",
			})
			continue
		}
		e.log.Log("verify", "hash match: ok")

		// Execute.
		_ = stream.Send(&pb.PipelineEvent{
			SessionId: sid, MessageId: mid,
			EventType:     pb.PipelineEventType_ACTION_STARTED,
			ActionStarted: &pb.ActionStarted{Summary: formatActionSummary(action)},
		})

		e.log.Log("executor", "%s starting", action.Type)
		start := time.Now()
		result := e.executors.Execute(ctx, action)
		result.DurationMs = time.Since(start).Milliseconds()
		results = append(results, result)

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
	}

	// Step 5: Generate streaming response with action results.
	e.log.Log("response", "streaming with %d action result(s)", len(results))
	return e.streamResponse(ctx, stream, sid, mid, req.Content, systemPrompt, history, results)
}

// streamConversation generates a streaming response without any action execution.
func (e *Engine) streamConversation(ctx context.Context, stream pb.PipelineService_ProcessMessageServer, sid, mid, content, systemPrompt string, history []llm.ChatMessage) error {
	return e.streamResponse(ctx, stream, sid, mid, content, systemPrompt, history, nil)
}

// streamResponse generates and streams the LLM response, with secret redaction.
func (e *Engine) streamResponse(ctx context.Context, stream pb.PipelineService_ProcessMessageServer, sid, mid, content, systemPrompt string, history []llm.ChatMessage, results []*types.ActionResult) error {
	reader, err := e.responder.Generate(ctx, content, systemPrompt, history, results)
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
	e.log.Log("response", "streaming %d tokens", tokenCount)
	e.storeMessage(sid, "", "assistant", fullText)

	return stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType:        pb.PipelineEventType_RESPONSE_COMPLETE,
		ResponseComplete: &pb.ResponseComplete{Content: fullText},
	})
}

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

func readFileQuiet(path string) ([]byte, error) {
	return os.ReadFile(path)
}
