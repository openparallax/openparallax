//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/types"
)

// SharedEngine is the single engine instance shared across all tests.
var SharedEngine *TestEngine

// TestEngine manages the full engine lifecycle for E2E tests.
type TestEngine struct {
	WorkspaceDir string
	WebPort      int
	ConfigPath   string
	mockServer   *MockLLMServer
	cmd          *exec.Cmd
	binaryPath   string
}

// probeTimeout is the max wait for engine + agent readiness. 90s gives
// the probe loop room to retry the warmup round-trip several times on
// slow CI runners; only a truly hung process hits this.
const probeTimeout = 90 * time.Second

// SetupSharedEngine creates workspace, starts mock LLM + engine, probes
// for full readiness. Called once from TestMain.
func SetupSharedEngine() *TestEngine {
	// Kill ALL leftover test engine processes. We identify them by the
	// --config flag pointing to a temp directory with our e2e prefix.
	if out, _ := exec.Command("pgrep", "-a", "openparallax").Output(); len(out) > 0 {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "openparallax-e2e") && strings.Contains(line, "internal-engine") {
				fields := strings.Fields(line)
				if len(fields) > 0 {
					var pid int
					if _, err := fmt.Sscanf(fields[0], "%d", &pid); err == nil {
						_ = syscall.Kill(-pid, syscall.SIGKILL)
					}
				}
			}
		}
		time.Sleep(1 * time.Second)
	}

	te := &TestEngine{}

	workspace, err := os.MkdirTemp("", "openparallax-e2e-*")
	if err != nil {
		fatal("create workspace: %v", err)
	}
	te.WorkspaceDir = workspace
	te.WebPort = freePortMain()
	te.ConfigPath = filepath.Join(workspace, "config.yaml")

	mode := os.Getenv("E2E_LLM")
	if mode == "" {
		mode = "mock"
	}

	var models []types.ModelEntry
	var roles types.RolesConfig

	switch mode {
	case "mock":
		mock, mockErr := NewMockLLMServer()
		if mockErr != nil {
			fatal("start mock LLM: %v", mockErr)
		}
		te.mockServer = mock
		os.Setenv("E2E_MOCK_KEY", "mock-test-key")
		models = []types.ModelEntry{
			{Name: "mock", Provider: "openai", Model: "mock-v1", APIKeyEnv: "E2E_MOCK_KEY", BaseURL: mock.BaseURL()},
		}
		roles = types.RolesConfig{Chat: "mock", SubAgent: "mock"}

	case "cloud":
		provider := os.Getenv("E2E_PROVIDER")
		if provider == "" {
			provider = "openai"
		}
		model := os.Getenv("E2E_MODEL")
		baseURL := os.Getenv("E2E_BASE_URL")
		apiKeyEnv := ""

		switch provider {
		case "anthropic":
			apiKeyEnv = "ANTHROPIC_API_KEY"
			if model == "" {
				model = "claude-haiku-4-5-20251001"
			}
		case "openai":
			apiKeyEnv = "OPENAI_API_KEY"
			if model == "" {
				model = "gpt-4o-mini"
			}
		case "google":
			apiKeyEnv = "GOOGLE_AI_API_KEY"
			if model == "" {
				model = "gemini-2.0-flash"
			}
		}

		if os.Getenv(apiKeyEnv) == "" {
			fatal("%s not set", apiKeyEnv)
		}
		models = []types.ModelEntry{
			{Name: "cloud", Provider: provider, Model: model, APIKeyEnv: apiKeyEnv, BaseURL: baseURL},
		}
		roles = types.RolesConfig{Chat: "cloud", SubAgent: "cloud"}

	case "ollama":
		model := os.Getenv("OLLAMA_MODEL")
		if model == "" {
			model = "llama3.2"
		}
		baseURL := os.Getenv("OLLAMA_HOST")
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		models = []types.ModelEntry{
			{Name: "ollama", Provider: "ollama", Model: model, BaseURL: baseURL},
		}
		roles = types.RolesConfig{Chat: "ollama", SubAgent: "ollama"}
	}

	grpcPort := te.WebPort + 1000

	cfg := &types.AgentConfig{
		Workspace: workspace,
		Models:    models,
		Roles:     roles,
		Identity:  types.IdentityConfig{Name: "E2ETest", Avatar: "T"},
		Shield: types.ShieldConfig{
			PolicyFile:       "security/shield/default.yaml",
			OnnxThreshold:    0.85,
			HeuristicEnabled: true,
		},
		Chronicle: types.ChronicleConfig{MaxSnapshots: 10, MaxAgeDays: 1},
		Web:       types.WebConfig{Enabled: true, Port: te.WebPort, GRPCPort: grpcPort, Auth: false},
		Agents: types.AgentsConfig{
			MaxToolRounds:          25,
			ContextWindow:          128000,
			CompactionThreshold:    70,
			MaxResponseTokens:      4096,
			ShellTimeoutSeconds:    10,
			SubAgentTimeoutSeconds: 60,
			MaxConcurrentSubAgents: 5,
			CrashRestartBudget:     3,
			CrashWindowSeconds:     30,
		},
		General: types.GeneralConfig{
			FailClosed:        true,
			RateLimit:         60,
			VerdictTTLSeconds: 60,
			DailyBudget:       100,
		},
		Security: types.SecurityConfig{
			IFCPolicy: "security/ifc/default.yaml",
		},
	}

	writeWorkspaceFiles(workspace)
	if saveErr := config.Save(te.ConfigPath, cfg); saveErr != nil {
		fatal("write config: %v", saveErr)
	}

	te.binaryPath = buildBinaryMain()
	return te
}

// Start boots the engine and waits for full readiness.
func (te *TestEngine) Start() error {
	te.cmd = exec.Command(te.binaryPath, "internal-engine",
		"--config", te.ConfigPath,
		"--verbose",
	)
	te.cmd.Env = append(os.Environ(), "E2E_MOCK_KEY=mock-test-key")
	te.cmd.Stderr = os.Stderr
	te.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := te.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	if err := te.cmd.Start(); err != nil {
		return fmt.Errorf("start engine: %w", err)
	}
	// Save PID so the next run can kill leftovers.
	pidFile := filepath.Join(os.TempDir(), "openparallax-e2e.pid")
	_ = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", te.cmd.Process.Pid)), 0o644)

	buf := make([]byte, 256)
	n, _ := stdout.Read(buf)
	line := strings.TrimSpace(string(buf[:n]))
	if !strings.HasPrefix(line, "PORT:") {
		return fmt.Errorf("expected PORT: line, got: %q", line)
	}

	return te.ProbeReady()
}

// ProbeReady waits for web server + agent to be fully operational, then
// performs a warmup round-trip through the WebSocket pipeline. The HTTP
// status check confirms the engine is up; the WebSocket round-trip
// confirms the agent gRPC stream is ready to receive and respond to
// messages. Without the round-trip, the first test in the suite races
// against the agent's gRPC connection setup and can hang indefinitely
// on slow CI runners.
func (te *TestEngine) ProbeReady() error {
	rest := te.REST()
	deadline := time.Now().Add(probeTimeout)

	for time.Now().Before(deadline) {
		body, code, err := rest.Get("/api/status")
		if err != nil || code != 200 {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		var status map[string]any
		if json.Unmarshal(body, &status) != nil {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		// Agent connected = sandbox probes ran.
		if sb, ok := status["sandbox"].(map[string]any); ok {
			if active, _ := sb["active"].(bool); active {
				// Verify session creation works (DB ready).
				_, code, _ := rest.Post("/api/sessions", map[string]string{"mode": "normal"})
				if code != 200 && code != 201 {
					time.Sleep(500 * time.Millisecond)
					continue
				}
				// Warmup round-trip: send a real message through the WS
				// pipeline and confirm response_complete arrives. This
				// guarantees the agent's gRPC stream is fully operational.
				if err := te.warmupRoundTrip(); err != nil {
					time.Sleep(500 * time.Millisecond)
					continue
				}
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("engine/agent not ready after %s", probeTimeout)
}

// warmupRoundTrip sends one message through the WebSocket pipeline and
// waits for response_complete. Used by ProbeReady to confirm the
// agent gRPC stream is alive before returning.
func (te *TestEngine) warmupRoundTrip() error {
	rest := te.REST()
	body, code, err := rest.Post("/api/sessions", map[string]string{"mode": "normal"})
	if err != nil || (code != 200 && code != 201) {
		return fmt.Errorf("warmup session create: code=%d err=%v", code, err)
	}
	var resp map[string]any
	if jsonErr := json.Unmarshal(body, &resp); jsonErr != nil {
		return fmt.Errorf("warmup parse session: %w", jsonErr)
	}
	sid, _ := resp["id"].(string)
	if sid == "" {
		return fmt.Errorf("warmup session id missing")
	}

	url := fmt.Sprintf("ws://127.0.0.1:%d/api/ws", te.WebPort)
	ws, err := NewWSClient(url)
	if err != nil {
		return fmt.Errorf("warmup ws connect: %w", err)
	}
	defer ws.Close()

	if err := ws.SendMessage(sid, "warmup"); err != nil {
		return fmt.Errorf("warmup send: %w", err)
	}

	if _, err := ws.CollectUntil("response_complete", 10*time.Second); err != nil {
		return fmt.Errorf("warmup collect: %w", err)
	}
	return nil
}

// Stop kills the entire process group (engine + agent child) and waits.
func (te *TestEngine) Stop() {
	if te.cmd == nil || te.cmd.Process == nil {
		return
	}
	pgid := te.cmd.Process.Pid

	// Try graceful first.
	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		_ = te.cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		// Force kill entire group.
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			// Process is truly stuck — move on.
		}
	}
	te.cmd = nil
}

// ProbeShutdown waits until the web port is no longer accepting connections.
// This confirms the engine process is fully dead and the port is released.
func (te *TestEngine) ProbeShutdown() error {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", te.WebPort), 200*time.Millisecond)
		if err != nil {
			// Connection refused = port is free = engine is dead.
			return nil
		}
		conn.Close()
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("engine still alive on port %d after shutdown", te.WebPort)
}

// Restart stops the engine, probes for full shutdown, then starts fresh
// and probes for readiness.
func (te *TestEngine) Restart() error {
	te.Stop()
	if err := te.ProbeShutdown(); err != nil {
		return fmt.Errorf("shutdown probe: %w", err)
	}
	return te.Start()
}

// Cleanup stops everything, probes for full shutdown, and removes workspace.
// Blocks until the engine is confirmed dead and ports are free.
func (te *TestEngine) Cleanup() {
	te.Stop()
	// Block until the port is truly free. This prevents the next test run
	// from colliding with a dying engine.
	if err := te.ProbeShutdown(); err != nil {
		// Last resort: the process might be stuck. Try SIGKILL on the pgid.
		if te.cmd != nil && te.cmd.Process != nil {
			_ = syscall.Kill(-te.cmd.Process.Pid, syscall.SIGKILL)
		}
		_ = te.ProbeShutdown() // Try once more.
	}
	if te.mockServer != nil {
		te.mockServer.Stop()
		te.mockServer = nil
	}
	if te.WorkspaceDir != "" {
		_ = os.RemoveAll(te.WorkspaceDir)
	}
	// Extra wait to ensure TCP sockets fully close.
	time.Sleep(2 * time.Second)
}

// REST returns a REST client.
func (te *TestEngine) REST() *RESTClient {
	return NewRESTClient(fmt.Sprintf("http://127.0.0.1:%d", te.WebPort))
}

// WS returns a WebSocket client.
func (te *TestEngine) WS(t *testing.T) *WSClient {
	t.Helper()
	url := fmt.Sprintf("ws://127.0.0.1:%d/api/ws", te.WebPort)
	ws, err := NewWSClient(url)
	if err != nil {
		t.Fatalf("ws connect: %v", err)
	}
	t.Cleanup(ws.Close)
	return ws
}

// CreateSession creates a new session via REST.
func (te *TestEngine) CreateSession(t *testing.T, mode string) string {
	t.Helper()
	if mode == "" {
		mode = "normal"
	}
	body, code, err := te.REST().Post("/api/sessions", map[string]string{"mode": mode})
	if err != nil || (code != 200 && code != 201) {
		t.Fatalf("create session: code=%d err=%v body=%s", code, err, string(body))
	}
	var resp map[string]any
	if jsonErr := json.Unmarshal(body, &resp); jsonErr != nil {
		t.Fatalf("parse session response: %v", jsonErr)
	}
	id, ok := resp["id"].(string)
	if !ok {
		t.Fatalf("session response missing id: %s", string(body))
	}
	return id
}

// --- Helpers ---

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "e2e: "+format+"\n", args...)
	os.Exit(1)
}

func freePortMain() int {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fatal("find free port: %v", err)
	}
	port := lis.Addr().(*net.TCPAddr).Port
	_ = lis.Close()
	return port
}

func repoRootMain() string {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	if err != nil {
		fatal("find repo root: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func buildBinaryMain() string {
	root := repoRootMain()
	distPath := filepath.Join(root, "dist", "openparallax")
	if _, err := os.Stat(distPath); err == nil {
		return distPath
	}
	tmpBin := filepath.Join(os.TempDir(), "openparallax-e2e")
	cmd := exec.Command("go", "build", "-o", tmpBin, "./cmd/agent")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		fatal("build binary: %v\n%s", err, out)
	}
	return tmpBin
}

func writeWorkspaceFiles(workspace string) {
	dirs := []string{
		filepath.Join(workspace, "security", "shield"),
		filepath.Join(workspace, "security", "ifc"),
		filepath.Join(workspace, ".openparallax"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			fatal("mkdir %s: %v", d, err)
		}
	}
	files := map[string]string{
		filepath.Join(workspace, "SOUL.md"):                  "# Core Values\n\nBe helpful, be safe.",
		filepath.Join(workspace, "IDENTITY.md"):              "# Identity\n\nName: E2ETest\nRole: Testing agent",
		filepath.Join(workspace, "USER.md"):                  "# User\n\nE2E test user.",
		filepath.Join(workspace, "MEMORY.md"):                "",
		filepath.Join(workspace, "HEARTBEAT.md"):             "",
		filepath.Join(workspace, "security", "shield", "default.yaml"): defaultTestPolicy,
		filepath.Join(workspace, "security", "ifc", "default.yaml"):    defaultTestIFCPolicy,
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			fatal("write %s: %v", path, err)
		}
	}
}

const defaultTestPolicy = `deny:
  - name: block_sensitive_paths
    action_types:
      - read_file
      - write_file
    paths:
      - "~/.ssh/**"
      - "~/.aws/**"
      - "/etc/shadow"

  - name: block_destructive_commands
    action_types:
      - execute_command
    paths:
      - "rm -rf /**"

verify:
  - name: evaluate_shell
    action_types:
      - execute_command
    tier_override: 1

allow:
  - name: allow_workspace
    action_types:
      - read_file
      - write_file
      - list_directory
      - search_files
      - memory_write
      - memory_search
`

const defaultTestIFCPolicy = `mode: enforce
sources:
  - name: ssh_keys
    sensitivity: critical
    match:
      path_contains: ["/.ssh/"]
  - name: env_files
    sensitivity: critical
    match:
      basename_in: [".env", ".env.local", ".env.production"]
      basename_not_in: [".env.example", ".env.template", ".env.sample"]
  - name: system_security
    sensitivity: critical
    match:
      path_in: ["/etc/shadow", "/etc/sudoers"]
  - name: default
    sensitivity: public
    match: {}
memory_block_levels: [critical, restricted]
sinks:
  external: [http_request, send_email, send_message]
  exec: [execute_command]
  memory: [memory_write]
  workspace_write: [write_file, create_directory, move_file, copy_file, delete_file]
  workspace_read: [read_file, list_directory, search_files, memory_search, grep_files]
rules:
  public:
    external: allow
    exec: allow
    memory: allow
    workspace_write: allow
    workspace_read: allow
  internal:
    external: block
    exec: allow
    memory: allow
    workspace_write: allow
    workspace_read: allow
  confidential:
    external: block
    exec: allow
    memory: allow
    workspace_write: allow
    workspace_read: allow
  restricted:
    external: block
    exec: block
    memory: block
    workspace_write: block
    workspace_read: allow
  critical:
    external: block
    exec: block
    memory: block
    workspace_write: block
    workspace_read: block
`
