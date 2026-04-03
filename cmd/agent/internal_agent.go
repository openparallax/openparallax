package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

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
			fmt.Fprintf(os.Stderr, "sandbox: failed to apply: %s\n", sbErr)
		}
	}

	// Canary probes: verify sandbox enforcement per platform.
	canary := sandbox.VerifyCanary()
	_ = sandbox.WriteCanaryResult(agentWorkspace, canary)
	fmt.Fprintf(os.Stderr, "sandbox: %s\n", canary.Summary)

	switch canary.Status {
	case "sandboxed":
		// All applicable probes blocked — proceed.
	case "unsandboxed", "partial":
		// Sandbox failed to enforce — refuse to start.
		return fmt.Errorf("sandbox verification failed: %s", canary.Summary)
	case "unavailable":
		// No sandbox mechanism available — warn but allow on unsupported platforms.
		fmt.Fprintf(os.Stderr, "sandbox: WARNING — no sandbox available on this platform, running unprotected\n")
	}

	// Create LLM provider.
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

	// Main directive loop: wait for Engine to send ProcessRequest directives.
	for {
		directive, recvErr := stream.Recv()
		if recvErr != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("recv directive: %w", recvErr)
		}

		switch d := directive.Directive.(type) {
		case *pb.EngineDirective_Process:
			processMessage(ctx, stream, loopCfg, d.Process, db)

		case *pb.EngineDirective_Shutdown:
			return nil
		}
	}
}

// processMessage runs the LLM reasoning loop for a single message.
func processMessage(
	ctx context.Context,
	stream pb.AgentService_RunSessionClient,
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

	// Load history from DB (read-only).
	var history []llm.ChatMessage
	if db != nil {
		if msgs, err := db.GetMessages(sid); err == nil {
			for _, m := range msgs {
				history = append(history, llm.ChatMessage{Role: m.Role, Content: m.Content})
			}
		}
	}

	// Result channel for tool execution responses from the Engine.
	resultCh := make(chan agent.ToolResult, 1)

	// Start a goroutine to read ToolResultDelivery directives from the stream
	// and feed them into resultCh.
	toolCtx, toolCancel := context.WithCancel(ctx)
	defer toolCancel()

	go func() {
		defer close(resultCh)
		for {
			directive, err := stream.Recv()
			if err != nil {
				return
			}
			switch d := directive.Directive.(type) {
			case *pb.EngineDirective_ToolResult:
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
				// Tool definitions delivered as a result for load_tools.
				summary := formatToolDefsSummary(d.ToolDefs.Tools)
				select {
				case resultCh <- agent.ToolResult{Content: summary}:
				case <-toolCtx.Done():
					return
				}
			case *pb.EngineDirective_Shutdown:
				return
			}
		}
	}()

	// Initial tools: just load_tools (sent by Engine or hardcoded).
	tools := []llm.ToolDefinition{{
		Name:        "load_tools",
		Description: "Request additional tool groups. Call with {\"groups\": [\"files\", \"shell\", ...]} to load tools.",
	}}

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
				var pbThoughts []*pb.Thought
				for _, t := range event.Thoughts {
					pbThoughts = append(pbThoughts, &pb.Thought{
						Stage:   t.Stage,
						Summary: t.Summary,
					})
				}
				_ = stream.Send(&pb.AgentEvent{
					Event: &pb.AgentEvent_ResponseComplete{
						ResponseComplete: &pb.AgentResponseComplete{
							SessionId: sid,
							MessageId: mid,
							Content:   event.Content,
							Thoughts:  pbThoughts,
						},
					},
				})

			case agent.EventLoopError:
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
