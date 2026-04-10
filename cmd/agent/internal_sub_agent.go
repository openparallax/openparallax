package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/openparallax/openparallax/internal/agent"
	"github.com/openparallax/openparallax/internal/types"
	pb "github.com/openparallax/openparallax/internal/types/pb"
	"github.com/openparallax/openparallax/llm"
	"github.com/openparallax/openparallax/sandbox"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	subAgentGRPCAddr  string
	subAgentWorkspace string
)

var internalSubAgentCmd = &cobra.Command{
	Use:          "internal-sub-agent",
	Short:        "Run a sub-agent process (internal use only)",
	Hidden:       true,
	SilenceUsage: true,
	RunE:         runInternalSubAgent,
}

func init() {
	internalSubAgentCmd.Flags().StringVar(&subAgentGRPCAddr, "grpc", "", "engine gRPC address")
	internalSubAgentCmd.Flags().StringVar(&subAgentWorkspace, "workspace", "", "workspace path")
	rootCmd.AddCommand(internalSubAgentCmd)
}

func runInternalSubAgent(_ *cobra.Command, _ []string) error {
	if subAgentGRPCAddr == "" {
		return fmt.Errorf("--grpc flag is required")
	}

	token := os.Getenv("OPENPARALLAX_SUB_AGENT_TOKEN")
	if token == "" {
		return fmt.Errorf("OPENPARALLAX_SUB_AGENT_TOKEN not set")
	}

	// Apply kernel sandbox before any network calls.
	sb := sandbox.New()
	if sb.Available() {
		if applyErr := sb.ApplySelf(sandbox.Config{
			AllowedTCPConnect: []string{subAgentGRPCAddr},
			AllowProcessSpawn: false,
		}); applyErr != nil {
			fmt.Fprintf(os.Stderr, "warning: sandbox apply failed: %s\n", applyErr)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	conn, err := grpc.NewClient(subAgentGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("connect to engine: %w", err)
	}
	defer func() { _ = conn.Close() }()

	client := pb.NewSubAgentServiceClient(conn)

	regResp, err := client.RegisterSubAgent(ctx, &pb.SubAgentRegisterRequest{Token: token})
	if err != nil {
		return fmt.Errorf("register sub-agent: %w", err)
	}

	name := regResp.Name

	provider, err := llm.NewProvider(llm.Config{
		Provider:  regResp.Provider,
		Model:     regResp.Model,
		APIKeyEnv: regResp.ApiKeyEnv,
		BaseURL:   regResp.BaseUrl,
	})
	if err != nil {
		reportFailure(ctx, client, name, fmt.Sprintf("create LLM provider: %s", err))
		return err
	}

	// Parse tool definitions.
	var tools []llm.ToolDefinition
	for _, td := range regResp.Tools {
		var params map[string]any
		if td.ParametersJson != "" {
			_ = json.Unmarshal([]byte(td.ParametersJson), &params)
		}
		tools = append(tools, llm.ToolDefinition{
			Name:        td.Name,
			Description: td.Description,
			Parameters:  params,
		})
	}

	// Use the shared reasoning loop.
	maxCalls := int(regResp.MaxLlmCalls)
	if maxCalls <= 0 {
		maxCalls = 20
	}

	cfg := agent.LoopConfig{
		Provider:      provider,
		Agent:         agent.NewAgent(provider, subAgentWorkspace, nil, nil, nil),
		MaxRounds:     maxCalls,
		ContextWindow: 128000,
	}

	history := []llm.ChatMessage{}
	content, loopErr := runSubAgentLoop(ctx, cfg, name, regResp.Task, history, tools, client)

	if loopErr != nil {
		reportFailure(ctx, client, name, loopErr.Error())
		return loopErr
	}

	if content == "" {
		reportFailure(ctx, client, name, "no response produced")
		return nil
	}

	// Poll for follow-up messages from the main agent.
	for {
		pollResp, pollErr := client.SubAgentPollMessage(ctx, &pb.SubAgentPollRequest{Name: name})
		if pollErr != nil || !pollResp.HasMessage {
			break
		}
		// Build conversation history for the follow-up.
		history = append(history, llm.ChatMessage{Role: "assistant", Content: content})
		history = append(history, llm.ChatMessage{Role: "user", Content: pollResp.Content})

		content, loopErr = runSubAgentLoop(ctx, cfg, name, pollResp.Content, history, tools, client)
		if loopErr != nil {
			reportFailure(ctx, client, name, loopErr.Error())
			return loopErr
		}
		if content == "" {
			reportFailure(ctx, client, name, "no response produced for follow-up message")
			return nil
		}
	}

	_, _ = client.SubAgentComplete(ctx, &pb.SubAgentCompleteRequest{
		Name:   name,
		Result: content,
	})

	return nil
}

// runSubAgentLoop executes a single reasoning loop iteration and returns the
// final content and any error. It blocks until the loop completes.
func runSubAgentLoop(
	ctx context.Context,
	cfg agent.LoopConfig,
	name, task string,
	history []llm.ChatMessage,
	tools []llm.ToolDefinition,
	client pb.SubAgentServiceClient,
) (string, error) {
	resultCh := make(chan agent.ToolResult, 1)
	var finalContent string
	var loopErr error

	agent.RunLoop(ctx, cfg, "", "", task, types.SessionNormal,
		history, tools,
		func(event agent.LoopEvent) {
			switch event.Type {
			case agent.EventToolProposal:
				resp, toolErr := client.SubAgentExecuteTool(ctx, &pb.SubAgentToolRequest{
					Name:          name,
					CallId:        event.Proposal.CallID,
					ToolName:      event.Proposal.ToolName,
					ArgumentsJson: event.Proposal.ArgumentsJSON,
				})
				if toolErr != nil {
					resultCh <- agent.ToolResult{
						CallID:  event.Proposal.CallID,
						Content: "Engine error: " + toolErr.Error(),
						IsError: true,
					}
				} else {
					resultCh <- agent.ToolResult{
						CallID:  event.Proposal.CallID,
						Content: resp.Content,
						IsError: resp.IsError,
					}
				}

			case agent.EventComplete:
				finalContent = event.Content

			case agent.EventLoopError:
				loopErr = fmt.Errorf("%s: %s", event.ErrorCode, event.ErrorMessage)

			case agent.EventToken, agent.EventToolDefsRequest, agent.EventMemoryFlush:
				// Sub-agents don't stream tokens or request tool defs.
			}
		},
		resultCh,
	)

	return finalContent, loopErr
}

func reportFailure(ctx context.Context, client pb.SubAgentServiceClient, name, errMsg string) {
	_, _ = client.SubAgentFailed(ctx, &pb.SubAgentFailedRequest{
		Name:  name,
		Error: errMsg,
	})
}
