package commands

import (
	"fmt"
	"strconv"
	"strings"
)

// registerAll registers all built-in commands.
func (r *Registry) registerAll() {
	// Tier 1 — Universal (all channels).
	r.Register(&Command{
		Name:        "help",
		Description: "Show available commands",
		Execute:     cmdHelp,
	})
	r.Register(&Command{
		Name:        "new",
		Description: "Start a new session",
		Execute:     cmdNew,
	})
	r.Register(&Command{
		Name:        "otr",
		Description: "Start an Off-The-Record session",
		Execute:     cmdOTR,
	})
	r.Register(&Command{
		Name:        "status",
		Description: "Show system health",
		Execute:     cmdStatus,
	})
	r.Register(&Command{
		Name:        "usage",
		Description: "Show today's token usage",
		Execute:     cmdUsage,
	})

	// Tier 2 — Session management (messaging channels + CLI only).
	r.Register(&Command{
		Name:        "sessions",
		Description: "List recent sessions",
		Channels:    []Channel{ChannelCLI, ChannelTelegram, ChannelDiscord, ChannelWhatsApp, ChannelSignal, ChannelIMessage},
		Execute:     cmdSessions,
	})
	r.Register(&Command{
		Name:        "switch",
		Description: "Switch to a session by ID prefix",
		Channels:    []Channel{ChannelCLI, ChannelTelegram, ChannelDiscord, ChannelWhatsApp, ChannelSignal, ChannelIMessage},
		Execute:     cmdSwitch,
	})
	r.Register(&Command{
		Name:        "delete",
		Description: "Delete current session",
		Execute:     cmdDelete,
	})
	r.Register(&Command{
		Name:        "title",
		Description: "Rename the current session",
		Execute:     cmdTitle,
	})
	r.Register(&Command{
		Name:        "history",
		Description: "Show recent messages",
		Execute:     cmdHistory,
	})

	// Tier 3 — Admin (CLI + Web only).
	r.Register(&Command{
		Name:        "restart",
		Description: "Restart the engine",
		Channels:    []Channel{ChannelCLI, ChannelWeb},
		Execute:     cmdRestart,
	})
	r.Register(&Command{
		Name:        "clear",
		Description: "Clear the chat view",
		Channels:    []Channel{ChannelCLI, ChannelWeb},
		Execute:     cmdClear,
	})
	r.Register(&Command{
		Name:        "export",
		Description: "Export session as markdown",
		Channels:    []Channel{ChannelCLI, ChannelWeb},
		Execute:     cmdExport,
	})
	r.Register(&Command{
		Name:        "quit",
		Description: "Close session, start new one",
		Channels:    []Channel{ChannelCLI, ChannelWeb},
		Execute:     cmdQuit,
	})

	// Tier 4 — Config (CLI + Web only).
	r.Register(&Command{
		Name:        "config",
		Description: "Show or update configuration",
		Channels:    []Channel{ChannelCLI, ChannelWeb},
		Execute:     cmdConfig,
	})

	r.Register(&Command{
		Name:        "model",
		Description: "Show or switch model role mapping",
		Execute:     cmdModel,
	})

	// Tier 5 — Maintenance (CLI + Web only).
	r.Register(&Command{
		Name:        "logs",
		Description: "Manage logs (flush old entries)",
		Channels:    []Channel{ChannelCLI, ChannelWeb},
		Execute:     cmdLogs,
	})
	r.Register(&Command{
		Name:        "audit",
		Description: "Verify audit trail integrity",
		Channels:    []Channel{ChannelCLI, ChannelWeb},
		Execute:     cmdAudit,
	})
	r.Register(&Command{
		Name:        "doctor",
		Description: "Run system health checks",
		Channels:    []Channel{ChannelCLI, ChannelWeb},
		Execute:     cmdDoctor,
	})
}

// --- Tier 1: Universal ---

func cmdHelp(ctx *Context, _ []string) Result {
	var lines []string
	lines = append(lines, "Available commands:")
	reg := NewRegistry()
	for _, cmd := range reg.List(ctx.Channel) {
		lines = append(lines, fmt.Sprintf("  /%s — %s", cmd.Name, cmd.Description))
	}
	return Result{Text: strings.Join(lines, "\n")}
}

func cmdNew(_ *Context, _ []string) Result {
	return Result{Text: "Starting new session.", Action: ActionNewSession}
}

func cmdOTR(_ *Context, _ []string) Result {
	return Result{Text: "Starting Off-The-Record session.", Action: ActionNewOTR}
}

func cmdStatus(ctx *Context, _ []string) Result {
	if ctx.Engine == nil {
		return Result{Text: "Engine not available."}
	}
	s := ctx.Engine.StatusInfo()

	shield := "Down"
	if s.ShieldActive {
		shield = fmt.Sprintf("Active · %s", s.ShieldTier2)
	}

	sandbox := "Inactive"
	if s.SandboxActive {
		sandbox = s.SandboxMode
		if s.SandboxDetail != "" {
			sandbox += " · " + s.SandboxDetail
		}
	}

	return Result{Text: fmt.Sprintf(
		"System Status\n"+
			"  Agent: %s\n"+
			"  Model: %s\n"+
			"  Shield: %s\n"+
			"  Sandbox: %s\n"+
			"  Sessions: %d · Messages: %d\n"+
			"  Workspace: %s",
		s.AgentName, s.Model, shield, sandbox,
		s.SessionCount, s.MessageCount, s.Workspace)}
}

func cmdUsage(ctx *Context, _ []string) Result {
	if ctx.Engine == nil {
		return Result{Text: "Engine not available."}
	}
	u := ctx.Engine.TokenUsageToday()
	return Result{Text: fmt.Sprintf(
		"Today's usage: %d tokens (%d in / %d out) · %d LLM calls · %d tool calls",
		u.TotalTokens, u.InputTokens, u.OutputTokens, u.LLMCalls, u.ToolCalls)}
}

// --- Tier 2: Session management ---

func cmdSessions(ctx *Context, _ []string) Result {
	if ctx.Engine == nil {
		return Result{Text: "Engine not available."}
	}
	sessions := ctx.Engine.SessionList()
	if len(sessions) == 0 {
		return Result{Text: "No sessions found."}
	}
	var lines []string
	lines = append(lines, "Recent sessions:")
	for _, s := range sessions {
		title := s.Title
		if title == "" {
			title = "Untitled"
		}
		line := fmt.Sprintf("  %s  %s", s.ID[:8], title)
		if s.Age != "" {
			line += fmt.Sprintf("  (%s)", s.Age)
		}
		if s.Mode == "otr" {
			line += "  [OTR]"
		}
		lines = append(lines, line)
	}
	lines = append(lines, "\nUse /switch <id> to switch sessions.")
	return Result{Text: strings.Join(lines, "\n")}
}

func cmdSwitch(ctx *Context, args []string) Result {
	if len(args) == 0 {
		return Result{Text: "Usage: /switch <session-id-prefix>"}
	}
	if ctx.Engine == nil {
		return Result{Text: "Engine not available."}
	}
	prefix := args[0]
	sessions := ctx.Engine.SessionList()
	for _, s := range sessions {
		if strings.HasPrefix(s.ID, prefix) {
			return Result{Text: fmt.Sprintf("Switched to session %s.", s.ID[:8])}
		}
	}
	return Result{Text: fmt.Sprintf("No session matching '%s'.", prefix)}
}

func cmdDelete(ctx *Context, args []string) Result {
	if ctx.Engine == nil {
		return Result{Text: "Engine not available."}
	}
	if ctx.SessionID == "" {
		return Result{Text: "No active session to delete."}
	}
	if len(args) == 0 || args[0] != "confirm" {
		return Result{Text: "Delete this session and all its messages? This cannot be undone.\nType /delete confirm to proceed."}
	}
	if err := ctx.Engine.DeleteSession(ctx.SessionID); err != nil {
		return Result{Text: fmt.Sprintf("Failed to delete session: %s", err)}
	}
	return Result{Text: "Session deleted.", Action: ActionNewSession}
}

func cmdTitle(ctx *Context, args []string) Result {
	if ctx.Engine == nil {
		return Result{Text: "Engine not available."}
	}
	if ctx.SessionID == "" {
		return Result{Text: "No active session."}
	}
	if len(args) == 0 {
		return Result{Text: "Usage: /title <new title>"}
	}
	title := strings.Join(args, " ")
	if err := ctx.Engine.UpdateSessionTitle(ctx.SessionID, title); err != nil {
		return Result{Text: fmt.Sprintf("Failed to update title: %s", err)}
	}
	return Result{Text: fmt.Sprintf("Session renamed to '%s'.", title)}
}

func cmdHistory(ctx *Context, args []string) Result {
	if ctx.Engine == nil {
		return Result{Text: "Engine not available."}
	}
	if ctx.SessionID == "" {
		return Result{Text: "No active session."}
	}
	n := 10
	if len(args) > 0 {
		if parsed, err := strconv.Atoi(args[0]); err == nil && parsed > 0 {
			n = parsed
		}
	}
	msgs := ctx.Engine.GetMessages(ctx.SessionID)
	if len(msgs) == 0 {
		return Result{Text: "No messages in this session."}
	}
	start := len(msgs) - n
	if start < 0 {
		start = 0
	}
	var lines []string
	for _, m := range msgs[start:] {
		who := m.Role
		preview := m.Content
		if len(preview) > 120 {
			preview = preview[:120] + "..."
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		lines = append(lines, fmt.Sprintf("  [%s] %s: %s", m.Time, who, preview))
	}
	return Result{Text: fmt.Sprintf("Last %d messages:\n%s", len(lines), strings.Join(lines, "\n"))}
}

// --- Tier 3: Admin ---

func cmdRestart(_ *Context, _ []string) Result {
	return Result{Text: "Restarting engine...", Action: ActionRestart}
}

func cmdClear(_ *Context, _ []string) Result {
	return Result{Text: "Chat cleared.", Action: ActionClear}
}

func cmdExport(ctx *Context, _ []string) Result {
	if ctx.Engine == nil {
		return Result{Text: "Engine not available."}
	}
	if ctx.SessionID == "" {
		return Result{Text: "No active session to export."}
	}
	msgs := ctx.Engine.GetMessages(ctx.SessionID)
	if len(msgs) == 0 {
		return Result{Text: "No messages to export."}
	}

	var md strings.Builder
	md.WriteString("# Session Export\n\n---\n\n")
	for _, m := range msgs {
		if m.Role == "system" {
			continue
		}
		who := "You"
		if m.Role == "assistant" {
			who = "Assistant"
		}
		fmt.Fprintf(&md, "%s (%s):\n%s\n\n---\n\n", who, m.Time, m.Content)
	}
	return Result{Text: md.String()}
}

func cmdQuit(_ *Context, _ []string) Result {
	return Result{Text: "Session closed.", Action: ActionQuit}
}

// --- Tier 4: Config ---

func cmdConfig(ctx *Context, args []string) Result {
	if ctx.Engine == nil {
		return Result{Text: "Engine not available."}
	}
	if len(args) == 0 {
		return Result{Text: ctx.Engine.ConfigSummary()}
	}
	if args[0] == "set" && len(args) >= 3 {
		key := args[1]
		value := strings.Join(args[2:], " ")
		restart, err := ctx.Engine.ConfigSet(key, value)
		if err != nil {
			return Result{Text: fmt.Sprintf("Failed to set %s: %s", key, err)}
		}
		msg := fmt.Sprintf("Set %s = %s", key, value)
		if restart {
			msg += " (restart required: /restart)"
		}
		return Result{Text: msg}
	}
	return Result{Text: "Usage: /config or /config set <key> <value>"}
}

func cmdModel(ctx *Context, args []string) Result {
	if ctx.Engine == nil {
		return Result{Text: "Engine not available."}
	}

	// /model — show current mapping.
	if len(args) == 0 {
		roles := ctx.Engine.ModelRoles()
		models := ctx.Engine.ModelList()
		var lines []string
		lines = append(lines, "Model roles:")
		for _, role := range []string{"chat", "shield", "embedding", "sub_agent", "image", "video"} {
			name := roles[role]
			if name == "" {
				name = "(not set)"
			}
			lines = append(lines, fmt.Sprintf("  %-12s %s", role+":", name))
		}
		lines = append(lines, fmt.Sprintf("\nAvailable models: %s", strings.Join(models, ", ")))
		lines = append(lines, "\nUsage: /model <role> <model-name>")
		return Result{Text: strings.Join(lines, "\n")}
	}

	// /model <role> <model-name> — switch role.
	if len(args) >= 2 {
		role := args[0]
		modelName := args[1]
		if err := ctx.Engine.SetModelRole(role, modelName); err != nil {
			return Result{Text: fmt.Sprintf("Failed: %s", err)}
		}
		return Result{Text: fmt.Sprintf("Switched %s to %s.", role, modelName)}
	}

	return Result{Text: "Usage: /model or /model <role> <model-name>"}
}

// --- Tier 5: Maintenance ---

func cmdLogs(ctx *Context, args []string) Result {
	if ctx.Engine == nil {
		return Result{Text: "Engine not available."}
	}
	if len(args) == 0 {
		return Result{Text: "Usage: /logs flush [days]\nDeletes log entries older than N days (default 30)."}
	}
	if args[0] == "flush" {
		days := 30
		if len(args) > 1 {
			if d, err := strconv.Atoi(args[1]); err == nil && d > 0 {
				days = d
			}
		}
		removed, err := ctx.Engine.FlushLogs(days)
		if err != nil {
			return Result{Text: fmt.Sprintf("Failed to flush logs: %s", err)}
		}
		return Result{Text: fmt.Sprintf("Flushed %d log entries older than %d days.", removed, days)}
	}
	return Result{Text: "Usage: /logs flush [days]"}
}

func cmdAudit(ctx *Context, args []string) Result {
	if ctx.Engine == nil {
		return Result{Text: "Engine not available."}
	}
	if len(args) > 0 && args[0] == "verify" {
		valid, breakAt, total := ctx.Engine.AuditVerify()
		if valid {
			return Result{Text: fmt.Sprintf("Audit chain valid. %d entries verified.", total)}
		}
		return Result{Text: fmt.Sprintf("Audit chain BROKEN at entry #%d of %d.", breakAt, total)}
	}
	return Result{Text: "Usage: /audit verify"}
}

func cmdDoctor(ctx *Context, _ []string) Result {
	if ctx.Engine == nil {
		return Result{Text: "Engine not available."}
	}
	checks := ctx.Engine.DoctorChecks()
	if len(checks) == 0 {
		return Result{Text: "No health checks available."}
	}
	var lines []string
	passed := 0
	for _, c := range checks {
		icon := "FAIL"
		if c.Passed {
			icon = "OK"
			passed++
		}
		lines = append(lines, fmt.Sprintf("  [%s] %s — %s", icon, c.Name, c.Detail))
	}
	lines = append(lines, fmt.Sprintf("\n%d/%d checks passed.", passed, len(checks)))
	return Result{Text: strings.Join(lines, "\n")}
}
