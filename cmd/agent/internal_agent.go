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
		llmHost := llm.APIHost(cfg.LLM)
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
	agentLog("info", "llm_provider_init", "provider", cfg.LLM.Provider, "model", cfg.LLM.Model)
	provider, err := llm.NewProvider(cfg.LLM)
	if err != nil {
		return fmt.Errorf("llm provider: %w", err)
	}

	// Open read-only DB for history access.
	dbPath := filepath.Join(agentWorkspace, ".openparallax", "openparallax.db")
	db, _ := storage.Open(dbPath)

	// Create agent (context assembly, compaction, skills).
	ag := agent.NewAgent(provider, agentWorkspace, nil)

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

	// Signal readiness.
	if sendErr := stream.Send(&pb.AgentEvent{
		Event: &pb.AgentEvent_Ready{Ready: &pb.AgentReady{AgentId: agentName}},
	}); sendErr != nil {
		return fmt.Errorf("send ready: %w", sendErr)
	}

	loopCfg := agent.LoopConfig{
		Provider:      provider,
		Agent:         ag,
		MaxRounds:     25,
		ContextWindow: 128000,
	}

	// directiveCh carries tool results from the stream reader to the active
	// processMessage call. Buffered so the reader doesn't block.
	directiveCh := make(chan *pb.EngineDirective, 4)

	// Stream reader goroutine: reads all directives and routes them.
	// ProcessRequest and Shutdown are handled in the main goroutine;
	// ToolResult/ToolDefs are forwarded to the active processMessage.
	processCh := make(chan *pb.ProcessRequest, 1)
	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)
		for {
			directive, recvErr := stream.Recv()
			if recvErr != nil {
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

	// Main directive loop.
	for {
		select {
		case req, ok := <-processCh:
			if !ok {
				return nil
			}
			processMessage(ctx, stream, directiveCh, loopCfg, req, db)

		case <-doneCh:
			return nil

		case <-ctx.Done():
			return nil
		}
	}
}

// processMessage runs the LLM reasoning loop for a single message.
// directiveCh receives ToolResult/ToolDefs directives from the outer stream
// reader goroutine; processMessage does not read from the gRPC stream directly.
func processMessage(
	ctx context.Context,
	stream pb.AgentService_RunSessionClient,
	directiveCh <-chan *pb.EngineDirective,
	cfg agent.LoopConfig,
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
					select {
					case resultCh <- agent.ToolResult{Content: summary}:
					case <-toolCtx.Done():
						return
					}
				}
			case <-toolCtx.Done():
				return
			}
		}
	}()

	// Initial tools: just load_tools (sent by Engine or hardcoded).
	tools := []llm.ToolDefinition{{
		Name:        "load_tools",
		Description: "Request additional tool groups. Call with {\"groups\": [\"files\", \"shell\", ...]} to load tools.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"groups": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Tool group names to load (e.g. files, shell, git, browser, memory).",
				},
			},
			"required": []string{"groups"},
		},
	}}

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
