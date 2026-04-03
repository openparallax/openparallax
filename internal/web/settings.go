package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/mcp"
)

func (s *Server) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	cfg := s.engine.Config()
	shieldStatus := s.engine.ShieldStatus()

	resp := map[string]any{
		"agent": map[string]any{
			"name":   cfg.Identity.Name,
			"avatar": cfg.Identity.Avatar,
		},
		"llm": map[string]any{
			"provider":           cfg.LLM.Provider,
			"model":              cfg.LLM.Model,
			"api_key_configured": isKeyConfigured(cfg.LLM.APIKeyEnv),
			"base_url":           cfg.LLM.BaseURL,
		},
		"shield": map[string]any{
			"policy": policyName(cfg.Shield.PolicyFile),
			"evaluator": map[string]any{
				"provider": cfg.Shield.Evaluator.Provider,
				"model":    cfg.Shield.Evaluator.Model,
			},
			"tier2_budget":     cfg.General.DailyBudget,
			"tier2_used_today": shieldStatus["tier2_used"],
		},
		"memory": map[string]any{
			"embedding": map[string]any{
				"provider":           cfg.Memory.Embedding.Provider,
				"model":              cfg.Memory.Embedding.Model,
				"api_key_configured": isKeyConfigured(cfg.Memory.Embedding.APIKeyEnv),
				"base_url":           cfg.Memory.Embedding.BaseURL,
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

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if len(body) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"success":          true,
			"restart_required": false,
			"changed":          []string{},
		})
		return
	}

	cfg := s.engine.Config()
	var changed []string
	var immediate []string
	var needsRestart []string

	// Agent identity — immediate.
	if agentMap, ok := body["agent"].(map[string]any); ok {
		if name, ok := agentMap["name"].(string); ok && name != "" {
			cfg.Identity.Name = name
			s.engine.UpdateIdentity(name, "")
			changed = append(changed, "agent.name")
			immediate = append(immediate, "agent.name")
		}
		if avatar, ok := agentMap["avatar"].(string); ok {
			cfg.Identity.Avatar = avatar
			s.engine.UpdateIdentity("", avatar)
			changed = append(changed, "agent.avatar")
			immediate = append(immediate, "agent.avatar")
		}
	}

	// LLM — restart required for provider/model/key changes.
	if llmMap, ok := body["llm"].(map[string]any); ok {
		if provider, ok := llmMap["provider"].(string); ok && provider != "" {
			cfg.LLM.Provider = provider
			changed = append(changed, "llm.provider")
			needsRestart = append(needsRestart, "llm.provider")
		}
		if model, ok := llmMap["model"].(string); ok && model != "" {
			cfg.LLM.Model = model
			changed = append(changed, "llm.model")
			needsRestart = append(needsRestart, "llm.model")
		}
		if key, ok := llmMap["api_key"].(string); ok && key != "" {
			changed = append(changed, "llm.api_key")
			needsRestart = append(needsRestart, "llm.api_key")
		}
		if keyEnv, ok := llmMap["api_key_env"].(string); ok && keyEnv != "" {
			cfg.LLM.APIKeyEnv = keyEnv
			changed = append(changed, "llm.api_key_env")
			needsRestart = append(needsRestart, "llm.api_key_env")
		}
		if baseURL, ok := llmMap["base_url"].(string); ok {
			cfg.LLM.BaseURL = baseURL
			changed = append(changed, "llm.base_url")
			needsRestart = append(needsRestart, "llm.base_url")
		}
	}

	// Shield — budget is immediate, evaluator is restart.
	if shieldMap, ok := body["shield"].(map[string]any); ok {
		if budget, ok := shieldMap["tier2_budget"].(float64); ok {
			b := int(budget)
			if b < 10 || b > 500 {
				writeError(w, http.StatusBadRequest, "tier2_budget must be 10-500")
				return
			}
			s.engine.UpdateShieldBudget(b)
			changed = append(changed, "shield.tier2_budget")
			immediate = append(immediate, "shield.tier2_budget")
		}
		if evalMap, ok := shieldMap["evaluator"].(map[string]any); ok {
			if provider, ok := evalMap["provider"].(string); ok && provider != "" {
				cfg.Shield.Evaluator.Provider = provider
				changed = append(changed, "shield.evaluator.provider")
				needsRestart = append(needsRestart, "shield.evaluator.provider")
			}
			if model, ok := evalMap["model"].(string); ok && model != "" {
				cfg.Shield.Evaluator.Model = model
				changed = append(changed, "shield.evaluator.model")
				needsRestart = append(needsRestart, "shield.evaluator.model")
			}
		}
	}

	// Memory embedding — restart required.
	if memMap, ok := body["memory"].(map[string]any); ok {
		if embMap, ok := memMap["embedding"].(map[string]any); ok {
			if provider, ok := embMap["provider"].(string); ok {
				cfg.Memory.Embedding.Provider = provider
				changed = append(changed, "memory.embedding.provider")
				needsRestart = append(needsRestart, "memory.embedding.provider")
			}
			if model, ok := embMap["model"].(string); ok {
				cfg.Memory.Embedding.Model = model
				changed = append(changed, "memory.embedding.model")
				needsRestart = append(needsRestart, "memory.embedding.model")
			}
		}
	}

	// Web port — restart required.
	if webMap, ok := body["web"].(map[string]any); ok {
		if port, ok := webMap["port"].(float64); ok {
			p := int(port)
			if p < 1024 || p > 65535 {
				writeError(w, http.StatusBadRequest, "port must be 1024-65535")
				return
			}
			cfg.Web.Port = p
			changed = append(changed, "web.port")
			needsRestart = append(needsRestart, "web.port")
		}
	}

	// Write config to disk.
	if len(changed) > 0 {
		if err := writeConfigToDisk(s.engine.ConfigPath(), cfg); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to write config: "+err.Error())
			return
		}
		s.log.Info("settings_updated", "changed", strings.Join(changed, ","))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":          true,
		"restart_required": len(needsRestart) > 0,
		"changed":          changed,
		"immediate":        immediate,
		"needs_restart":    needsRestart,
	})
}

func (s *Server) handleTestMCP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string            `json:"name"`
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if body.Command == "" {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}

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
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

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
	return os.Getenv(envName) != ""
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

// writeConfigToDisk serializes the config to YAML and writes it.
func writeConfigToDisk(path string, cfg *types.AgentConfig) error {
	var sb strings.Builder
	sb.WriteString("# OpenParallax Configuration\n\n")

	fmt.Fprintf(&sb, "workspace: %s\n\n", cfg.Workspace)

	sb.WriteString("identity:\n")
	fmt.Fprintf(&sb, "  name: %s\n", cfg.Identity.Name)
	if cfg.Identity.Avatar != "" {
		fmt.Fprintf(&sb, "  avatar: %s\n", cfg.Identity.Avatar)
	}
	sb.WriteString("\n")

	sb.WriteString("llm:\n")
	fmt.Fprintf(&sb, "  provider: %s\n", cfg.LLM.Provider)
	fmt.Fprintf(&sb, "  model: %s\n", cfg.LLM.Model)
	if cfg.LLM.APIKeyEnv != "" {
		fmt.Fprintf(&sb, "  api_key_env: %s\n", cfg.LLM.APIKeyEnv)
	}
	if cfg.LLM.BaseURL != "" {
		fmt.Fprintf(&sb, "  base_url: %s\n", cfg.LLM.BaseURL)
	}
	sb.WriteString("\n")

	sb.WriteString("shield:\n")
	if cfg.Shield.Evaluator.Provider != "" {
		sb.WriteString("  evaluator:\n")
		fmt.Fprintf(&sb, "    provider: %s\n", cfg.Shield.Evaluator.Provider)
		fmt.Fprintf(&sb, "    model: %s\n", cfg.Shield.Evaluator.Model)
		if cfg.Shield.Evaluator.APIKeyEnv != "" {
			fmt.Fprintf(&sb, "    api_key_env: %s\n", cfg.Shield.Evaluator.APIKeyEnv)
		}
		if cfg.Shield.Evaluator.BaseURL != "" {
			fmt.Fprintf(&sb, "    base_url: %s\n", cfg.Shield.Evaluator.BaseURL)
		}
	}
	if cfg.Shield.PolicyFile != "" {
		fmt.Fprintf(&sb, "  policy_file: %s\n", cfg.Shield.PolicyFile)
	}
	if cfg.Shield.HeuristicEnabled {
		sb.WriteString("  heuristic_enabled: true\n")
	}
	sb.WriteString("\n")

	if cfg.Memory.Embedding.Provider != "" {
		sb.WriteString("memory:\n")
		sb.WriteString("  embedding:\n")
		fmt.Fprintf(&sb, "    provider: %s\n", cfg.Memory.Embedding.Provider)
		if cfg.Memory.Embedding.Model != "" {
			fmt.Fprintf(&sb, "    model: %s\n", cfg.Memory.Embedding.Model)
		}
		if cfg.Memory.Embedding.APIKeyEnv != "" {
			fmt.Fprintf(&sb, "    api_key_env: %s\n", cfg.Memory.Embedding.APIKeyEnv)
		}
		if cfg.Memory.Embedding.BaseURL != "" {
			fmt.Fprintf(&sb, "    base_url: %s\n", cfg.Memory.Embedding.BaseURL)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("chronicle:\n")
	fmt.Fprintf(&sb, "  max_snapshots: %d\n", cfg.Chronicle.MaxSnapshots)
	fmt.Fprintf(&sb, "  max_age_days: %d\n", cfg.Chronicle.MaxAgeDays)
	sb.WriteString("\n")

	sb.WriteString("web:\n")
	fmt.Fprintf(&sb, "  enabled: %t\n", cfg.Web.Enabled)
	fmt.Fprintf(&sb, "  port: %d\n", cfg.Web.Port)
	fmt.Fprintf(&sb, "  auth: %t\n", cfg.Web.Auth)
	sb.WriteString("\n")

	sb.WriteString("general:\n")
	fmt.Fprintf(&sb, "  fail_closed: %t\n", cfg.General.FailClosed)
	fmt.Fprintf(&sb, "  rate_limit: %d\n", cfg.General.RateLimit)
	fmt.Fprintf(&sb, "  verdict_ttl_seconds: %d\n", cfg.General.VerdictTTLSeconds)
	fmt.Fprintf(&sb, "  daily_budget: %d\n", cfg.General.DailyBudget)

	return os.WriteFile(path, []byte(sb.String()), 0o644)
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
