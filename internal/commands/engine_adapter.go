package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/engine"
	"github.com/openparallax/openparallax/internal/types"
	"gopkg.in/yaml.v3"
)

// EngineAdapter implements EngineAccess using a real Engine instance.
type EngineAdapter struct {
	Engine *engine.Engine
}

// StatusInfo returns system health info.
func (a *EngineAdapter) StatusInfo() StatusInfo {
	cfg := a.Engine.Config()
	ss := a.Engine.ShieldStatus()
	sb := a.Engine.SandboxStatus()
	sc, _ := a.Engine.DB().SessionCount()

	agentName := cfg.Identity.Name
	if agentName == "" {
		agentName = "Atlas"
	}

	shieldActive, _ := ss["active"].(bool)
	tier2 := fmt.Sprintf("Tier 2: %v/%v", ss["tier2_used"], ss["tier2_budget"])

	sandboxActive, _ := sb["active"].(bool)
	sandboxMode, _ := sb["mode"].(string)
	var sandboxDetail string
	if fs, ok := sb["filesystem"].(bool); ok && fs {
		sandboxDetail = "FS"
	}
	if net, ok := sb["network"].(bool); ok && net {
		if sandboxDetail != "" {
			sandboxDetail += " · "
		}
		sandboxDetail += "Net"
	}

	return StatusInfo{
		AgentName:     agentName,
		Model:         a.Engine.LLMModel(),
		Workspace:     cfg.Workspace,
		ShieldActive:  shieldActive,
		ShieldTier2:   tier2,
		SandboxActive: sandboxActive,
		SandboxMode:   sandboxMode,
		SandboxDetail: sandboxDetail,
		SessionCount:  sc,
	}
}

// SessionList returns recent sessions.
func (a *EngineAdapter) SessionList() []SessionInfo {
	sessions, err := a.Engine.DB().ListSessions()
	if err != nil {
		return nil
	}
	var result []SessionInfo
	for _, s := range sessions {
		age := ""
		if s.LastMsgAt != "" {
			if t, err := time.Parse(time.RFC3339, s.LastMsgAt); err == nil {
				age = formatAge(time.Since(t))
			}
		} else if s.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339, s.CreatedAt); err == nil {
				age = formatAge(time.Since(t))
			}
		}
		result = append(result, SessionInfo{
			ID:      s.ID,
			Title:   s.Title,
			Mode:    string(s.Mode),
			Preview: s.Preview,
			Age:     age,
		})
	}
	return result
}

// DeleteSession deletes a session by ID.
func (a *EngineAdapter) DeleteSession(id string) error {
	return a.Engine.DB().DeleteSession(id)
}

// UpdateSessionTitle changes a session's title.
func (a *EngineAdapter) UpdateSessionTitle(id, title string) error {
	return a.Engine.DB().UpdateSessionTitle(id, title)
}

// GetMessages returns messages for a session.
func (a *EngineAdapter) GetMessages(sessionID string) []MessageInfo {
	msgs, err := a.Engine.DB().GetMessages(sessionID)
	if err != nil {
		return nil
	}
	var result []MessageInfo
	for _, m := range msgs {
		result = append(result, MessageInfo{
			Role:    m.Role,
			Content: m.Content,
			Time:    m.Timestamp.Format("15:04"),
		})
	}
	return result
}

// TokenUsageToday returns today's token usage summary.
func (a *EngineAdapter) TokenUsageToday() UsageInfo {
	today := time.Now().Format("2006-01-02")
	summary := a.Engine.DB().GetMetricsSummary(today, today)
	return UsageInfo{
		InputTokens:  summary.TokenUsage["input"],
		OutputTokens: summary.TokenUsage["output"],
		TotalTokens:  summary.TokenUsage["total"],
		LLMCalls:     summary.TokenUsage["llm_calls"],
		ToolCalls:    summary.DailyMetrics["tool_calls"],
	}
}

// ConfigSummary returns the current config with secrets masked.
func (a *EngineAdapter) ConfigSummary() string {
	cfg := a.Engine.Config()
	chat, _ := cfg.ChatModel()
	shieldModel, _ := cfg.ShieldModel()
	subModel, _ := cfg.SubAgentModel()
	var lines []string
	lines = append(lines, "Current configuration:")
	lines = append(lines, fmt.Sprintf("  chat.provider: %s", chat.Provider))
	lines = append(lines, fmt.Sprintf("  chat.model: %s", chat.Model))
	lines = append(lines, fmt.Sprintf("  chat.api_key_env: %s", chat.APIKeyEnv))
	lines = append(lines, fmt.Sprintf("  identity.name: %s", cfg.Identity.Name))
	lines = append(lines, fmt.Sprintf("  identity.avatar: %s", cfg.Identity.Avatar))
	lines = append(lines, fmt.Sprintf("  web.port: %d", cfg.Web.Port))
	lines = append(lines, fmt.Sprintf("  shield.provider: %s", shieldModel.Provider))
	lines = append(lines, fmt.Sprintf("  shield.model: %s", shieldModel.Model))
	lines = append(lines, fmt.Sprintf("  sub_agent.model: %s", subModel.Model))
	lines = append(lines, fmt.Sprintf("  agents.max_rounds: %d", cfg.Agents.MaxRounds))
	if len(cfg.Tools.DisabledGroups) > 0 {
		lines = append(lines, fmt.Sprintf("  tools.disabled_groups: %s", strings.Join(cfg.Tools.DisabledGroups, ", ")))
	}
	return strings.Join(lines, "\n")
}

// readOnlyKeys are config keys that cannot be changed via /config set.
// They must be edited directly in config.yaml for safety.
var readOnlyKeys = map[string]bool{
	"general.fail_closed":         true,
	"general.rate_limit":          true,
	"general.daily_budget":        true,
	"general.verdict_ttl_seconds": true,
	"general.output_sanitization": true,
	"agents.max_tool_rounds":      true,
	"agents.context_window":       true,
	"agents.compaction_threshold": true,
	"agents.max_response_tokens":  true,
	"shield.policy_file":          true,
	"shield.onnx_threshold":       true,
	"shield.heuristic_enabled":    true,
	"web.host":                    true,
	"web.port":                    true,
	"web.password_hash":           true,
}

// ConfigSet updates a config value through the canonical SettableKeys
// registry, persists it via config.Save, and applies live changes to
// the running engine where applicable. Security-sensitive and pipeline
// parameters remain read-only — they must be edited directly in
// config.yaml. The bool return reports whether the change requires a
// restart to take effect.
func (a *EngineAdapter) ConfigSet(key, value string) (bool, error) {
	if readOnlyKeys[key] {
		return false, fmt.Errorf("%s is read-only — edit config.yaml directly", key)
	}

	settable, ok := config.SettableKeys[key]
	if !ok {
		return false, fmt.Errorf("unknown setting %q — see /config keys for the list", key)
	}

	cfg := a.Engine.Config()

	// Snapshot the current value so we can roll back if Save fails.
	snapshot, _ := yamlClone(cfg)

	if err := settable.Setter(cfg, value); err != nil {
		return false, err
	}

	if err := config.Save(a.Engine.ConfigPath(), cfg); err != nil {
		// Roll back the in-memory mutation.
		if snapshot != nil {
			*cfg = *snapshot
		}
		return false, fmt.Errorf("save failed: %w", err)
	}

	switch key {
	case "identity.name":
		a.Engine.UpdateIdentity(cfg.Identity.Name, "")
	case "identity.avatar":
		a.Engine.UpdateIdentity("", cfg.Identity.Avatar)
	}

	return settable.RequiresRestart, nil
}

// yamlClone produces a deep copy of cfg via YAML round-trip. Used as a
// rollback snapshot before a mutation that might fail validation.
func yamlClone(cfg *types.AgentConfig) (*types.AgentConfig, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var clone types.AgentConfig
	if err := yaml.Unmarshal(data, &clone); err != nil {
		return nil, err
	}
	return &clone, nil
}

// FlushLogs deletes log entries older than the given number of days.
func (a *EngineAdapter) FlushLogs(days int) (int, error) {
	logPath := a.Engine.LogPath()
	return flushOldLines(logPath, days)
}

// AuditVerify checks the audit hash chain integrity.
func (a *EngineAdapter) AuditVerify() (bool, int, int) {
	auditPath := a.Engine.AuditPath()
	f, err := os.Open(auditPath)
	if err != nil {
		return true, 0, 0
	}
	defer func() { _ = f.Close() }()

	var prevHash string
	total := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		total++
		ph, _ := entry["previous_hash"].(string)
		if total > 1 && ph != prevHash {
			return false, total, total
		}
		prevHash, _ = entry["hash"].(string)
	}
	return true, 0, total
}

// DoctorChecks runs health checks.
func (a *EngineAdapter) DoctorChecks() []DoctorCheck {
	var checks []DoctorCheck
	cfg := a.Engine.Config()

	// Config file.
	if _, err := os.Stat(a.Engine.ConfigPath()); err == nil {
		checks = append(checks, DoctorCheck{Name: "Config", Passed: true, Detail: "config.yaml found"})
	} else {
		checks = append(checks, DoctorCheck{Name: "Config", Passed: false, Detail: "config.yaml missing"})
	}

	// LLM provider.
	model := a.Engine.LLMModel()
	if model != "" {
		checks = append(checks, DoctorCheck{Name: "LLM", Passed: true, Detail: model})
	} else {
		checks = append(checks, DoctorCheck{Name: "LLM", Passed: false, Detail: "no model configured"})
	}

	// Shield.
	ss := a.Engine.ShieldStatus()
	if active, ok := ss["active"].(bool); ok && active {
		checks = append(checks, DoctorCheck{Name: "Shield", Passed: true, Detail: "active"})
	} else {
		checks = append(checks, DoctorCheck{Name: "Shield", Passed: false, Detail: "inactive"})
	}

	// Sandbox.
	sb := a.Engine.SandboxStatus()
	if active, ok := sb["active"].(bool); ok && active {
		mode, _ := sb["mode"].(string)
		checks = append(checks, DoctorCheck{Name: "Sandbox", Passed: true, Detail: mode})
	} else {
		checks = append(checks, DoctorCheck{Name: "Sandbox", Passed: false, Detail: "not available"})
	}

	// Database.
	sc, err := a.Engine.DB().SessionCount()
	if err == nil {
		checks = append(checks, DoctorCheck{Name: "Database", Passed: true, Detail: fmt.Sprintf("%d sessions", sc)})
	} else {
		checks = append(checks, DoctorCheck{Name: "Database", Passed: false, Detail: err.Error()})
	}

	// Workspace.
	if _, err := os.Stat(cfg.Workspace); err == nil {
		checks = append(checks, DoctorCheck{Name: "Workspace", Passed: true, Detail: cfg.Workspace})
	} else {
		checks = append(checks, DoctorCheck{Name: "Workspace", Passed: false, Detail: "directory missing"})
	}

	return checks
}

// flushOldLines removes lines from a JSONL file where the timestamp is older than days.
func flushOldLines(path string, days int) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, nil
	}
	defer func() { _ = f.Close() }()

	cutoff := time.Now().AddDate(0, 0, -days)
	var kept []string
	removed := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			kept = append(kept, line)
			continue
		}
		ts, _ := entry["timestamp"].(string)
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil && t.Before(cutoff) {
			removed++
			continue
		}
		kept = append(kept, line)
	}

	if removed == 0 {
		return 0, nil
	}

	return removed, os.WriteFile(path, []byte(strings.Join(kept, "\n")+"\n"), 0o644)
}

// ModelList returns all registered model names.
func (a *EngineAdapter) ModelList() []string {
	reg := a.Engine.ModelRegistry()
	if reg == nil {
		return nil
	}
	return reg.ModelNames()
}

// ModelRoles returns the current role → model name mapping.
func (a *EngineAdapter) ModelRoles() map[string]string {
	reg := a.Engine.ModelRegistry()
	if reg == nil {
		return nil
	}
	return reg.RoleMapping()
}

// SetModelRole switches a role to a different model. The change is
// applied to the live registry and persisted to config.yaml so it
// survives a restart. On Save failure both layers roll back.
func (a *EngineAdapter) SetModelRole(role, modelName string) error {
	reg := a.Engine.ModelRegistry()
	if reg == nil {
		return fmt.Errorf("model registry not available")
	}

	cfg := a.Engine.Config()
	if _, ok := cfg.ModelByName(modelName); !ok {
		return fmt.Errorf("model %q is not in the model pool", modelName)
	}

	previous := ""
	switch role {
	case "chat":
		previous = cfg.Roles.Chat
		cfg.Roles.Chat = modelName
	case "shield":
		previous = cfg.Roles.Shield
		cfg.Roles.Shield = modelName
	case "embedding":
		previous = cfg.Roles.Embedding
		cfg.Roles.Embedding = modelName
	case "sub_agent":
		previous = cfg.Roles.SubAgent
		cfg.Roles.SubAgent = modelName
	default:
		return fmt.Errorf("unknown role %q", role)
	}

	if err := reg.SetRole(role, modelName); err != nil {
		return err
	}

	if err := config.Save(a.Engine.ConfigPath(), cfg); err != nil {
		// Roll back both layers.
		_ = reg.SetRole(role, previous)
		switch role {
		case "chat":
			cfg.Roles.Chat = previous
		case "shield":
			cfg.Roles.Shield = previous
		case "embedding":
			cfg.Roles.Embedding = previous
		case "sub_agent":
			cfg.Roles.SubAgent = previous
		}
		return fmt.Errorf("save failed: %w", err)
	}
	return nil
}

func formatAge(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}
