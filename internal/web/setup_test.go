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
	t.Setenv("OP_DATA_DIR", tmpDir)
	workspace := filepath.Join(tmpDir, "test-agent")

	s := NewSetupServer(0)
	reqBody := `{
		"agent": {"name": "TestBot", "avatar": "🤖"},
		"llm": {"provider": "anthropic", "api_key": "test", "model": "claude-sonnet-4-6"},
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
	assert.Contains(t, content, `\U0001F916`)
	assert.Contains(t, content, "provider: anthropic")
}

func TestSetupCompleteDefaultsAgentName(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("OP_DATA_DIR", tmpDir)
	workspace := filepath.Join(tmpDir, "default-agent")

	s := NewSetupServer(0)
	reqBody := `{
		"agent": {"name": "", "avatar": ""},
		"llm": {"provider": "openai", "api_key": "test", "model": "gpt-5.4"},
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

func TestSetupCompleteRejectsWorkspaceOutsideAllowedRoots(t *testing.T) {
	t.Setenv("OP_DATA_DIR", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	s := NewSetupServer(0)
	reqBody := `{
		"agent": {"name": "TestBot"},
		"llm": {"provider": "anthropic", "api_key": "test", "model": "claude-sonnet-4-6"},
		"embedding": {},
		"workspace": "/etc/openparallax-attack"
	}`
	req := httptest.NewRequest("POST", "/api/setup/complete", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	s.handleSetupComplete(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "$HOME")

	// And the directory must not have been created.
	_, err := os.Stat("/etc/openparallax-attack")
	assert.True(t, os.IsNotExist(err))
}

func TestValidateSetupWorkspace(t *testing.T) {
	home := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OP_DATA_DIR", dataDir)

	t.Run("under home", func(t *testing.T) {
		assert.NoError(t, validateSetupWorkspace(filepath.Join(home, "atlas")))
	})
	t.Run("under op_data_dir", func(t *testing.T) {
		assert.NoError(t, validateSetupWorkspace(filepath.Join(dataDir, "atlas")))
	})
	t.Run("escapes via parent", func(t *testing.T) {
		assert.Error(t, validateSetupWorkspace(filepath.Join(home, "..", "etc")))
	})
	t.Run("absolute outside both", func(t *testing.T) {
		assert.Error(t, validateSetupWorkspace("/etc/passwd"))
	})
	t.Run("relative path", func(t *testing.T) {
		assert.Error(t, validateSetupWorkspace("relative/path"))
	})
	t.Run("empty", func(t *testing.T) {
		assert.Error(t, validateSetupWorkspace(""))
	})
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
