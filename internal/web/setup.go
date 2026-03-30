package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/types"
)

// SetupServer serves the web UI in setup mode when no config exists.
// It provides endpoints for the onboarding wizard and transitions to
// the full server once setup completes.
type SetupServer struct {
	port   int
	server *http.Server
	doneCh chan SetupResult
}

// SetupResult is returned when the user completes the web wizard.
type SetupResult struct {
	ConfigPath string
	Workspace  string
}

// NewSetupServer creates a setup-mode web server.
func NewSetupServer(port int) *SetupServer {
	return &SetupServer{
		port:   port,
		doneCh: make(chan SetupResult, 1),
	}
}

// Done returns a channel that receives the setup result when complete.
func (s *SetupServer) Done() <-chan SetupResult { return s.doneCh }

// Start begins serving the setup wizard.
func (s *SetupServer) Start() error {
	mux := http.NewServeMux()

	// Setup-only API endpoints.
	mux.HandleFunc("GET /api/status", s.handleSetupStatus)
	mux.HandleFunc("POST /api/setup/test-provider", s.handleTestProvider)
	mux.HandleFunc("POST /api/setup/test-embedding", s.handleTestEmbedding)
	mux.HandleFunc("POST /api/setup/complete", s.handleSetupComplete)

	// Static files.
	staticFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		return fmt.Errorf("setup server: sub filesystem: %w", err)
	}
	fileServer := http.FileServer(http.FS(staticFS))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		if f, openErr := staticFS.Open(path[1:]); openErr == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      withCORS(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("setup server listen: %w", err)
	}
	return s.server.Serve(listener)
}

// Stop gracefully shuts down the setup server.
func (s *SetupServer) Stop() {
	if s.server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.server.Shutdown(ctx)
}

// handleSetupStatus signals the UI that setup is required.
func (s *SetupServer) handleSetupStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"setup_required": true,
	})
}

// handleTestProvider tests an LLM provider connection.
func (s *SetupServer) handleTestProvider(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Provider string `json:"provider"`
		APIKey   string `json:"api_key"`
		Model    string `json:"model"`
		BaseURL  string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	cfg := types.LLMConfig{
		Provider: body.Provider,
		Model:    body.Model,
		BaseURL:  body.BaseURL,
	}

	if err := llm.TestConnection(cfg, body.APIKey); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"model":   body.Model,
	})
}

// handleTestEmbedding tests an embedding provider connection.
func (s *SetupServer) handleTestEmbedding(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Provider string `json:"provider"`
		APIKey   string `json:"api_key"`
		Model    string `json:"model"`
		BaseURL  string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// For embedding, we just verify the provider is reachable.
	cfg := types.LLMConfig{
		Provider: body.Provider,
		Model:    body.Model,
		BaseURL:  body.BaseURL,
	}

	if err := llm.TestConnection(cfg, body.APIKey); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"model":   body.Model,
	})
}

// setupCompleteRequest is the body for POST /api/setup/complete.
type setupCompleteRequest struct {
	Agent struct {
		Name   string `json:"name"`
		Avatar string `json:"avatar"`
	} `json:"agent"`
	LLM struct {
		Provider string `json:"provider"`
		APIKey   string `json:"api_key"`
		Model    string `json:"model"`
		BaseURL  string `json:"base_url"`
	} `json:"llm"`
	Embedding struct {
		Provider string `json:"provider"`
		APIKey   string `json:"api_key"`
		Model    string `json:"model"`
	} `json:"embedding"`
	Workspace string `json:"workspace"`
}

// handleSetupComplete writes config.yaml, creates the workspace, and signals completion.
func (s *SetupServer) handleSetupComplete(w http.ResponseWriter, r *http.Request) {
	var body setupCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if body.Agent.Name == "" {
		body.Agent.Name = "Atlas"
	}
	if body.Workspace == "" {
		home, _ := os.UserHomeDir()
		body.Workspace = filepath.Join(home, ".openparallax", strings.ToLower(body.Agent.Name))
	}
	body.Workspace = expandPath(body.Workspace)

	// Create workspace.
	if err := os.MkdirAll(body.Workspace, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create workspace: "+err.Error())
		return
	}
	dotDir := filepath.Join(body.Workspace, ".openparallax")
	if err := os.MkdirAll(dotDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create internal dir: "+err.Error())
		return
	}

	// Write config.
	configPath := filepath.Join(body.Workspace, "config.yaml")
	if err := writeSetupConfig(configPath, body); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write config: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"workspace": body.Workspace,
	})

	s.doneCh <- SetupResult{
		ConfigPath: configPath,
		Workspace:  body.Workspace,
	}
}

// writeSetupConfig generates config.yaml from the web wizard.
func writeSetupConfig(path string, req setupCompleteRequest) error {
	var sb strings.Builder
	sb.WriteString("# OpenParallax Configuration\n")
	sb.WriteString("# Generated by web setup wizard\n\n")

	fmt.Fprintf(&sb, "workspace: %s\n\n", req.Workspace)

	sb.WriteString("identity:\n")
	fmt.Fprintf(&sb, "  name: %s\n", req.Agent.Name)
	if req.Agent.Avatar != "" {
		fmt.Fprintf(&sb, "  avatar: %s\n", req.Agent.Avatar)
	}
	sb.WriteString("\n")

	sb.WriteString("llm:\n")
	fmt.Fprintf(&sb, "  provider: %s\n", req.LLM.Provider)
	fmt.Fprintf(&sb, "  model: %s\n", req.LLM.Model)
	if req.LLM.BaseURL != "" {
		fmt.Fprintf(&sb, "  base_url: %s\n", req.LLM.BaseURL)
	}
	sb.WriteString("\n")

	sb.WriteString("shield:\n")
	sb.WriteString("  evaluator:\n")
	fmt.Fprintf(&sb, "    provider: %s\n", req.LLM.Provider)
	fmt.Fprintf(&sb, "    model: %s\n", req.LLM.Model)
	sb.WriteString("  policy_file: policies/default.yaml\n")
	sb.WriteString("  heuristic_enabled: true\n\n")

	if req.Embedding.Provider != "" {
		sb.WriteString("memory:\n")
		sb.WriteString("  embedding:\n")
		fmt.Fprintf(&sb, "    provider: %s\n", req.Embedding.Provider)
		fmt.Fprintf(&sb, "    model: %s\n", req.Embedding.Model)
		sb.WriteString("\n")
	}

	sb.WriteString("chronicle:\n")
	sb.WriteString("  max_snapshots: 100\n")
	sb.WriteString("  max_age_days: 30\n\n")

	sb.WriteString("web:\n")
	sb.WriteString("  enabled: true\n")
	sb.WriteString("  port: 3100\n")
	sb.WriteString("  auth: true\n\n")

	sb.WriteString("general:\n")
	sb.WriteString("  fail_closed: true\n")
	sb.WriteString("  rate_limit: 30\n")
	sb.WriteString("  verdict_ttl_seconds: 60\n")
	sb.WriteString("  daily_budget: 100\n")

	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// expandPath replaces ~ with home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
