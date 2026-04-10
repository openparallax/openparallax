package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/internal/engine/executors"
	"github.com/openparallax/openparallax/llm"
	"github.com/openparallax/openparallax/sandbox"
)

// SubAgentStatus represents the lifecycle state of a sub-agent.
type SubAgentStatus string

const (
	// StatusSpawning means the sub-agent process is starting.
	StatusSpawning SubAgentStatus = "spawning"
	// StatusWorking means the sub-agent is actively running its LLM loop.
	StatusWorking SubAgentStatus = "working"
	// StatusCompleted means the sub-agent finished its task.
	StatusCompleted SubAgentStatus = "completed"
	// StatusFailed means the sub-agent encountered an error.
	StatusFailed SubAgentStatus = "failed"
	// StatusTimedOut means the sub-agent exceeded its timeout.
	StatusTimedOut SubAgentStatus = "timed_out"
	// StatusCancelled means the sub-agent was terminated by the user or main agent.
	StatusCancelled SubAgentStatus = "cancelled"
)

// SubAgent represents an ephemeral worker sub-agent.
type SubAgent struct {
	// Name is the randomly assigned name from the pool.
	Name string `json:"name"`
	// Task is the task description given to the sub-agent.
	Task string `json:"task"`
	// Status is the current lifecycle state.
	Status SubAgentStatus `json:"status"`
	// Model is the LLM model the sub-agent is using.
	Model string `json:"model"`
	// ToolGroups are the tool groups assigned to the sub-agent.
	ToolGroups []string `json:"tool_groups,omitempty"`
	// CreatedAt is when the sub-agent was spawned.
	CreatedAt time.Time `json:"created_at"`
	// CompletedAt is when the sub-agent finished (nil if still running).
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// Result is the final text response from the sub-agent.
	Result string `json:"result,omitempty"`
	// Error is the error message if the sub-agent failed.
	Error string `json:"error,omitempty"`
	// LLMCallCount is the number of LLM calls the sub-agent has made.
	LLMCallCount int `json:"llm_call_count"`
	// ToolCallCount is the number of tool calls executed.
	ToolCallCount int `json:"tool_call_count"`

	// Internal fields (not serialized).
	pid       int
	authToken string
	cancel    context.CancelFunc
	resultCh  chan struct{}
	messageCh chan string
	cmd       *exec.Cmd
	tools     []llm.ToolDefinition
	sessionID string
	provider  string
	apiKeyEnv string
	baseURL   string
}

// SubAgentRequest contains the parameters for creating a sub-agent.
type SubAgentRequest struct {
	// Task is the task description for the sub-agent.
	Task string
	// ToolGroups are the tool groups to assign. Empty means all tools.
	ToolGroups []string
	// Model overrides the default sub-agent model.
	Model string
	// TimeoutSeconds is how long the sub-agent can run (default 300).
	TimeoutSeconds int
	// SessionID is the parent session ID for audit context.
	SessionID string
	// IsOTR indicates whether the parent session is in OTR mode.
	IsOTR bool
}

// SubAgentManager manages the lifecycle of ephemeral sub-agent processes.
type SubAgentManager struct {
	mu            sync.RWMutex
	agents        map[string]*SubAgent
	usedNames     map[string]bool
	maxConcurrent int
	workspace     string
	grpcAddr      string
	engine        *Engine
	broadcaster   func(*PipelineEvent)
}

// NewSubAgentManager creates a new sub-agent manager.
func NewSubAgentManager(engine *Engine, grpcAddr string, maxConcurrent int) *SubAgentManager {
	return &SubAgentManager{
		agents:        make(map[string]*SubAgent),
		usedNames:     make(map[string]bool),
		maxConcurrent: maxConcurrent,
		workspace:     engine.cfg.Workspace,
		grpcAddr:      grpcAddr,
		engine:        engine,
	}
}

// SetEventBroadcaster sets the callback for broadcasting sub-agent events
// to WebSocket clients and gRPC subscribers.
func (m *SubAgentManager) SetEventBroadcaster(fn func(*PipelineEvent)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcaster = fn
}

// Create spawns a new sub-agent process.
func (m *SubAgentManager) Create(req SubAgentRequest) (string, error) {
	m.mu.Lock()

	// Check concurrency limit.
	activeCount := 0
	for _, a := range m.agents {
		if a.Status == StatusSpawning || a.Status == StatusWorking {
			activeCount++
		}
	}
	if activeCount >= m.maxConcurrent {
		m.mu.Unlock()
		m.engine.db.IncrementDailyMetric("subagent_concurrency_cap_hits", 1)
		return "", fmt.Errorf("maximum %d concurrent sub-agents reached — wait for one to complete or cancel an active one", m.maxConcurrent)
	}

	name := pickName(m.usedNames)
	m.usedNames[name] = true

	token, err := crypto.RandomHex(16)
	if err != nil {
		m.mu.Unlock()
		return "", fmt.Errorf("generate auth token: %w", err)
	}

	defaultTimeout := 900 * time.Second
	if m.engine.cfg.Agents.SubAgentTimeoutSeconds > 0 {
		defaultTimeout = time.Duration(m.engine.cfg.Agents.SubAgentTimeoutSeconds) * time.Second
	}
	timeout := defaultTimeout
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
	}

	// Resolve model. Priority: explicit request → roles.sub_agent → cheapest → chat.
	model := req.Model
	if model == "" {
		if subModel, ok := m.engine.cfg.SubAgentModel(); ok {
			model = subModel.Model
		}
	}
	if model == "" {
		model = m.engine.llm.CheapestModel()
	}
	if model == "" {
		model = m.engine.llm.Model()
	}

	// Resolve tools.
	var toolDefs []llm.ToolDefinition
	if len(req.ToolGroups) > 0 {
		toolDefs, _ = m.engine.executors.Groups.ResolveGroups(req.ToolGroups, req.IsOTR)
	} else {
		// All groups minus excluded ones.
		allGroups := m.engine.executors.Groups.AvailableGroups()
		var names []string
		for _, g := range allGroups {
			if isExcludedSubAgentGroup(g.Name) {
				continue
			}
			names = append(names, g.Name)
		}
		toolDefs, _ = m.engine.executors.Groups.ResolveGroups(names, req.IsOTR)
	}

	// Strip sub-agent tools from the resolved set (prevent recursion).
	toolDefs = filterSubAgentTools(toolDefs)

	ctx, cancel := context.WithTimeout(m.engine.backgroundCtx, timeout)

	// Resolve sub-agent provider credentials from the role mapping.
	// Falls back to the chat model when no sub_agent role is set.
	subModel, _ := m.engine.cfg.SubAgentModel()

	sa := &SubAgent{
		Name:       name,
		Task:       req.Task,
		Status:     StatusSpawning,
		Model:      model,
		ToolGroups: req.ToolGroups,
		CreatedAt:  time.Now(),
		authToken:  token,
		cancel:     cancel,
		resultCh:   make(chan struct{}),
		messageCh:  make(chan string, 1),
		tools:      toolDefs,
		sessionID:  req.SessionID,
		provider:   m.engine.llm.Name(),
		apiKeyEnv:  subModel.APIKeyEnv,
		baseURL:    subModel.BaseURL,
	}
	m.agents[name] = sa
	m.mu.Unlock()

	// Spawn process.
	if err := m.spawnProcess(ctx, sa); err != nil {
		m.mu.Lock()
		sa.Status = StatusFailed
		sa.Error = err.Error()
		now := time.Now()
		sa.CompletedAt = &now
		close(sa.resultCh)
		m.mu.Unlock()
		return name, fmt.Errorf("spawn sub-agent: %w", err)
	}

	m.broadcastEvent(&PipelineEvent{
		Type: EventSubAgentSpawned,
		SubAgentSpawned: &SubAgentSpawnedEvent{
			Name: name, Task: req.Task, ToolGroups: req.ToolGroups,
		},
	})
	WriteAgentsMD(m.workspace, m.listLocked())

	// Start timeout/monitoring goroutine.
	go m.monitorSubAgent(ctx, sa)

	return name, nil
}

// spawnProcess starts the sub-agent OS process.
func (m *SubAgentManager) spawnProcess(ctx context.Context, sa *SubAgent) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find own executable: %w", err)
	}

	cmd := exec.CommandContext(ctx, executable, "internal-sub-agent",
		"--grpc", m.grpcAddr,
		"--workspace", m.workspace)
	cmd.Env = append(os.Environ(), "OPENPARALLAX_SUB_AGENT_TOKEN="+sa.authToken)

	// Redirect stdout/stderr to devnull (sub-agent has no TUI).
	devNull, openErr := os.Open(os.DevNull)
	if openErr == nil {
		cmd.Stdout = devNull
		cmd.Stderr = devNull
	}

	// Apply sandbox wrapping.
	sb := sandbox.New()
	if sb.Available() {
		if wrapErr := sb.WrapCommand(cmd, sandbox.Config{
			AllowedReadPaths:  []string{executable},
			AllowedTCPConnect: []string{m.grpcAddr},
			AllowProcessSpawn: false,
		}); wrapErr != nil {
			m.engine.Log().Warn("sub_agent_sandbox_wrap_failed", "error", wrapErr)
		}
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	sa.cmd = cmd
	sa.pid = cmd.Process.Pid
	return nil
}

// monitorSubAgent watches the sub-agent process and handles timeout/crash.
func (m *SubAgentManager) monitorSubAgent(ctx context.Context, sa *SubAgent) {
	defer sa.cancel()

	if sa.cmd == nil {
		return
	}

	// Wait for process exit.
	exitCh := make(chan error, 1)
	go func() {
		exitCh <- sa.cmd.Wait()
	}()

	select {
	case err := <-exitCh:
		m.mu.Lock()
		if sa.Status == StatusWorking || sa.Status == StatusSpawning {
			if err != nil {
				sa.Status = StatusFailed
				sa.Error = fmt.Sprintf("process exited unexpectedly: %v", err)
			} else if sa.Status != StatusCompleted {
				sa.Status = StatusFailed
				sa.Error = "process exited without reporting completion"
			}
			now := time.Now()
			sa.CompletedAt = &now
			close(sa.resultCh)
		}
		m.mu.Unlock()

	case <-ctx.Done():
		m.mu.Lock()
		if sa.Status == StatusWorking || sa.Status == StatusSpawning {
			sa.Status = StatusTimedOut
			sa.Error = "sub-agent timed out"
			now := time.Now()
			sa.CompletedAt = &now
			close(sa.resultCh)
			m.engine.db.IncrementDailyMetric("subagent_timeout_kills", 1)
		}
		m.mu.Unlock()
		// Kill the process.
		if sa.cmd != nil && sa.cmd.Process != nil {
			_ = sa.cmd.Process.Signal(syscall.SIGTERM)
			time.Sleep(2 * time.Second)
			_ = sa.cmd.Process.Kill()
		}
	}

	m.broadcastFinalEvent(sa)
	WriteAgentsMD(m.workspace, m.listLocked())
}

// RegisterSubAgent validates a sub-agent's auth token and returns its assignment.
// Called by the gRPC handler when a sub-agent process connects.
func (m *SubAgentManager) RegisterSubAgent(token string) (*SubAgent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, sa := range m.agents {
		if sa.authToken == token && sa.Status == StatusSpawning {
			sa.Status = StatusWorking
			return sa, nil
		}
	}
	return nil, fmt.Errorf("invalid sub-agent token")
}

// CompleteSubAgent marks a sub-agent as completed with a result.
func (m *SubAgentManager) CompleteSubAgent(name, result string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sa, ok := m.agents[name]
	if !ok || (sa.Status != StatusWorking && sa.Status != StatusSpawning) {
		return
	}
	sa.Status = StatusCompleted
	sa.Result = result
	now := time.Now()
	sa.CompletedAt = &now
	select {
	case <-sa.resultCh:
	default:
		close(sa.resultCh)
	}
}

// FailSubAgent marks a sub-agent as failed.
func (m *SubAgentManager) FailSubAgent(name, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sa, ok := m.agents[name]
	if !ok || (sa.Status != StatusWorking && sa.Status != StatusSpawning) {
		return
	}
	sa.Status = StatusFailed
	sa.Error = errMsg
	now := time.Now()
	sa.CompletedAt = &now
	select {
	case <-sa.resultCh:
	default:
		close(sa.resultCh)
	}
}

// IncrementToolCall increments the tool call count for a sub-agent
// and broadcasts a progress event.
func (m *SubAgentManager) IncrementToolCall(name string) {
	m.mu.Lock()
	sa, ok := m.agents[name]
	if ok {
		sa.ToolCallCount++
	}
	m.mu.Unlock()

	if ok {
		elapsed := time.Since(sa.CreatedAt).Milliseconds()
		m.broadcastEvent(&PipelineEvent{
			Type: EventSubAgentProgress,
			SubAgentProgress: &SubAgentProgressEvent{
				Name: sa.Name, LLMCalls: sa.LLMCallCount,
				ToolCalls: sa.ToolCallCount, ElapsedMs: elapsed,
			},
		})
	}
}

// IncrementLLMCall increments the LLM call count for a sub-agent.
func (m *SubAgentManager) IncrementLLMCall(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sa, ok := m.agents[name]; ok {
		sa.LLMCallCount++
	}
}

// SendMessage delivers a follow-up instruction to a running sub-agent.
// The message is buffered (capacity 1); if a previous message has not been
// consumed yet, the call returns an error.
func (m *SubAgentManager) SendMessage(name, content string) error {
	m.mu.RLock()
	sa, ok := m.agents[name]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("sub-agent %q not found", name)
	}
	if sa.Status != StatusWorking {
		return fmt.Errorf("sub-agent %q is not running (status: %s)", name, sa.Status)
	}
	select {
	case sa.messageCh <- content:
		return nil
	default:
		return fmt.Errorf("sub-agent %q has a pending message — wait for it to process", name)
	}
}

// PollMessage returns a pending follow-up message for the named sub-agent.
// Non-blocking: returns ("", false) when no message is queued.
func (m *SubAgentManager) PollMessage(name string) (string, bool) {
	m.mu.RLock()
	sa, ok := m.agents[name]
	m.mu.RUnlock()
	if !ok {
		return "", false
	}
	select {
	case msg := <-sa.messageCh:
		return msg, true
	default:
		return "", false
	}
}

// Status returns a sub-agent by name.
func (m *SubAgentManager) Status(name string) (*SubAgent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sa, ok := m.agents[name]
	if !ok {
		return nil, fmt.Errorf("sub-agent %q not found", name)
	}
	return sa, nil
}

// Result blocks until the sub-agent completes or times out.
func (m *SubAgentManager) Result(name string, timeout time.Duration) (string, error) {
	m.mu.RLock()
	sa, ok := m.agents[name]
	m.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("sub-agent %q not found", name)
	}

	select {
	case <-sa.resultCh:
		m.mu.RLock()
		defer m.mu.RUnlock()
		if sa.Status == StatusCompleted {
			return sa.Result, nil
		}
		return "", fmt.Errorf("sub-agent %s %s: %s", name, sa.Status, sa.Error)
	case <-time.After(timeout):
		return "", fmt.Errorf("timed out waiting for sub-agent %s", name)
	}
}

// Delete terminates a sub-agent immediately.
func (m *SubAgentManager) Delete(name string) error {
	m.mu.Lock()
	sa, ok := m.agents[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("sub-agent %q not found", name)
	}
	sa.Status = StatusCancelled
	now := time.Now()
	sa.CompletedAt = &now
	sa.cancel()
	select {
	case <-sa.resultCh:
	default:
		close(sa.resultCh)
	}
	m.mu.Unlock()

	// Kill the process.
	if sa.cmd != nil && sa.cmd.Process != nil {
		_ = sa.cmd.Process.Signal(syscall.SIGTERM)
		time.Sleep(2 * time.Second)
		_ = sa.cmd.Process.Kill()
	}

	m.broadcastEvent(&PipelineEvent{
		Type:              EventSubAgentCancelled,
		SubAgentCancelled: &SubAgentCancelledEvent{Name: name},
	})
	WriteAgentsMD(m.workspace, m.listLocked())
	return nil
}

// List returns all sub-agents.
func (m *SubAgentManager) List() []*SubAgent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.listLocked()
}

func (m *SubAgentManager) listLocked() []*SubAgent {
	result := make([]*SubAgent, 0, len(m.agents))
	for _, sa := range m.agents {
		result = append(result, sa)
	}
	return result
}

// Shutdown terminates all active sub-agents.
func (m *SubAgentManager) Shutdown() {
	m.mu.Lock()
	agents := make([]*SubAgent, 0)
	for _, sa := range m.agents {
		if sa.Status == StatusSpawning || sa.Status == StatusWorking {
			agents = append(agents, sa)
			sa.Status = StatusCancelled
			now := time.Now()
			sa.CompletedAt = &now
			sa.cancel()
			select {
			case <-sa.resultCh:
			default:
				close(sa.resultCh)
			}
		}
	}
	m.mu.Unlock()

	for _, sa := range agents {
		if sa.cmd != nil && sa.cmd.Process != nil {
			_ = sa.cmd.Process.Signal(syscall.SIGTERM)
		}
	}

	// Wait briefly for processes to exit.
	time.Sleep(2 * time.Second)
	for _, sa := range agents {
		if sa.cmd != nil && sa.cmd.Process != nil {
			_ = sa.cmd.Process.Kill()
		}
	}

	ClearAgentsMD(m.workspace)
}

// SubAgentSystemPrompt generates the system prompt for a sub-agent.
func SubAgentSystemPrompt(task string) string {
	return fmt.Sprintf(
		"You are a sub-agent working on a specific task. Complete the task and provide your result.\n\n"+
			"Task: %s\n\n"+
			"You have the tools you need. Complete your task efficiently and return a clear, structured result. "+
			"Do not ask questions — work with what you have. If you cannot complete the task, explain what is blocking you.",
		task)
}

// SubAgentToolDefsJSON serializes tool definitions to JSON for the gRPC response.
func SubAgentToolDefsJSON(tools []llm.ToolDefinition) string {
	data, _ := json.Marshal(tools)
	return string(data)
}

func (m *SubAgentManager) broadcastEvent(event *PipelineEvent) {
	m.mu.RLock()
	fn := m.broadcaster
	m.mu.RUnlock()
	if fn != nil {
		fn(event)
	}
}

func (m *SubAgentManager) broadcastFinalEvent(sa *SubAgent) {
	switch sa.Status {
	case StatusCompleted:
		dur := int64(0)
		if sa.CompletedAt != nil {
			dur = sa.CompletedAt.Sub(sa.CreatedAt).Milliseconds()
		}
		m.broadcastEvent(&PipelineEvent{
			Type: EventSubAgentCompleted,
			SubAgentCompleted: &SubAgentCompletedEvent{
				Name: sa.Name, Result: truncateResult(sa.Result, 2000), DurationMs: dur,
			},
		})
	case StatusFailed, StatusTimedOut:
		m.broadcastEvent(&PipelineEvent{
			Type: EventSubAgentFailed,
			SubAgentFailed: &SubAgentFailedEvent{
				Name: sa.Name, Error: sa.Error,
			},
		})
	case StatusCancelled:
		// Already broadcast in Delete.
	}
}

func truncateResult(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// isExcludedSubAgentGroup returns true for groups that sub-agents should never get.
func isExcludedSubAgentGroup(name string) bool {
	switch name {
	case "agents", "schedule", "memory":
		return true
	}
	return false
}

// filterSubAgentTools removes tools that sub-agents must not see.
// Agent management tools are excluded to prevent recursion. load_tools
// is excluded because sub-agents receive their full tool set at spawn
// time — if the LLM called load_tools, the loop would deadlock waiting
// for a response that never comes (the sub-agent callback ignores
// EventToolDefsRequest).
func filterSubAgentTools(tools []llm.ToolDefinition) []llm.ToolDefinition {
	excluded := map[string]bool{
		"load_tools":   true,
		"create_agent": true, "agent_status": true, "agent_result": true,
		"agent_message": true, "delete_agent": true, "list_agents": true,
	}
	filtered := make([]llm.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		if !excluded[t.Name] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// SubAgentManagerAdapter wraps SubAgentManager to implement the executor interface.
type SubAgentManagerAdapter struct {
	mgr *SubAgentManager
}

// NewSubAgentManagerAdapter creates an adapter for the executor package.
func NewSubAgentManagerAdapter(mgr *SubAgentManager) *SubAgentManagerAdapter {
	return &SubAgentManagerAdapter{mgr: mgr}
}

// Create delegates to SubAgentManager.Create.
func (a *SubAgentManagerAdapter) Create(req executors.SubAgentRequest) (string, error) {
	return a.mgr.Create(SubAgentRequest{
		Task:           req.Task,
		ToolGroups:     req.ToolGroups,
		Model:          req.Model,
		TimeoutSeconds: req.TimeoutSeconds,
		SessionID:      req.SessionID,
		IsOTR:          req.IsOTR,
	})
}

// Status delegates to SubAgentManager.Status.
func (a *SubAgentManagerAdapter) Status(name string) (executors.SubAgentInfo, error) {
	sa, err := a.mgr.Status(name)
	if err != nil {
		return executors.SubAgentInfo{}, err
	}
	return toSubAgentInfo(sa), nil
}

// Result delegates to SubAgentManager.Result.
func (a *SubAgentManagerAdapter) Result(name string, timeout time.Duration) (string, error) {
	return a.mgr.Result(name, timeout)
}

// SendMessage delegates to SubAgentManager.SendMessage.
func (a *SubAgentManagerAdapter) SendMessage(name, content string) error {
	return a.mgr.SendMessage(name, content)
}

// Delete delegates to SubAgentManager.Delete.
func (a *SubAgentManagerAdapter) Delete(name string) error {
	return a.mgr.Delete(name)
}

// List delegates to SubAgentManager.List.
func (a *SubAgentManagerAdapter) List() []executors.SubAgentInfo {
	agents := a.mgr.List()
	result := make([]executors.SubAgentInfo, len(agents))
	for i, sa := range agents {
		result[i] = toSubAgentInfo(sa)
	}
	return result
}

func toSubAgentInfo(sa *SubAgent) executors.SubAgentInfo {
	return executors.SubAgentInfo{
		Name:          sa.Name,
		Task:          sa.Task,
		Status:        string(sa.Status),
		Model:         sa.Model,
		ToolGroups:    sa.ToolGroups,
		Result:        sa.Result,
		Error:         sa.Error,
		LLMCallCount:  sa.LLMCallCount,
		ToolCallCount: sa.ToolCallCount,
		CreatedAt:     sa.CreatedAt,
		CompletedAt:   sa.CompletedAt,
	}
}

// Ensure SubAgentManagerAdapter satisfies the executor interface.
var _ executors.SubAgentManagerInterface = (*SubAgentManagerAdapter)(nil)
