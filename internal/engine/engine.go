// Package engine implements the privileged execution engine that evaluates
// and executes agent-proposed actions. The engine runs as a separate OS process
// and communicates with the agent via gRPC.
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/openparallax/openparallax/audit"
	"github.com/openparallax/openparallax/chronicle"
	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/ifc"
	"github.com/openparallax/openparallax/internal/agent"
	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/engine/executors"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/oauth"
	"github.com/openparallax/openparallax/internal/session"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	pb "github.com/openparallax/openparallax/internal/types/pb"
	"github.com/openparallax/openparallax/llm"
	"github.com/openparallax/openparallax/mcp"
	"github.com/openparallax/openparallax/memory"
	memsqlite "github.com/openparallax/openparallax/memory/sqlite"
	"github.com/openparallax/openparallax/shield"
	"google.golang.org/grpc"
)

// maxToolRounds limits the number of tool-call round-trips per message
// to prevent infinite loops.
const maxToolRounds = 25

// Engine is the execution engine and gRPC server.
type Engine struct {
	pb.UnimplementedAgentServiceServer
	pb.UnimplementedClientServiceServer
	pb.UnimplementedSubAgentServiceServer

	cfg           *types.AgentConfig
	llm           llm.Provider
	modelRegistry *llm.Registry
	log           *logging.Logger
	agent         *agent.Agent
	executors     *executors.Registry
	shield        *shield.Pipeline
	enricher      *shield.MetadataEnricher
	chronicle     *chronicle.Chronicle
	memory        *memory.Manager
	audit         *audit.Logger
	verifier      *Verifier
	db            *storage.DB
	mcpManager    *mcp.Manager

	tier3Manager    *Tier3Manager
	subAgentManager *SubAgentManager
	oauthManager    *oauth.Manager
	broadcaster     *EventBroadcaster

	server   *grpc.Server
	listener net.Listener

	agentStream   pb.AgentService_RunSessionServer
	currentMsgOTR bool

	sandboxStatus      sandboxInfo
	heartbeatSessionID string

	backgroundCtx    context.Context
	backgroundCancel context.CancelFunc
	backgroundWG     sync.WaitGroup

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

	// Build multi-model registry from config.
	modelReg := llm.NewRegistry()
	for _, m := range cfg.Models {
		if regErr := modelReg.Register(llm.ModelEntry{
			Name: m.Name, Provider: m.Provider,
			Model: m.Model, APIKeyEnv: m.APIKeyEnv, BaseURL: m.BaseURL,
		}); regErr != nil {
			log.Warn("model_register_failed", "name", m.Name, "error", regErr)
		}
	}
	if cfg.Roles.Chat != "" {
		_ = modelReg.SetRole("chat", cfg.Roles.Chat)
	}
	if cfg.Roles.Shield != "" {
		_ = modelReg.SetRole("shield", cfg.Roles.Shield)
	}
	if cfg.Roles.Embedding != "" {
		_ = modelReg.SetRole("embedding", cfg.Roles.Embedding)
	}
	if cfg.Roles.SubAgent != "" {
		_ = modelReg.SetRole("sub_agent", cfg.Roles.SubAgent)
	}
	if cfg.Roles.Image != "" {
		_ = modelReg.SetRole("image", cfg.Roles.Image)
	}
	if cfg.Roles.Video != "" {
		_ = modelReg.SetRole("video", cfg.Roles.Video)
	}

	dbPath := filepath.Join(cfg.Workspace, ".openparallax", "openparallax.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	db.RepairSessions()
	canaryToken := readCanaryToken(cfg.Workspace)

	// Create OAuth manager (nil-safe if no providers configured).
	var oauthMgr *oauth.Manager
	oauthProviders := buildOAuthProviders(cfg)
	if len(oauthProviders) > 0 {
		oauthMgr, _ = oauth.NewManager(db, canaryToken, oauthProviders, log)
	}

	registry := executors.NewRegistry(cfg.Workspace, cfg, oauthMgr, log)
	if len(cfg.Tools.DisabledGroups) > 0 {
		registry.Groups.DisableGroups(cfg.Tools.DisabledGroups)
		log.Info("tools_groups_disabled", "groups", strings.Join(cfg.Tools.DisabledGroups, ", "))
	}

	configDir := filepath.Dir(configPath)
	policyFile := resolveFilePath(cfg.Shield.PolicyFile, configDir, cfg.Workspace)
	promptPath := resolveFilePath("prompts/evaluator-v1.md", configDir, cfg.Workspace)

	// Validate security-critical files exist.
	if _, statErr := os.Stat(policyFile); statErr != nil {
		return nil, fmt.Errorf("shield policy file not found: %s — run 'openparallax init' to create it", policyFile)
	}
	if cfg.Shield.Evaluator.Provider != "" {
		if _, statErr := os.Stat(promptPath); statErr != nil {
			return nil, fmt.Errorf("shield evaluator prompt not found: %s — run 'openparallax init' to create it", promptPath)
		}
	}

	// Warn if skills directory is empty.
	skillsDir := filepath.Join(cfg.Workspace, "skills")
	if entries, readErr := os.ReadDir(skillsDir); readErr != nil || len(entries) == 0 {
		log.Warn("no_skills_found", "message", "no skills found — agent runs without domain-specific guidance")
	}

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

	memStore := memsqlite.NewStore(db)
	mem := memory.NewManager(cfg.Workspace, memStore, provider)
	registry.RegisterMemory(mem)
	ag := agent.NewAgent(provider, cfg.Workspace, mem)

	// MCP manager (optional — only if servers are configured).
	var mcpMgr *mcp.Manager
	if len(cfg.MCP.Servers) > 0 {
		mcpMgr = mcp.NewManager(cfg.MCP.Servers, log)
	}

	eng := &Engine{
		cfg:           cfg,
		llm:           provider,
		modelRegistry: modelReg,
		log:           log,
		agent:         ag,
		executors:     registry,
		shield:        shieldPipeline,
		enricher:      shield.NewMetadataEnricher(),
		chronicle:     chron,
		memory:        mem,
		audit:         auditLogger,
		verifier:      NewVerifier(),
		db:            db,
		mcpManager:    mcpMgr,
		tier3Manager:  NewTier3Manager(cfg.Shield.Tier3.MaxPerHour, cfg.Shield.Tier3.TimeoutSeconds),
		oauthManager:  oauthMgr,
		broadcaster:   NewEventBroadcaster(),
	}
	eng.backgroundCtx, eng.backgroundCancel = context.WithCancel(context.Background())
	return eng, nil
}

// Start begins the gRPC server. If a non-zero port is provided, the server
// binds to that port; otherwise a dynamic port is allocated by the OS.
func (e *Engine) Start(listenPort ...int) (int, error) {
	addr := "localhost:0"
	if len(listenPort) > 0 && listenPort[0] > 0 {
		addr = fmt.Sprintf("localhost:%d", listenPort[0])
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		// Fall back to dynamic port if the requested port is in use.
		if addr != "localhost:0" {
			lis, err = net.Listen("tcp", "localhost:0")
			if err != nil {
				return 0, fmt.Errorf("failed to listen: %w", err)
			}
		} else {
			return 0, fmt.Errorf("failed to listen: %w", err)
		}
	}
	e.listener = lis

	e.server = grpc.NewServer()
	pb.RegisterAgentServiceServer(e.server, e)
	pb.RegisterClientServiceServer(e.server, e)
	pb.RegisterSubAgentServiceServer(e.server, e)

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

// Stop gracefully shuts down the engine. Background tasks (session
// summarization) get a 5-second grace period before forced cancellation.
func (e *Engine) Stop() {
	e.mu.Lock()
	e.shutdown = true
	e.mu.Unlock()

	// Cancel background tasks and wait up to 5 seconds for in-flight work.
	if e.backgroundCancel != nil {
		e.backgroundCancel()
	}
	done := make(chan struct{})
	go func() {
		e.backgroundWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		e.log.Warn("background_tasks_timeout", "message", "in-flight background tasks did not finish within 5s")
	}

	if e.subAgentManager != nil {
		e.subAgentManager.Shutdown()
	}
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
	e.log.Info("agent_connected", "id", ready.AgentId)

	// Store the agent stream for forwarding messages.
	e.mu.Lock()
	e.agentStream = stream
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.agentStream = nil
		e.mu.Unlock()
		e.log.Info("agent_disconnected", "id", ready.AgentId)
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
			isOTR := false // OTR state could be tracked per session
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
		return &pb.ToolResultDelivery{CallId: tp.CallId, Content: "Action requires human approval", IsError: true}
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

func truncateForLog(s string) string {
	if len(s) > 120 {
		return s[:120] + "..."
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
		return llm.ToolResult{CallID: tc.ID, Content: "Action requires human approval — escalation is not available in this session", IsError: true}
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
func (e *Engine) ListSessions(_ context.Context, req *pb.ListSessionsRequest) (*pb.ListSessionsResponse, error) {
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

// auditLog writes an audit entry and logs a warning if the write fails.
func (e *Engine) auditLog(entry audit.Entry) {
	if err := e.audit.Log(entry); err != nil {
		e.log.Warn("audit_write_failed", "event_type", entry.EventType,
			"session", entry.SessionID, "action", entry.ActionType, "error", err)
	}
}

func (e *Engine) sendErrorEvent(sender EventSender, sid, mid, code, message string) error {
	return sender.SendEvent(&PipelineEvent{
		SessionID: sid, MessageID: mid, Type: EventError,
		Error: &PipelineErrorEvent{Code: code, Message: message, Recoverable: true},
	})
}

// ProcessHeartbeatTask processes a scheduled task from HEARTBEAT.md. It uses a
// persistent internal session so the agent has continuity across scheduled runs.
// Events are discarded — heartbeat tasks run silently in the background.
func (e *Engine) ProcessHeartbeatTask(ctx context.Context, task string) {
	e.mu.Lock()
	sid := e.heartbeatSessionID
	if sid == "" {
		sid = crypto.NewID()
		e.heartbeatSessionID = sid
		e.mu.Unlock()
		_ = e.db.InsertSession(&types.Session{
			ID:    sid,
			Mode:  types.SessionHeartbeat,
			Title: "Heartbeat",
		})
	} else {
		e.mu.Unlock()
	}

	mid := "hb-" + crypto.NewID()
	sender := &noopEventSender{}

	taskCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	e.log.Info("heartbeat_process", "session", sid, "task", task)
	if err := e.ProcessMessageForWeb(taskCtx, sender, sid, mid, task, types.SessionNormal); err != nil {
		e.log.Error("heartbeat_failed", "task", task, "error", err)
	}
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

// ModelRegistry returns the multi-model registry.
func (e *Engine) ModelRegistry() *llm.Registry { return e.modelRegistry }

// LLMModel returns the configured LLM model name.
func (e *Engine) LLMModel() string { return e.llm.Model() }

// subAgentMaxRounds returns the configured max LLM calls per sub-agent.
func (e *Engine) subAgentMaxRounds() int {
	if e.cfg.Agents.MaxRounds > 0 {
		return e.cfg.Agents.MaxRounds
	}
	return 20
}

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

// OAuthManager returns the OAuth2 token manager, or nil if not configured.
func (e *Engine) OAuthManager() *oauth.Manager { return e.oauthManager }

// Tier3 returns the Tier 3 human-in-the-loop manager.
func (e *Engine) Tier3() *Tier3Manager { return e.tier3Manager }

// Broadcaster returns the event broadcaster for subscribing clients.
func (e *Engine) Broadcaster() *EventBroadcaster { return e.broadcaster }

// ConfigPath returns the path to the config.yaml file.
func (e *Engine) ConfigPath() string {
	return filepath.Join(e.cfg.Workspace, "config.yaml")
}

// SubAgentManager returns the sub-agent manager.
func (e *Engine) SubAgentManager() *SubAgentManager { return e.subAgentManager }

// SetupSubAgents creates the sub-agent manager and registers the executor.
// Called from internal_engine.go after the gRPC port is known.
func (e *Engine) SetupSubAgents(grpcAddr string) {
	e.subAgentManager = NewSubAgentManager(e, grpcAddr, 5)
	adapter := NewSubAgentManagerAdapter(e.subAgentManager)
	e.executors.RegisterSubAgents(adapter)
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

// noopEventSender discards events. Used for sub-agent tool calls where events
// are managed by the SubAgentManager instead of a client connection.
type noopEventSender struct{}

func (n *noopEventSender) SendEvent(_ *PipelineEvent) error { return nil }

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

// Audit returns the audit logger for recording security events.
func (e *Engine) Audit() *audit.Logger { return e.audit }

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

// buildOAuthProviders creates OAuth provider configs from the agent config.
func buildOAuthProviders(cfg *types.AgentConfig) map[string]oauth.ProviderConfig {
	providers := make(map[string]oauth.ProviderConfig)
	if cfg.OAuth.Google != nil && cfg.OAuth.Google.ClientID != "" {
		providers["google"] = oauth.ProviderConfig{
			ClientID:     cfg.OAuth.Google.ClientID,
			ClientSecret: cfg.OAuth.Google.ClientSecret,
		}
	}
	if cfg.OAuth.Microsoft != nil && cfg.OAuth.Microsoft.ClientID != "" {
		providers["microsoft"] = oauth.ProviderConfig{
			ClientID:     cfg.OAuth.Microsoft.ClientID,
			ClientSecret: cfg.OAuth.Microsoft.ClientSecret,
			TenantID:     cfg.OAuth.Microsoft.TenantID,
		}
	}
	return providers
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
