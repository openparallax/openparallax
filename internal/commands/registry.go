// Package commands provides a centralized slash command registry shared by all
// input channels (Web UI, CLI, Telegram, Discord, WhatsApp, Signal, iMessage).
// Commands are tiered by channel availability and executed through a uniform interface.
package commands

import (
	"fmt"
	"sort"
	"strings"
)

// Channel identifies an input channel type.
type Channel string

const (
	ChannelWeb      Channel = "web"
	ChannelCLI      Channel = "cli"
	ChannelTelegram Channel = "telegram"
	ChannelDiscord  Channel = "discord"
	ChannelWhatsApp Channel = "whatsapp"
	ChannelSignal   Channel = "signal"
	ChannelIMessage Channel = "imessage"
)

// Result is the response from executing a command.
type Result struct {
	// Text is the response to show the user.
	Text string

	// Action signals a side-effect the channel adapter must handle.
	Action Action
}

// Action signals a side-effect that the channel adapter must handle after
// command execution. Most commands return ActionNone.
type Action int

const (
	ActionNone       Action = iota
	ActionNewSession        // Create a new session (normal mode)
	ActionNewOTR            // Create a new OTR session
	ActionRestart           // Restart the engine
	ActionQuit              // Close session + start new one
	ActionClear             // Clear local chat view
)

// Command is a registered slash command.
type Command struct {
	// Name is the command name without the leading slash (e.g., "help").
	Name string

	// Description is a one-line summary shown in /help.
	Description string

	// Channels lists which channels this command is available on.
	// Empty means all channels.
	Channels []Channel

	// Execute runs the command and returns a text response.
	Execute func(ctx *Context, args []string) Result
}

// Context carries the state needed by command handlers.
type Context struct {
	Channel   Channel
	SessionID string
	Engine    EngineAccess
}

// EngineAccess is the interface commands use to interact with the engine.
// Keeps the commands package decoupled from the engine package.
type EngineAccess interface {
	// Status returns system health info.
	StatusInfo() StatusInfo

	// SessionList returns recent sessions.
	SessionList() []SessionInfo

	// DeleteSession deletes a session by ID.
	DeleteSession(id string) error

	// UpdateSessionTitle changes a session's title.
	UpdateSessionTitle(id, title string) error

	// GetMessages returns messages for a session.
	GetMessages(sessionID string) []MessageInfo

	// TokenUsageToday returns today's token usage summary.
	TokenUsageToday() UsageInfo

	// ConfigSummary returns the current config (secrets masked).
	ConfigSummary() string

	// ConfigSet updates a config value. Returns whether restart is required.
	ConfigSet(key, value string) (restartRequired bool, err error)

	// FlushLogs deletes log entries older than the given number of days.
	FlushLogs(days int) (int, error)

	// AuditVerify checks the audit hash chain integrity.
	AuditVerify() (valid bool, breakAt int, total int)

	// DoctorChecks runs health checks and returns results.
	DoctorChecks() []DoctorCheck

	// ModelList returns all registered model names.
	ModelList() []string

	// ModelRoles returns the current role → model name mapping.
	ModelRoles() map[string]string

	// SetModelRole switches a role to a different model. Returns error if model not found.
	SetModelRole(role, modelName string) error
}

// StatusInfo holds system status for the /status command.
type StatusInfo struct {
	AgentName     string
	Model         string
	Workspace     string
	ShieldActive  bool
	ShieldTier2   string
	SandboxActive bool
	SandboxMode   string
	SandboxDetail string
	SessionCount  int
	MessageCount  int
}

// SessionInfo holds session metadata for the /sessions command.
type SessionInfo struct {
	ID      string
	Title   string
	Mode    string
	Preview string
	Age     string
}

// MessageInfo holds a message for the /history command.
type MessageInfo struct {
	Role    string
	Content string
	Time    string
}

// UsageInfo holds token usage for the /usage command.
type UsageInfo struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	LLMCalls     int
	ToolCalls    int
}

// DoctorCheck holds a single health check result.
type DoctorCheck struct {
	Name   string
	Passed bool
	Detail string
}

// Registry holds all registered commands.
type Registry struct {
	commands map[string]*Command
	order    []string
}

// NewRegistry creates a command registry with all built-in commands.
func NewRegistry() *Registry {
	r := &Registry{commands: make(map[string]*Command)}
	r.registerAll()
	return r
}

// Register adds a command to the registry.
func (r *Registry) Register(cmd *Command) {
	r.commands[cmd.Name] = cmd
	r.order = append(r.order, cmd.Name)
}

// Execute parses and runs a slash command. Returns the result and whether
// the command was found.
func (r *Registry) Execute(input string, ctx *Context) (Result, bool) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return Result{}, false
	}

	parts := strings.Fields(input)
	name := strings.TrimPrefix(parts[0], "/")
	name = strings.ToLower(name)

	// Strip bot username suffix (e.g., /new@botname → new).
	if idx := strings.Index(name, "@"); idx > 0 {
		name = name[:idx]
	}

	cmd, ok := r.commands[name]
	if !ok {
		return Result{Text: fmt.Sprintf("Unknown command: /%s\nType /help for available commands.", name)}, true
	}

	if !cmd.availableOn(ctx.Channel) {
		return Result{Text: fmt.Sprintf("/%s is not available on this channel.", name)}, true
	}

	args := parts[1:]
	return cmd.Execute(ctx, args), true
}

// List returns all commands available on the given channel.
func (r *Registry) List(ch Channel) []*Command {
	var result []*Command
	for _, name := range r.order {
		cmd := r.commands[name]
		if cmd.availableOn(ch) {
			result = append(result, cmd)
		}
	}
	return result
}

// Names returns all command names available on the given channel, sorted.
func (r *Registry) Names(ch Channel) []string {
	var names []string
	for _, name := range r.order {
		if r.commands[name].availableOn(ch) {
			names = append(names, "/"+name)
		}
	}
	sort.Strings(names)
	return names
}

func (c *Command) availableOn(ch Channel) bool {
	if len(c.Channels) == 0 {
		return true
	}
	for _, allowed := range c.Channels {
		if allowed == ch {
			return true
		}
	}
	return false
}
