// Package engine implements the privileged execution engine that evaluates
// and executes agent-proposed actions. The engine runs as a separate OS process
// and communicates with the agent via gRPC.
package engine

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/openparallax/openparallax/internal/agent"
	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/engine/executors"
	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/parser"
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
	parser    *parser.Parser
	planner   *agent.Planner
	builder   *agent.ActionBuilder
	selfEval  *agent.SelfEvaluator
	context   *agent.ContextAssembler
	responder *agent.Responder
	executors *executors.Registry
	db        *storage.DB

	server   *grpc.Server
	listener net.Listener

	mu       sync.Mutex
	shutdown bool
}

// New creates an Engine from a config file path.
func New(configPath string) (*Engine, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("config load: %w", err)
	}

	provider, err := llm.NewProvider(cfg.LLM)
	if err != nil {
		return nil, fmt.Errorf("llm provider: %w", err)
	}

	dbPath := fmt.Sprintf("%s/.openparallax/openparallax.db", cfg.Workspace)
	db, err := storage.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	return &Engine{
		cfg:       cfg,
		llm:       provider,
		parser:    parser.New(provider),
		planner:   agent.NewPlanner(provider),
		builder:   agent.NewActionBuilder(),
		selfEval:  agent.NewSelfEvaluator(provider),
		context:   agent.NewContextAssembler(cfg.Workspace),
		responder: agent.NewResponder(provider),
		executors: executors.NewRegistry(cfg.Workspace),
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
// It runs the full pipeline: parse → plan → self-eval → execute → respond.
// For pure conversation (no actions), it skips planning and execution.
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
		// Parse failure: fall back to conversation mode.
		return e.streamConversation(ctx, stream, sid, mid, req.Content, systemPrompt, history)
	}

	_ = stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType: pb.PipelineEventType_INTENT_PARSED,
		IntentParsed: &pb.IntentParsed{
			Goal:        goalToProto(intent.Goal),
			Confidence:  intent.Confidence,
			Destructive: intent.Destructive,
		},
	})

	// Pure conversation: skip planning and execution.
	if intent.Goal == types.GoalConversation || intent.PrimaryAction == "conversation" {
		return e.streamConversation(ctx, stream, sid, mid, req.Content, systemPrompt, history)
	}

	// Step 2: Plan actions.
	rawPlan, err := e.planner.Plan(ctx, intent, systemPrompt, history)
	if err != nil {
		return e.streamConversation(ctx, stream, sid, mid, req.Content, systemPrompt, history)
	}

	actions, err := e.builder.Build(rawPlan)
	if err != nil || len(actions) == 0 {
		return e.streamConversation(ctx, stream, sid, mid, req.Content, systemPrompt, history)
	}

	_ = stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType:      pb.PipelineEventType_ACTIONS_PLANNED,
		ActionsPlanned: &pb.ActionsPlanned{Count: int32(len(actions))},
	})

	// Step 3: Self-eval (Layer 0) — log result but don't block in this chunk.
	passed, reason, _ := e.selfEval.Evaluate(ctx, actions, req.Content)
	_ = stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType:      pb.PipelineEventType_SELF_EVAL_PASSED,
		SelfEvalResult: &pb.SelfEvalResult{Passed: passed, Reason: reason},
	})

	// Step 4: Execute each action.
	var results []*types.ActionResult
	for _, action := range actions {
		_ = stream.Send(&pb.PipelineEvent{
			SessionId: sid, MessageId: mid,
			EventType:     pb.PipelineEventType_ACTION_STARTED,
			ActionStarted: &pb.ActionStarted{Summary: formatActionSummary(action)},
		})

		start := time.Now()
		result := e.executors.Execute(ctx, action)
		result.DurationMs = time.Since(start).Milliseconds()
		results = append(results, result)

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
	return e.streamResponse(ctx, stream, sid, mid, req.Content, systemPrompt, history, results)
}

// streamConversation generates a streaming response without any action execution.
func (e *Engine) streamConversation(ctx context.Context, stream pb.PipelineService_ProcessMessageServer, sid, mid, content, systemPrompt string, history []llm.ChatMessage) error {
	return e.streamResponse(ctx, stream, sid, mid, content, systemPrompt, history, nil)
}

// streamResponse generates and streams the LLM response, incorporating action results if any.
func (e *Engine) streamResponse(ctx context.Context, stream pb.PipelineService_ProcessMessageServer, sid, mid, content, systemPrompt string, history []llm.ChatMessage, results []*types.ActionResult) error {
	reader, err := e.responder.Generate(ctx, content, systemPrompt, history, results)
	if err != nil {
		return e.sendError(stream, sid, mid, "LLM_FAILED", err.Error())
	}
	defer func() { _ = reader.Close() }()

	for {
		token, tokenErr := reader.Next()
		if tokenErr == io.EOF {
			break
		}
		if tokenErr != nil {
			return e.sendError(stream, sid, mid, "STREAM_FAILED", tokenErr.Error())
		}
		if sendErr := stream.Send(&pb.PipelineEvent{
			SessionId: sid, MessageId: mid,
			EventType: pb.PipelineEventType_LLM_TOKEN,
			LlmToken:  &pb.LLMToken{Text: token},
		}); sendErr != nil {
			return sendErr
		}
	}

	fullText := reader.FullText()
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

// storeMessage saves a message to the database.
func (e *Engine) storeMessage(sessionID, messageID, role, content string) {
	if messageID == "" {
		messageID = fmt.Sprintf("msg-%d", time.Now().UnixNano())
	}
	_ = e.db.InsertMessage(&types.Message{
		ID:        messageID,
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
}

// getHistory retrieves conversation history for a session as LLM messages.
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

// sendError sends an ERROR pipeline event.
func (e *Engine) sendError(stream pb.PipelineService_ProcessMessageServer, sid, mid, code, message string) error {
	return stream.Send(&pb.PipelineEvent{
		SessionId: sid, MessageId: mid,
		EventType: pb.PipelineEventType_ERROR,
		PipelineError: &pb.PipelineError{
			Code: code, Message: message, Recoverable: true,
		},
	})
}

// formatActionSummary creates a human-readable description of an action.
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

// goalToProto converts a GoalType to the protobuf enum value.
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

// toProtoArtifact converts a native Artifact to the protobuf type.
func toProtoArtifact(a *types.Artifact) *pb.Artifact {
	return &pb.Artifact{
		Id: a.ID, Type: a.Type, Title: a.Title, Path: a.Path,
		Content: a.Content, Language: a.Language,
		SizeBytes: a.SizeBytes, PreviewType: a.PreviewType,
	}
}
