package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/openparallax/openparallax/internal/types"
	pb "github.com/openparallax/openparallax/internal/types/pb"
	"github.com/openparallax/openparallax/llm"
	"github.com/openparallax/openparallax/memory"
)

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
// Runs on the engine's background context so it survives session switches.
func (e *Engine) summarizeActiveSessions(_ context.Context) {
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
		// Copy history so the goroutine has its own slice.
		histCopy := make([]llm.ChatMessage, len(history))
		copy(histCopy, history)
		sid := sess.ID

		e.backgroundWG.Add(1)
		go func() {
			defer e.backgroundWG.Done()
			sumCtx, cancel := context.WithTimeout(e.backgroundCtx, 30*time.Second)
			defer cancel()
			if sumErr := e.memory.SummarizeSession(sumCtx, "", histCopy); sumErr != nil {
				e.log.Warn("session_summarize_failed", "session", sid, "error", sumErr)
			} else {
				e.log.Info("session_summarized", "session", sid)
			}
		}()
	}
}

// ReadMemory implements the PipelineService gRPC method.
func (e *Engine) ReadMemory(_ context.Context, req *pb.MemoryReadRequest) (*pb.MemoryReadResponse, error) {
	content, err := e.memory.Read(memory.FileType(req.FileType))
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

// ListSessions implements ClientService.ListSessions.
func (e *Engine) ListSessions(_ context.Context, _ *pb.ListSessionsRequest) (*pb.ListSessionsResponse, error) {
	sessions, err := e.db.ListSessions()
	if err != nil {
		return nil, err
	}
	var infos []*pb.SessionInfo
	for _, s := range sessions {
		var createdAt int64
		if t, err := time.Parse(time.RFC3339, s.CreatedAt); err == nil {
			createdAt = t.Unix()
		}
		infos = append(infos, &pb.SessionInfo{
			Id:        s.ID,
			Title:     s.Title,
			CreatedAt: createdAt,
		})
	}
	return &pb.ListSessionsResponse{Sessions: infos}, nil
}

// GetHistory implements ClientService.GetHistory.
func (e *Engine) GetHistory(_ context.Context, req *pb.GetHistoryRequest) (*pb.GetHistoryResponse, error) {
	messages, err := e.db.GetMessages(req.SessionId)
	if err != nil {
		return nil, err
	}
	var pbMsgs []*pb.ChatMessage
	for _, m := range messages {
		pbMsgs = append(pbMsgs, &pb.ChatMessage{
			Id:        m.ID,
			SessionId: m.SessionID,
			Role:      m.Role,
			Content:   m.Content,
			Timestamp: m.Timestamp.Unix(),
		})
	}
	return &pb.GetHistoryResponse{Messages: pbMsgs}, nil
}

// ResolveApproval implements ClientService.ResolveApproval.
func (e *Engine) ResolveApproval(_ context.Context, req *pb.ApprovalResponse) (*pb.ApprovalAck, error) {
	if e.tier3Manager == nil {
		return &pb.ApprovalAck{Received: false}, nil
	}
	if err := e.tier3Manager.Decide(req.ApprovalId, req.Approved); err != nil {
		return nil, err
	}
	return &pb.ApprovalAck{Received: true}, nil
}

// RegisterSubAgent implements the PipelineService gRPC method.
func (e *Engine) RegisterSubAgent(_ context.Context, req *pb.SubAgentRegisterRequest) (*pb.SubAgentRegisterResponse, error) {
	if e.subAgentManager == nil {
		return nil, fmt.Errorf("sub-agent manager not initialized")
	}
	sa, err := e.subAgentManager.RegisterSubAgent(req.Token)
	if err != nil {
		return nil, err
	}

	// Serialize tool definitions.
	var toolDefs []*pb.SubAgentToolDef
	for _, t := range sa.tools {
		paramsJSON, _ := json.Marshal(t.Parameters)
		toolDefs = append(toolDefs, &pb.SubAgentToolDef{
			Name:           t.Name,
			Description:    t.Description,
			ParametersJson: string(paramsJSON),
		})
	}

	return &pb.SubAgentRegisterResponse{
		Name:         sa.Name,
		Task:         sa.Task,
		Tools:        toolDefs,
		SystemPrompt: SubAgentSystemPrompt(sa.Task),
		Model:        sa.Model,
		Provider:     sa.provider,
		ApiKeyEnv:    sa.apiKeyEnv,
		BaseUrl:      sa.baseURL,
		MaxLlmCalls:  int32(e.subAgentMaxRounds()),
	}, nil
}

// SubAgentExecuteTool implements the PipelineService gRPC method.
func (e *Engine) SubAgentExecuteTool(ctx context.Context, req *pb.SubAgentToolRequest) (*pb.SubAgentToolResponse, error) {
	if e.subAgentManager == nil {
		return nil, fmt.Errorf("sub-agent manager not initialized")
	}

	// Parse arguments.
	var args map[string]any
	if req.ArgumentsJson != "" {
		if err := json.Unmarshal([]byte(req.ArgumentsJson), &args); err != nil {
			return &pb.SubAgentToolResponse{Content: "invalid arguments JSON: " + err.Error(), IsError: true}, nil
		}
	}

	tc := &llm.ToolCall{
		ID:        req.CallId,
		Name:      req.ToolName,
		Arguments: args,
	}

	// Determine session mode.
	mode := types.SessionNormal
	sid := req.SessionId
	if sid == "" {
		sid = "sub-agent-" + req.Name
	}

	// Create a no-op sender for sub-agent tool calls (events go through the manager).
	sender := &noopEventSender{}

	result := e.processToolCall(ctx, tc, mode, sid, "sub-"+req.Name, sender)
	e.subAgentManager.IncrementToolCall(req.Name)

	return &pb.SubAgentToolResponse{
		Content: result.Content,
		IsError: result.IsError,
	}, nil
}

// SubAgentComplete implements the PipelineService gRPC method.
func (e *Engine) SubAgentComplete(_ context.Context, req *pb.SubAgentCompleteRequest) (*pb.SubAgentCompleteResponse, error) {
	if e.subAgentManager == nil {
		return nil, fmt.Errorf("sub-agent manager not initialized")
	}
	e.subAgentManager.CompleteSubAgent(req.Name, req.Result)
	e.log.Info("sub_agent_completed", "name", req.Name, "result_len", len(req.Result))
	return &pb.SubAgentCompleteResponse{}, nil
}

// SubAgentFailed implements the PipelineService gRPC method.
func (e *Engine) SubAgentFailed(_ context.Context, req *pb.SubAgentFailedRequest) (*pb.SubAgentFailedResponse, error) {
	if e.subAgentManager == nil {
		return nil, fmt.Errorf("sub-agent manager not initialized")
	}
	e.subAgentManager.FailSubAgent(req.Name, req.Error)
	e.log.Warn("sub_agent_failed", "name", req.Name, "error", req.Error)
	return &pb.SubAgentFailedResponse{}, nil
}

// SubAgentPollMessage implements the SubAgentService gRPC method.
func (e *Engine) SubAgentPollMessage(_ context.Context, req *pb.SubAgentPollRequest) (*pb.SubAgentPollResponse, error) {
	if e.subAgentManager == nil {
		return nil, fmt.Errorf("sub-agent manager not initialized")
	}
	content, hasMessage := e.subAgentManager.PollMessage(req.Name)
	return &pb.SubAgentPollResponse{HasMessage: hasMessage, Content: content}, nil
}

// noopEventSender discards events. Used for sub-agent tool calls where events
// are managed by the SubAgentManager instead of a client connection.
type noopEventSender struct{}

func (n *noopEventSender) SendEvent(_ *PipelineEvent) error { return nil }
