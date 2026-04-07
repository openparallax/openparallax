package web

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/mcp"
)

func (s *Server) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	s.log.Debug("api_get_settings")
	cfg := s.engine.Config()
	shieldStatus := s.engine.ShieldStatus()
	chat, _ := cfg.ChatModel()
	shieldM, _ := cfg.ShieldModel()
	embM, _ := cfg.EmbeddingModel()

	resp := map[string]any{
		"agent": map[string]any{
			"name":   cfg.Identity.Name,
			"avatar": cfg.Identity.Avatar,
		},
		"chat": map[string]any{
			"provider":           chat.Provider,
			"model":              chat.Model,
			"api_key_configured": isKeyConfigured(chat.APIKeyEnv),
			"base_url":           chat.BaseURL,
		},
		"shield": map[string]any{
			"policy": policyName(cfg.Shield.PolicyFile),
			"evaluator": map[string]any{
				"provider": shieldM.Provider,
				"model":    shieldM.Model,
			},
			"tier2_budget":     cfg.General.DailyBudget,
			"tier2_used_today": shieldStatus["tier2_used"],
		},
		"memory": map[string]any{
			"embedding": map[string]any{
				"provider":           embM.Provider,
				"model":              embM.Model,
				"api_key_configured": isKeyConfigured(embM.APIKeyEnv),
				"base_url":           embM.BaseURL,
			},
		},
		"mcp": map[string]any{
			"servers": s.engine.MCPServerStatus(),
		},
		"email":    s.buildEmailSettings(cfg),
		"calendar": s.buildCalendarSettings(cfg),
		"web": map[string]any{
			"port": cfg.Web.Port,
		},
		"sandbox": s.engine.SandboxStatus(),
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePutSettings(w http.ResponseWriter, _ *http.Request) {
	s.log.Warn("api_settings_update_unavailable")
	writeError(w, http.StatusServiceUnavailable, "settings updates are temporarily unavailable while the persistence layer is rewritten")
}

func (s *Server) handleTestMCP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string            `json:"name"`
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.log.Warn("api_test_mcp_bad_request", "error", err)
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if body.Command == "" {
		s.log.Warn("api_test_mcp_bad_request", "error", "command is required")
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}

	s.log.Info("api_test_mcp", "name", body.Name, "command", body.Command)
	cfg := types.MCPServerConfig{
		Name:    body.Name,
		Command: body.Command,
		Args:    body.Args,
		Env:     body.Env,
	}

	// Try to discover tools with a timeout.
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	p, err := mcp.TestServer(ctx, cfg, s.log)
	if err != nil {
		s.log.Warn("api_test_mcp_failed", "name", body.Name, "error", err)
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	s.log.Info("api_test_mcp_success", "name", body.Name, "tools", len(p))
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"tools":   p,
	})
}

// isKeyConfigured checks if an API key env var is set.
func isKeyConfigured(envName string) bool {
	if envName == "" {
		return false
	}
	if os.Getenv(envName) != "" {
		return true
	}
	return os.Getenv("OP_"+envName) != ""
}

// policyName extracts the policy name from a file path.
func policyName(path string) string {
	if path == "" {
		return "default"
	}
	name := path
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.TrimSuffix(name, ".yaml")
	name = strings.TrimSuffix(name, ".yml")
	return name
}

func (s *Server) buildEmailSettings(cfg *types.AgentConfig) map[string]any {
	result := map[string]any{
		"provider":   cfg.Email.Provider,
		"configured": cfg.Email.SMTP.Host != "",
		"from":       cfg.Email.SMTP.From,
	}
	oauthMgr := s.engine.OAuthManager()
	if oauthMgr != nil {
		ctx := context.Background()
		googleAccounts, _ := oauthMgr.ListAccounts(ctx, "google")
		msAccounts, _ := oauthMgr.ListAccounts(ctx, "microsoft")
		var oauthAccounts []string
		oauthAccounts = append(oauthAccounts, googleAccounts...)
		oauthAccounts = append(oauthAccounts, msAccounts...)
		result["oauth_accounts"] = oauthAccounts
	}
	return result
}

func (s *Server) buildCalendarSettings(cfg *types.AgentConfig) map[string]any {
	result := map[string]any{
		"provider":   cfg.Calendar.Provider,
		"configured": cfg.Calendar.Provider != "",
	}
	oauthMgr := s.engine.OAuthManager()
	if oauthMgr != nil && cfg.Calendar.Provider == "microsoft" {
		ctx := context.Background()
		accounts, _ := oauthMgr.ListAccounts(ctx, "microsoft")
		result["oauth_accounts"] = accounts
	}
	return result
}
