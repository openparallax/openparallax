package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupStatusReturnsSetupRequired(t *testing.T) {
	s := NewSetupServer(0)
	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	s.handleSetupStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, true, body["setup_required"])
}

func TestTestProviderRejectsEmptyBody(t *testing.T) {
	s := NewSetupServer(0)
	req := httptest.NewRequest("POST", "/api/setup/test-provider",
		strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	s.handleTestProvider(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, false, body["success"])
}

func TestSetupCompleteCreatesWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	workspace := filepath.Join(tmpDir, "test-agent")

	s := NewSetupServer(0)
	reqBody := `{
		"agent": {"name": "TestBot", "avatar": "🤖"},
		"llm": {"provider": "anthropic", "api_key": "test", "model": "claude-sonnet-4-20250514"},
		"embedding": {"provider": "", "api_key": "", "model": ""},
		"workspace": "` + workspace + `"
	}`

	req := httptest.NewRequest("POST", "/api/setup/complete",
		strings.NewReader(reqBody))
	w := httptest.NewRecorder()

	// Run in goroutine since handleSetupComplete sends to doneCh.
	go s.handleSetupComplete(w, req)

	result := <-s.Done()
	assert.Equal(t, workspace, result.Workspace)

	// Verify config was created.
	configPath := filepath.Join(workspace, "config.yaml")
	assert.FileExists(t, configPath)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "name: TestBot")
	assert.Contains(t, content, `avatar: 🤖`)
	assert.Contains(t, content, "provider: anthropic")
}

func TestSetupCompleteDefaultsAgentName(t *testing.T) {
	tmpDir := t.TempDir()
	workspace := filepath.Join(tmpDir, "default-agent")

	s := NewSetupServer(0)
	reqBody := `{
		"agent": {"name": "", "avatar": ""},
		"llm": {"provider": "openai", "api_key": "test", "model": "gpt-4o"},
		"embedding": {},
		"workspace": "` + workspace + `"
	}`

	req := httptest.NewRequest("POST", "/api/setup/complete",
		strings.NewReader(reqBody))
	w := httptest.NewRecorder()

	go s.handleSetupComplete(w, req)
	<-s.Done()

	data, err := os.ReadFile(filepath.Join(workspace, "config.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "name: Atlas")
}

func TestSetupEndpointsAreOnlyForSetup(t *testing.T) {
	// After setup completes, the setup server stops and the main server starts.
	// The main server does not have /api/setup/* routes.
	// This test verifies setup endpoints exist on SetupServer.
	s := NewSetupServer(0)

	statusReq := httptest.NewRequest("GET", "/api/status", nil)
	statusW := httptest.NewRecorder()
	s.handleSetupStatus(statusW, statusReq)
	assert.Equal(t, http.StatusOK, statusW.Code)
}
