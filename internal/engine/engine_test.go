package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	pb "github.com/openparallax/openparallax/internal/types/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// setupTestWorkspace creates a minimal workspace with config and database.
// Skips the test if the LLM endpoint is not available.
func setupTestWorkspace(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()

	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	model := os.Getenv("OPENAI_MODEL")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping engine integration test")
	}
	if model == "" {
		model = "gpt-5.4-mini"
	}

	// Verify the endpoint is reachable.
	checkURL := "https://api.openai.com/v1/models"
	if baseURL != "" {
		checkURL = baseURL + "/models"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	req, _ := http.NewRequest("GET", checkURL, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("LLM endpoint unreachable (%s): %v", checkURL, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Skip("OPENAI_API_KEY is invalid or expired, skipping engine integration test")
	}

	// Create a minimal policy file in the test workspace.
	policyDir := filepath.Join(dir, "policies")
	require.NoError(t, os.MkdirAll(policyDir, 0o755))
	policyContent := `allow:
  - name: allow_all_reads
    action_types:
      - read_file
      - list_directory
      - search_files
      - memory_search
      - git_status
      - git_diff
      - git_log
verify:
  - name: evaluate_shell
    action_types:
      - execute_command
    tier_override: 1
`
	policyPath := filepath.Join(policyDir, "test.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(policyContent), 0o644))

	// Create a minimal evaluator prompt.
	promptDir := filepath.Join(dir, "prompts")
	require.NoError(t, os.MkdirAll(promptDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(promptDir, "evaluator-v1.md"),
		[]byte("You are a security evaluator. Canary: {{CANARY_TOKEN}}"), 0o644))

	configContent := fmt.Sprintf("workspace: %s\nllm:\n  provider: openai\n  model: %s\n  api_key_env: OPENAI_API_KEY\n", dir, model)
	if baseURL != "" {
		configContent += fmt.Sprintf("  base_url: %s\n", baseURL)
	}
	configContent += "identity:\n  name: TestBot\n"
	configContent += fmt.Sprintf("shield:\n  policy_file: %s\n  heuristic_enabled: true\n", policyPath)

	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o644))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".openparallax"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("You are a helpful assistant."), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "IDENTITY.md"), []byte("Name: TestBot"), 0o644))

	db, err := storage.Open(filepath.Join(dir, ".openparallax", "openparallax.db"))
	require.NoError(t, err)
	_ = db.Close()

	return dir, configPath
}

func TestEngineStartAndStop(t *testing.T) {
	_, configPath := setupTestWorkspace(t)

	eng, err := New(configPath, false)
	require.NoError(t, err)

	port, err := eng.Start()
	require.NoError(t, err)
	assert.Greater(t, port, 0)

	eng.Stop()
}

func TestEngineGRPCHealth(t *testing.T) {
	_, configPath := setupTestWorkspace(t)

	eng, err := New(configPath, false)
	require.NoError(t, err)
	port, err := eng.Start()
	require.NoError(t, err)
	defer eng.Stop()

	// Connect as a client.
	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := pb.NewClientServiceClient(conn)
	resp, err := client.GetStatus(context.Background(), &pb.StatusRequest{})
	require.NoError(t, err)
	assert.Equal(t, "TestBot", resp.AgentName)
}

func TestEngineProcessMessage(t *testing.T) {
	workspace, configPath := setupTestWorkspace(t)

	eng, err := New(configPath, false)
	require.NoError(t, err)
	port, err := eng.Start()
	require.NoError(t, err)
	defer eng.Stop()

	// Create a session in the database.
	db, err := storage.Open(filepath.Join(workspace, ".openparallax", "openparallax.db"))
	require.NoError(t, err)
	sessionID := "test-session-001"
	require.NoError(t, db.InsertSession(&types.Session{
		ID:        sessionID,
		Mode:      types.SessionNormal,
		CreatedAt: types.Session{}.CreatedAt, // zero time is fine for test
	}))
	_ = db.Close()

	// Connect as a client.
	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := pb.NewClientServiceClient(conn)
	stream, err := client.SendMessage(context.Background(), &pb.ClientMessageRequest{
		Content:   "Say exactly: hello",
		SessionId: sessionID,

		Mode:   pb.SessionMode_NORMAL,
		Source: "test",
	})
	require.NoError(t, err)

	var gotToken bool
	var gotComplete bool
	var fullContent string

	for {
		event, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		require.NoError(t, recvErr)

		switch event.EventType {
		case pb.PipelineEventType_LLM_TOKEN:
			gotToken = true
		case pb.PipelineEventType_RESPONSE_COMPLETE:
			gotComplete = true
			fullContent = event.ResponseComplete.Content
		case pb.PipelineEventType_ERROR:
			t.Fatalf("pipeline error: %s: %s", event.PipelineError.Code, event.PipelineError.Message)
		}
	}

	assert.True(t, gotToken, "should have received at least one LLM_TOKEN event")
	assert.True(t, gotComplete, "should have received RESPONSE_COMPLETE event")
	assert.NotEmpty(t, fullContent, "response content should not be empty")
}

func TestEngineProcessMessageEmptyContent(t *testing.T) {
	workspace, configPath := setupTestWorkspace(t)

	eng, err := New(configPath, false)
	require.NoError(t, err)
	port, err := eng.Start()
	require.NoError(t, err)
	defer eng.Stop()

	db, err := storage.Open(filepath.Join(workspace, ".openparallax", "openparallax.db"))
	require.NoError(t, err)
	sessionID := "test-session-002"
	require.NoError(t, db.InsertSession(&types.Session{ID: sessionID, Mode: types.SessionNormal}))
	_ = db.Close()

	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := pb.NewClientServiceClient(conn)
	stream, err := client.SendMessage(context.Background(), &pb.ClientMessageRequest{
		Content:   "",
		SessionId: sessionID,

		Mode:   pb.SessionMode_NORMAL,
		Source: "test",
	})
	require.NoError(t, err)

	// Should still get a response (LLM handles empty gracefully).
	var gotResponse bool
	for {
		event, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			break
		}
		if event.EventType == pb.PipelineEventType_RESPONSE_COMPLETE ||
			event.EventType == pb.PipelineEventType_ERROR {
			gotResponse = true
		}
	}
	assert.True(t, gotResponse, "should have received either a response or error")
}

func TestEngineShutdownRPC(t *testing.T) {
	_, configPath := setupTestWorkspace(t)

	eng, err := New(configPath, false)
	require.NoError(t, err)
	port, err := eng.Start()
	require.NoError(t, err)

	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := pb.NewClientServiceClient(conn)
	resp, err := client.Shutdown(context.Background(), &pb.ShutdownRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Clean)
}

// helperSendAndCollect sends a message and collects all pipeline events.
func helperSendAndCollect(t *testing.T, client pb.ClientServiceClient, sessionID, content string) map[pb.PipelineEventType]bool {
	t.Helper()
	stream, err := client.SendMessage(context.Background(), &pb.ClientMessageRequest{
		Content:   content,
		SessionId: sessionID,

		Mode:   pb.SessionMode_NORMAL,
		Source: "test",
	})
	require.NoError(t, err)

	events := make(map[pb.PipelineEventType]bool)
	for {
		event, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			break
		}
		events[event.EventType] = true
	}
	return events
}

func TestEngineFullPipelineReadFile(t *testing.T) {
	workspace, configPath := setupTestWorkspace(t)

	eng, err := New(configPath, false)
	require.NoError(t, err)
	port, err := eng.Start()
	require.NoError(t, err)
	defer eng.Stop()

	db, err := storage.Open(filepath.Join(workspace, ".openparallax", "openparallax.db"))
	require.NoError(t, err)
	sessionID := "pipe-session-001"
	require.NoError(t, db.InsertSession(&types.Session{ID: sessionID, Mode: types.SessionNormal}))
	_ = db.Close()

	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := pb.NewClientServiceClient(conn)

	// Ask to read SOUL.md which exists in the workspace.
	events := helperSendAndCollect(t, client, sessionID, "read the file SOUL.md")

	assert.True(t, events[pb.PipelineEventType_RESPONSE_COMPLETE], "should have RESPONSE_COMPLETE")
}

func TestEngineConversationMode(t *testing.T) {
	workspace, configPath := setupTestWorkspace(t)

	eng, err := New(configPath, false)
	require.NoError(t, err)
	port, err := eng.Start()
	require.NoError(t, err)
	defer eng.Stop()

	db, err := storage.Open(filepath.Join(workspace, ".openparallax", "openparallax.db"))
	require.NoError(t, err)
	sessionID := "conv-session-001"
	require.NoError(t, db.InsertSession(&types.Session{ID: sessionID, Mode: types.SessionNormal}))
	_ = db.Close()

	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := pb.NewClientServiceClient(conn)

	// Pure conversation — should not trigger action execution.
	events := helperSendAndCollect(t, client, sessionID, "what is 2+2")

	assert.True(t, events[pb.PipelineEventType_RESPONSE_COMPLETE], "should have RESPONSE_COMPLETE")
	// Should NOT have action events (but might have INTENT_PARSED).
	assert.False(t, events[pb.PipelineEventType_ACTION_STARTED], "conversation should not start actions")
}
