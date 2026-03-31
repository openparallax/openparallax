package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/sandbox"
	"github.com/openparallax/openparallax/internal/types"
	pb "github.com/openparallax/openparallax/internal/types/pb"
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
		_ = sb.ApplySelf(sandbox.Config{
			AllowedTCPConnect: []string{subAgentGRPCAddr},
			AllowProcessSpawn: false,
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	// Connect to engine.
	conn, err := grpc.NewClient(subAgentGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("connect to engine: %w", err)
	}
	defer func() { _ = conn.Close() }()

	client := pb.NewPipelineServiceClient(conn)

	// Register with engine.
	regResp, err := client.RegisterSubAgent(ctx, &pb.SubAgentRegisterRequest{Token: token})
	if err != nil {
		return fmt.Errorf("register sub-agent: %w", err)
	}

	name := regResp.Name

	// Create LLM provider with the assigned model.
	provider, err := llm.NewProvider(types.LLMConfig{
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

	// Run the LLM tool-use loop.
	maxCalls := int(regResp.MaxLlmCalls)
	if maxCalls <= 0 {
		maxCalls = 20
	}

	messages := []llm.ChatMessage{
		{Role: "user", Content: regResp.Task},
	}

	for round := 0; round < maxCalls; round++ {
		stream, streamErr := provider.StreamWithTools(ctx, messages, tools,
			llm.WithSystem(regResp.SystemPrompt), llm.WithMaxTokens(4096))
		if streamErr != nil {
			reportFailure(ctx, client, name, fmt.Sprintf("LLM call failed: %s", streamErr))
			return streamErr
		}

		var toolResults []llm.ToolResult
		var fullText string

		for {
			event, eventErr := stream.Next()
			if eventErr == io.EOF || event.Type == llm.EventDone {
				if len(toolResults) > 0 {
					if sendErr := stream.SendToolResults(toolResults); sendErr != nil {
						_ = stream.Close()
						reportFailure(ctx, client, name, fmt.Sprintf("send tool results: %s", sendErr))
						return sendErr
					}
					toolResults = nil
					continue
				}
				break
			}
			if eventErr != nil {
				_ = stream.Close()
				reportFailure(ctx, client, name, fmt.Sprintf("stream error: %s", eventErr))
				return eventErr
			}

			switch event.Type {
			case llm.EventTextDelta:
				fullText += event.Text

			case llm.EventToolCallComplete:
				tc := event.ToolCall
				argsJSON, _ := json.Marshal(tc.Arguments)

				resp, toolErr := client.SubAgentExecuteTool(ctx, &pb.SubAgentToolRequest{
					Name:          name,
					CallId:        tc.ID,
					ToolName:      tc.Name,
					ArgumentsJson: string(argsJSON),
				})
				if toolErr != nil {
					toolResults = append(toolResults, llm.ToolResult{
						CallID:  tc.ID,
						Content: "Engine error: " + toolErr.Error(),
						IsError: true,
					})
				} else {
					toolResults = append(toolResults, llm.ToolResult{
						CallID:  tc.ID,
						Content: resp.Content,
						IsError: resp.IsError,
					})
				}
			}
		}

		fullText = stream.FullText()
		_ = stream.Close()

		// If no tool calls were made, we have the final response.
		if fullText != "" {
			_, _ = client.SubAgentComplete(ctx, &pb.SubAgentCompleteRequest{
				Name:   name,
				Result: fullText,
			})
			return nil
		}

		// Tool calls were made — add assistant + tool results to messages for next round.
		messages = append(messages, llm.ChatMessage{
			Role:    "assistant",
			Content: fullText,
		})
	}

	// Max LLM calls reached without a final response.
	reportFailure(ctx, client, name, fmt.Sprintf("exceeded maximum %d LLM calls without producing a final response", maxCalls))
	return nil
}

func reportFailure(ctx context.Context, client pb.PipelineServiceClient, name, errMsg string) {
	_, _ = client.SubAgentFailed(ctx, &pb.SubAgentFailedRequest{
		Name:  name,
		Error: errMsg,
	})
}
