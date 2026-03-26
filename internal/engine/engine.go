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
	"github.com/openparallax/openparallax/internal/llm"
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
	context   *agent.ContextAssembler
	responder *agent.Responder
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
		context:   agent.NewContextAssembler(cfg.Workspace),
		responder: agent.NewResponder(provider),
		db:        db,
	}, nil
}

// Start begins the gRPC server on a dynamic port. It writes the chosen port
// to the returned channel so the parent process can connect.
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
// In this simplified version (conversation mode), it assembles context from
// memory files, streams the LLM response, and sends it back as pipeline events.
func (e *Engine) ProcessMessage(req *pb.ProcessMessageRequest, stream pb.PipelineService_ProcessMessageServer) error {
	ctx := stream.Context()
	sid := req.SessionId
	mid := req.MessageId

	// Store the user message.
	e.storeMessage(sid, mid, "user", req.Content)

	// Assemble context from workspace memory files.
	systemPrompt, err := e.context.Assemble()
	if err != nil {
		return e.sendError(stream, sid, mid, "CONTEXT_FAILED", err.Error())
	}

	// Build conversation history from this session.
	history := e.getHistory(sid)

	// Stream the LLM response.
	reader, err := e.responder.Generate(ctx, req.Content, systemPrompt, history)
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
			SessionId: sid,
			MessageId: mid,
			EventType: pb.PipelineEventType_LLM_TOKEN,
			LlmToken:  &pb.LLMToken{Text: token},
		}); sendErr != nil {
			return sendErr
		}
	}

	fullText := reader.FullText()

	// Store the assistant response.
	e.storeMessage(sid, "", "assistant", fullText)

	// Send response complete event.
	return stream.Send(&pb.PipelineEvent{
		SessionId:        sid,
		MessageId:        mid,
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
// Excludes the most recent user message (which is the current input).
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
		SessionId: sid,
		MessageId: mid,
		EventType: pb.PipelineEventType_ERROR,
		PipelineError: &pb.PipelineError{
			Code:        code,
			Message:     message,
			Recoverable: true,
		},
	})
}
