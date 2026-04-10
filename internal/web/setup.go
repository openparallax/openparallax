package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/llm"
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
		Handler:      withCORS(mux, nil),
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

	cfg := llm.Config{
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
	cfg := llm.Config{
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

	// The setup endpoint runs before any config exists and is unauthenticated.
	// body.Workspace is fed straight to os.MkdirAll, so reject any path that
	// escapes the user's home directory or an explicit OP_DATA_DIR root.
	if err := validateSetupWorkspace(body.Workspace); err != nil {
		slog.Warn("setup_workspace_rejected", "workspace", body.Workspace, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Create workspace.
	if err := os.MkdirAll(body.Workspace, 0o755); err != nil {
		slog.Error("setup_create_workspace_failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	dotDir := filepath.Join(body.Workspace, ".openparallax")
	if err := os.MkdirAll(dotDir, 0o755); err != nil {
		slog.Error("setup_create_internal_dir_failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Write config.
	configPath := filepath.Join(body.Workspace, "config.yaml")
	if err := writeSetupConfig(configPath, body); err != nil {
		slog.Error("setup_write_config_failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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

// defaultAPIKeyEnv returns the conventional env var name for a provider.
// Empty for ollama (no key required).
func defaultAPIKeyEnv(provider string) string {
	switch provider {
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "openai":
		return "OPENAI_API_KEY"
	case "google":
		return "GOOGLE_AI_API_KEY"
	}
	return ""
}

// writeSetupConfig generates config.yaml from the web wizard.
func writeSetupConfig(path string, req setupCompleteRequest) error {
	// Start from the canonical defaults — single source of truth.
	defaults := config.DefaultConfig()
	cfg := &defaults
	cfg.Workspace = req.Workspace
	cfg.Identity = types.IdentityConfig{Name: req.Agent.Name, Avatar: req.Agent.Avatar}
	cfg.Roles = types.RolesConfig{Chat: "chat", Shield: "chat", SubAgent: "chat"}
	cfg.Models = []types.ModelEntry{
		{Name: "chat", Provider: req.LLM.Provider, Model: req.LLM.Model, APIKeyEnv: defaultAPIKeyEnv(req.LLM.Provider), BaseURL: req.LLM.BaseURL},
	}
	cfg.Shield.ClassifierEnabled = true
	cfg.Shield.ClassifierMode = "local"

	if req.Embedding.Provider != "" {
		cfg.Models = append(cfg.Models, types.ModelEntry{
			Name:     "embedding",
			Provider: req.Embedding.Provider,
			Model:    req.Embedding.Model,
		})
		cfg.Roles.Embedding = "embedding"
	}

	return config.Save(path, cfg)
}

// expandPath replaces ~ with home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

// validateSetupWorkspace asserts that the wizard-supplied workspace path is
// confined to the user's home directory or an explicit OP_DATA_DIR root.
// The setup endpoint is unauthenticated (it runs before any config exists)
// and feeds the path straight to os.MkdirAll, so a hostile request body
// could otherwise create directories anywhere the engine process can write.
// The path must already be absolute and tilde-expanded by the caller.
func validateSetupWorkspace(workspace string) error {
	if workspace == "" {
		return fmt.Errorf("workspace path is required")
	}
	clean := filepath.Clean(workspace)
	if !filepath.IsAbs(clean) {
		return fmt.Errorf("workspace path must be absolute")
	}

	var roots []string
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		roots = append(roots, filepath.Clean(home))
	}
	if dataDir := os.Getenv("OP_DATA_DIR"); dataDir != "" {
		roots = append(roots, filepath.Clean(dataDir))
	}
	if len(roots) == 0 {
		return fmt.Errorf("no allowed workspace root: set $HOME or $OP_DATA_DIR")
	}

	for _, root := range roots {
		rel, err := filepath.Rel(root, clean)
		if err != nil {
			continue
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return nil
		}
	}
	return fmt.Errorf("workspace path %q must be under $HOME or $OP_DATA_DIR", workspace)
}
