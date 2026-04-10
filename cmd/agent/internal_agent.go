package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/openparallax/openparallax/internal/agent"
	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	pb "github.com/openparallax/openparallax/internal/types/pb"
	"github.com/openparallax/openparallax/llm"
	"github.com/openparallax/openparallax/sandbox"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// agentLog writes a structured JSON log line to stderr. The engine captures
// agent stderr and appends it to engine.log, so the format matches the
// engine's own structured log entries.
func agentLog(level, event string, kvs ...any) {
	data := make(map[string]any, len(kvs)/2)
	for i := 0; i+1 < len(kvs); i += 2 {
		data[fmt.Sprintf("%v", kvs[i])] = kvs[i+1]
	}
	entry := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339Nano),
		"level":     level,
		"event":     event,
		"data":      data,
	}
	b, _ := json.Marshal(entry)
	fmt.Fprintln(os.Stderr, string(b))
}

var (
	agentGRPCAddr  string
	agentName      string
	agentWorkspace string
)

var internalAgentCmd = &cobra.Command{
	Use:          "internal-agent",
	Short:        "Run the sandboxed agent process (internal use only)",
	Hidden:       true,
	SilenceUsage: true,
	RunE:         runInternalAgent,
}

func init() {
	internalAgentCmd.Flags().StringVar(&agentGRPCAddr, "grpc", "", "engine gRPC address")
	internalAgentCmd.Flags().StringVar(&agentName, "name", "Atlas", "agent display name")
	internalAgentCmd.Flags().StringVar(&agentWorkspace, "workspace", "", "workspace path")
	rootCmd.AddCommand(internalAgentCmd)
}

// runInternalAgent starts the headless sandboxed agent process. It applies
// kernel-level sandboxing, connects to the Engine via gRPC, and runs the
// LLM reasoning loop. No TUI — this process is purely for LLM interaction.
func runInternalAgent(_ *cobra.Command, _ []string) error {
	if agentGRPCAddr == "" {
		return fmt.Errorf("--grpc flag is required")
	}
	if agentWorkspace == "" {
		return fmt.Errorf("--workspace flag is required")
	}

	// Load config to get LLM settings.
	cfgPath := filepath.Join(agentWorkspace, "config.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Apply sandbox FIRST, before any untrusted operations.
	sb := sandbox.New()
	if sb.Available() {
		chatModel, _ := cfg.ChatModel()
		llmHost := llm.APIHost(chatModel.LLMConfig())
		sbErr := sb.ApplySelf(sandbox.Config{
			AllowedReadPaths:  []string{agentWorkspace},
			AllowedWritePaths: []string{},
			AllowedTCPConnect: []string{agentGRPCAddr, llmHost},
			AllowProcessSpawn: false,
		})
		if sbErr != nil {
			agentLog("error", "sandbox_apply_failed", "error", sbErr.Error())
		}
	}

	// Canary probes: verify sandbox enforcement per platform.
	// Write the result via stderr (JSON log) since the sandbox blocks file writes.
	// The engine reads it from the agent's log output.
	canary := sandbox.VerifyCanary()
	canaryJSON, _ := json.Marshal(canary)
	agentLog("info", "sandbox_canary_result", "result", string(canaryJSON))
	agentLog("info", "sandbox_canary", "status", canary.Status, "summary", canary.Summary)

	if canary.Status == "unavailable" {
		agentLog("warn", "sandbox_unavailable", "detail", "no sandbox available on this platform, running unprotected")
	} else {
		// Check required probes — must all pass or agent refuses to start.
		if reqFailed := canary.RequiredFailed(); len(reqFailed) > 0 {
			return fmt.Errorf("sandbox verification failed: required probes failed: %v", reqFailed)
		}
		// Advisory probes — warn but continue.
		if advFailed := canary.AdvisoryFailed(); len(advFailed) > 0 {
			agentLog("warn", "sandbox_advisory_failed", "probes", advFailed)
		}
	}

	// Create LLM provider.
	chatCfg, _ := cfg.ChatModel()
	agentLog("info", "llm_provider_init", "provider", chatCfg.Provider, "model", chatCfg.Model)
	provider, err := llm.NewProvider(chatCfg.LLMConfig())
	if err != nil {
		return fmt.Errorf("llm provider: %w", err)
	}

	// Open read-only DB for history access.
	dbPath := filepath.Join(agentWorkspace, ".openparallax", "openparallax.db")
	db, _ := storage.Open(dbPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		cancel()
	}()

	// Connect to Engine gRPC.
	agentLog("info", "grpc_connect", "address", agentGRPCAddr)
	conn, err := grpc.NewClient(agentGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("connect to engine: %w", err)
	}
	defer func() { _ = conn.Close() }()

	client := pb.NewAgentServiceClient(conn)

	// Open bidirectional RunSession stream.
	stream, err := client.RunSession(ctx)
	if err != nil {
		return fmt.Errorf("open session stream: %w", err)
	}

	// Create agent (context assembly, compaction, skills). The memory
	// flusher callback ships extracted facts over the gRPC stream to the
	// engine for persistence — the agent itself is sandboxed and cannot
	// write to the workspace.
	memoryFlusher := agent.MemoryFlusher(func(content string) {
		_ = stream.Send(&pb.AgentEvent{
			Event: &pb.AgentEvent_MemoryFlush{
				MemoryFlush: &pb.MemoryFlush{Content: content},
			},
		})
	})
	ag := agent.NewAgent(provider, agentWorkspace, nil, memoryFlusher, nil)
	ag.Context.OutputSanitization = cfg.General.OutputSanitization

	// Signal readiness with auth token and the canary verification
	// result so the engine can audit-log it (the agent itself cannot
	// write to audit.jsonl — it lives under the hard-blocked
	// .openparallax/ directory).
	agentID := agentName
	if token := os.Getenv("OPENPARALLAX_AGENT_TOKEN"); token != "" {
		agentID = agentName + ":" + token
	}
	if sendErr := stream.Send(&pb.AgentEvent{
		Event: &pb.AgentEvent_Ready{Ready: &pb.AgentReady{
			AgentId:           agentID,
			SandboxCanaryJson: string(canaryJSON),
		}},
	}); sendErr != nil {
		return fmt.Errorf("send ready: %w", sendErr)
	}

	// Wait for the engine to push the initial tool set. The agent
	// holds nothing tool-related of its own — the engine is the
	// authoritative source for the load_tools menu (it knows which
	// conditional groups are registered for this workspace, which
	// MCP groups are mounted, and which groups the user disabled in
	// config). The agent must receive InitialToolDefs before it can
	// process any user message; if the directive does not arrive
	// within the deadline below, the agent fails the session
	// fail-closed rather than starting with a stale or hardcoded
	// tool list.
	const initialToolDeadline = 10 * time.Second
	initialDirective, err := recvWithDeadline(stream, initialToolDeadline)
	if err != nil {
		return fmt.Errorf("wait for initial tool defs: %w", err)
	}
	initialPayload := initialDirective.GetInitialToolDefs()
	if initialPayload == nil {
		return fmt.Errorf("expected InitialToolDefs as first directive after AgentReady, got %T", initialDirective.Directive)
	}
	initialTools := agent.ToolDefsToLLM(initialPayload.Tools)
	if len(initialTools) == 0 {
		return fmt.Errorf("engine sent empty InitialToolDefs — agent has no entry-point tool to call")
	}
	agentLog("info", "initial_tool_defs_received", "tool_count", len(initialTools))

	maxRounds := cfg.Agents.MaxToolRounds
	if maxRounds <= 0 {
		maxRounds = 25
	}
	contextWindow := cfg.Agents.ContextWindow
	if contextWindow <= 0 {
		contextWindow = 128000
	}
	loopCfg := agent.LoopConfig{
		Provider:            provider,
		Agent:               ag,
		MaxRounds:           maxRounds,
		ContextWindow:       contextWindow,
		CompactionThreshold: cfg.Agents.CompactionThreshold,
		MaxResponseTokens:   cfg.Agents.MaxResponseTokens,
	}

	// directiveCh carries tool results from the stream reader to the active
	// processMessage call. Buffered so the reader doesn't block.
	directiveCh := make(chan *pb.EngineDirective, 4)

	// Stream reader goroutine: reads all directives and routes them.
	// ProcessRequest and Shutdown are handled in the main goroutine;
	// ToolResult/ToolDefs are forwarded to the active processMessage.
	//
	// The reader owns directiveCh and closes it on exit. Without the close,
	// an in-flight processMessage waiting on a tool result would block
	// forever after the engine drops the stream — its inner pump goroutine
	// only unwinds when directiveCh closes or its own context cancels.
	processCh := make(chan *pb.ProcessRequest, 1)
	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)
		defer close(directiveCh)
		for {
			directive, recvErr := stream.Recv()
			if recvErr != nil {
				agentLog("info", "stream_recv_exit", "error", recvErr.Error())
				return
			}
			switch d := directive.Directive.(type) {
			case *pb.EngineDirective_Process:
				processCh <- d.Process
			case *pb.EngineDirective_Shutdown:
				return
			default:
				// ToolResult, ToolDefs — forward to active processMessage.
				select {
				case directiveCh <- directive:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	// Main directive loop. processMessage is launched in its own goroutine
	// so the loop can observe doneCh (stream death) mid-message and unwind
	// cleanly. Without this wrap, a synchronous processMessage call would
	// block the select indefinitely if the engine dropped the stream during
	// a tool round-trip — the agent process would stay alive but wedged.
	for {
		select {
		case req, ok := <-processCh:
			if !ok {
				return nil
			}
			msgDone := make(chan struct{})
			go func() {
				defer close(msgDone)
				processMessage(ctx, stream, directiveCh, loopCfg, initialTools, req, db)
			}()
			select {
			case <-msgDone:
			case <-doneCh:
				return nil
			case <-ctx.Done():
				return nil
			}

		case <-doneCh:
			return nil

		case <-ctx.Done():
			return nil
		}
	}
}

// recvWithDeadline reads one directive from the agent stream with a
// hard timeout. Used during the AgentReady → InitialToolDefs handshake
// so the agent fails the session fail-closed if the engine never sends
// the initial tool set, rather than blocking forever or starting with
// a stale list.
func recvWithDeadline(stream pb.AgentService_RunSessionClient, deadline time.Duration) (*pb.EngineDirective, error) {
	type recvResult struct {
		directive *pb.EngineDirective
		err       error
	}
	resultCh := make(chan recvResult, 1)
	go func() {
		d, err := stream.Recv()
		resultCh <- recvResult{directive: d, err: err}
	}()
	select {
	case r := <-resultCh:
		return r.directive, r.err
	case <-time.After(deadline):
		return nil, fmt.Errorf("timeout waiting for engine directive after %s", deadline)
	}
}

// processMessage runs the LLM reasoning loop for a single message.
// directiveCh receives ToolResult/ToolDefs directives from the outer stream
// reader goroutine; processMessage does not read from the gRPC stream directly.
//
// initialTools is the engine-pushed startup tool set delivered via
// InitialToolDefs at session start. The agent never constructs tool
// definitions itself; this slice is the only source for the LLM's
// starting tool list. New tools loaded mid-session via load_tools
// arrive over directiveCh.
func processMessage(
	ctx context.Context,
	stream pb.AgentService_RunSessionClient,
	directiveCh <-chan *pb.EngineDirective,
	cfg agent.LoopConfig,
	initialTools []llm.ToolDefinition,
	req *pb.ProcessRequest,
	db *storage.DB,
) {
	sid := req.SessionId
	mid := req.MessageId
	mode := types.SessionNormal
	if req.Mode == pb.SessionMode_OTR {
		mode = types.SessionOTR
	}

	agentLog("info", "process_message", "session", sid, "message", mid, "content_len", len(req.Content), "mode", mode)

	// Load history from DB (read-only).
	var history []llm.ChatMessage
	if db != nil {
		if msgs, err := db.GetMessages(sid); err == nil {
			for _, m := range msgs {
				history = append(history, llm.ChatMessage{Role: m.Role, Content: m.Content})
			}
		}
	}
	agentLog("info", "history_loaded", "session", sid, "count", len(history))

	// Result channel: converts directives from the outer reader into ToolResults
	// that the RunLoop expects.
	resultCh := make(chan agent.ToolResult, 1)

	toolCtx, toolCancel := context.WithCancel(ctx)
	defer toolCancel()

	go func() {
		defer close(resultCh)
		for {
			select {
			case directive, ok := <-directiveCh:
				if !ok {
					return
				}
				switch d := directive.Directive.(type) {
				case *pb.EngineDirective_ToolResult:
					agentLog("info", "tool_result_received", "call_id", d.ToolResult.CallId, "is_error", d.ToolResult.IsError, "content_len", len(d.ToolResult.Content))
					select {
					case resultCh <- agent.ToolResult{
						CallID:  d.ToolResult.CallId,
						Content: d.ToolResult.Content,
						IsError: d.ToolResult.IsError,
					}:
					case <-toolCtx.Done():
						return
					}
				case *pb.EngineDirective_ToolDefs:
					summary := formatToolDefsSummary(d.ToolDefs.Tools)
					newTools := agent.ToolDefsToLLM(d.ToolDefs.Tools)
					select {
					case resultCh <- agent.ToolResult{Content: summary, NewTools: newTools}:
					case <-toolCtx.Done():
						return
					}
				}
			case <-toolCtx.Done():
				return
			}
		}
	}()

	// The starting tool set comes from the engine's InitialToolDefs
	// directive received at session start. We copy the slice so that
	// tools loaded mid-session via load_tools (which the reasoning
	// loop appends to its own working slice) do not leak into the
	// next message.
	tools := append([]llm.ToolDefinition(nil), initialTools...)

	agentLog("info", "run_loop_start", "session", sid, "tool_count", len(tools), "provider", cfg.Provider.Name(), "max_rounds", cfg.MaxRounds)

	// Run the reasoning loop.
	agent.RunLoop(ctx, cfg, sid, mid, req.Content, mode, history, tools,
		func(event agent.LoopEvent) {
			switch event.Type {
			case agent.EventToken:
				_ = stream.Send(&pb.AgentEvent{
					Event: &pb.AgentEvent_LlmTokenEmitted{
						LlmTokenEmitted: &pb.LLMTokenEmitted{
							SessionId: sid, MessageId: mid, Text: event.Token,
						},
					},
				})

			case agent.EventToolProposal:
				agentLog("info", "tool_proposal", "session", sid, "call_id", event.Proposal.CallID, "tool", event.Proposal.ToolName)
				_ = stream.Send(&pb.AgentEvent{
					Event: &pb.AgentEvent_ToolProposal{
						ToolProposal: &pb.ToolCallProposed{
							SessionId:     sid,
							MessageId:     mid,
							CallId:        event.Proposal.CallID,
							ToolName:      event.Proposal.ToolName,
							ArgumentsJson: event.Proposal.ArgumentsJSON,
						},
					},
				})

			case agent.EventToolDefsRequest:
				_ = stream.Send(&pb.AgentEvent{
					Event: &pb.AgentEvent_ToolDefsRequest{
						ToolDefsRequest: &pb.ToolDefsRequest{
							Groups: event.RequestedGroups,
						},
					},
				})

			case agent.EventMemoryFlush:
				_ = stream.Send(&pb.AgentEvent{
					Event: &pb.AgentEvent_MemoryFlush{
						MemoryFlush: &pb.MemoryFlush{Content: event.FlushContent},
					},
				})

			case agent.EventComplete:
				agentLog("info", "response_complete", "session", sid, "content_len", len(event.Content), "thought_count", len(event.Thoughts))
				var pbThoughts []*pb.Thought
				for _, t := range event.Thoughts {
					pbThoughts = append(pbThoughts, &pb.Thought{
						Stage:   t.Stage,
						Summary: t.Summary,
					})
				}
				var pbUsage *pb.TokenUsage
				if event.Usage != nil {
					pbUsage = &pb.TokenUsage{
						InputTokens:      int32(event.Usage.InputTokens),
						OutputTokens:     int32(event.Usage.OutputTokens),
						CacheReadTokens:  int32(event.Usage.CacheReadTokens),
						CacheWriteTokens: int32(event.Usage.CacheCreationTokens),
					}
				}
				_ = stream.Send(&pb.AgentEvent{
					Event: &pb.AgentEvent_ResponseComplete{
						ResponseComplete: &pb.AgentResponseComplete{
							SessionId: sid,
							MessageId: mid,
							Content:   event.Content,
							Thoughts:  pbThoughts,
							Usage:     pbUsage,
						},
					},
				})

			case agent.EventLoopError:
				agentLog("error", "loop_error", "session", sid, "code", event.ErrorCode, "message", event.ErrorMessage)
				_ = stream.Send(&pb.AgentEvent{
					Event: &pb.AgentEvent_AgentError{
						AgentError: &pb.AgentError{
							SessionId: sid,
							MessageId: mid,
							Code:      event.ErrorCode,
							Message:   event.ErrorMessage,
						},
					},
				})
			}
		},
		resultCh,
	)
}

// formatToolDefsSummary creates a text summary of tool definitions for the LLM.
func formatToolDefsSummary(tools []*pb.ToolDef) string {
	var parts []string
	for _, t := range tools {
		parts = append(parts, fmt.Sprintf("- %s: %s", t.Name, t.Description))
	}
	return fmt.Sprintf("Loaded %d tools:\n%s", len(tools), joinStrings(parts))
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += "\n"
		}
		result += s
	}
	return result
}
