package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	pb "github.com/openparallax/openparallax/internal/types/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// setupTestWorkspace creates a minimal workspace with config and database.
func setupTestWorkspace(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()

	// Write minimal config.
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	model := os.Getenv("OPENAI_MODEL")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping engine integration test")
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	configContent := fmt.Sprintf(`workspace: %s
llm:
  provider: openai
  model: %s
  api_key_env: OPENAI_API_KEY`, dir, model)
	if baseURL != "" {
		configContent += fmt.Sprintf("\n  base_url: %s", baseURL)
	}

	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o644))

	// Create workspace structure.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".openparallax"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("You are a helpful assistant."), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "IDENTITY.md"), []byte("Name: TestBot"), 0o644))

	// Initialize database.
	db, err := storage.Open(filepath.Join(dir, ".openparallax", "openparallax.db"))
	require.NoError(t, err)
	_ = db.Close()

	return dir, configPath
}

func TestEngineStartAndStop(t *testing.T) {
	_, configPath := setupTestWorkspace(t)

	eng, err := New(configPath)
	require.NoError(t, err)

	port, err := eng.Start()
	require.NoError(t, err)
	assert.Greater(t, port, 0)

	eng.Stop()
}

func TestEngineGRPCHealth(t *testing.T) {
	_, configPath := setupTestWorkspace(t)

	eng, err := New(configPath)
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

	client := pb.NewPipelineServiceClient(conn)
	resp, err := client.GetStatus(context.Background(), &pb.StatusRequest{})
	require.NoError(t, err)
	assert.Equal(t, "TestBot", resp.AgentName)
}

func TestEngineProcessMessage(t *testing.T) {
	workspace, configPath := setupTestWorkspace(t)

	eng, err := New(configPath)
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

	client := pb.NewPipelineServiceClient(conn)
	stream, err := client.ProcessMessage(context.Background(), &pb.ProcessMessageRequest{
		Content:   "Say exactly: hello",
		SessionId: sessionID,
		MessageId: "msg-test-001",
		Mode:      pb.SessionMode_NORMAL,
		Source:    "test",
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

	eng, err := New(configPath)
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

	client := pb.NewPipelineServiceClient(conn)
	stream, err := client.ProcessMessage(context.Background(), &pb.ProcessMessageRequest{
		Content:   "",
		SessionId: sessionID,
		MessageId: "msg-test-002",
		Mode:      pb.SessionMode_NORMAL,
		Source:    "test",
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

	eng, err := New(configPath)
	require.NoError(t, err)
	port, err := eng.Start()
	require.NoError(t, err)

	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := pb.NewPipelineServiceClient(conn)
	resp, err := client.Shutdown(context.Background(), &pb.ShutdownRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Clean)
}
