package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/openparallax/openparallax/internal/channels"
	"github.com/openparallax/openparallax/internal/channels/discord"
	signalch "github.com/openparallax/openparallax/internal/channels/signal"
	"github.com/openparallax/openparallax/internal/channels/telegram"
	"github.com/openparallax/openparallax/internal/channels/whatsapp"
	"github.com/openparallax/openparallax/internal/engine"
	"github.com/openparallax/openparallax/internal/engine/executors"
	"github.com/openparallax/openparallax/internal/registry"
	"github.com/openparallax/openparallax/internal/sandbox"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/internal/web"
	"github.com/spf13/cobra"
)

var (
	engineConfigPath string
	engineVerbose    bool
)

var internalEngineCmd = &cobra.Command{
	Use:          "internal-engine",
	Short:        "Run the engine process (internal use only)",
	Hidden:       true,
	SilenceUsage: true,
	RunE:         runInternalEngine,
}

func init() {
	internalEngineCmd.Flags().StringVar(&engineConfigPath, "config", "", "path to config.yaml")
	internalEngineCmd.Flags().BoolVar(&engineVerbose, "verbose", false, "enable verbose pipeline logging")
	rootCmd.AddCommand(internalEngineCmd)
}

// runInternalEngine starts the engine gRPC server, web UI, and spawns the
// sandboxed Agent child process. It blocks until the agent exits or a signal
// is received. Writes the gRPC port to stdout so the parent process manager
// can read it.
func runInternalEngine(_ *cobra.Command, _ []string) error {
	if engineConfigPath == "" {
		return fmt.Errorf("--config flag is required")
	}

	eng, err := engine.New(engineConfigPath, engineVerbose)
	if err != nil {
		return fmt.Errorf("engine init failed: %w", err)
	}

	cfg := eng.Config()
	port, err := eng.Start(cfg.Web.GRPCPort)
	if err != nil {
		return fmt.Errorf("engine start failed: %w", err)
	}

	grpcAddr := fmt.Sprintf("localhost:%d", port)
	_, _ = fmt.Fprintf(os.Stdout, "PORT:%d\n", port)

	// Write PID file so stop/list commands can find us.
	_ = registry.WritePID(cfg.Workspace, os.Getpid())

	// Initialize sub-agent orchestration.
	eng.SetupSubAgents(grpcAddr)

	// Probe sandbox capability (what the kernel supports).
	sbStatus := sandbox.Probe()
	eng.SetSandboxStatus(sbStatus.Active, sbStatus.Mode, sbStatus.Version,
		sbStatus.Filesystem, sbStatus.Network, sbStatus.Reason)
	if sbStatus.Active {
		eng.Log().Info("sandbox_available",
			"mode", sbStatus.Mode,
			"version", sbStatus.Version,
			"filesystem", sbStatus.Filesystem,
			"network", sbStatus.Network)
	} else {
		eng.Log().Warn("sandbox_unavailable", "reason", sbStatus.Reason)
	}

	// Start web server if configured.
	var webServer *web.Server
	if cfg.Web.Enabled {
		webPort := cfg.Web.Port
		if webPort == 0 {
			webPort = 3000
		}
		webServer = web.NewServer(eng, eng.Log(), webPort)
		go func() {
			if webErr := webServer.Start(); webErr != nil {
				eng.Log().Error("web_server_failed", "error", webErr)
			}
		}()

		url := fmt.Sprintf("http://127.0.0.1:%d", webPort)
		eng.Log().Info("web_ui_available", "url", url)

		if browserPath := executors.DetectBrowser(); browserPath != "" {
			parts := strings.Fields(browserPath)
			if len(parts) > 1 {
				_ = exec.Command(parts[0], append(parts[1:], url)...).Start()
			} else {
				_ = exec.Command(browserPath, url).Start()
			}
		}
	}

	// Wire sub-agent events to WebSocket broadcast.
	if webServer != nil && eng.SubAgentManager() != nil {
		eng.SubAgentManager().SetEventBroadcaster(webServer.BroadcastEvent)
	}

	// Start channel adapters (Telegram, etc.).
	channelCtx, channelCancel := context.WithCancel(context.Background())
	defer channelCancel()
	channelMgr := channels.NewManager(eng, eng.Log())
	if tgAdapter := telegram.New(cfg.Channels.Telegram, channelMgr, eng.Log()); tgAdapter != nil {
		channelMgr.Register(tgAdapter)
	}
	if waAdapter := whatsapp.New(cfg.Channels.WhatsApp, channelMgr, eng.Log()); waAdapter != nil {
		channelMgr.Register(waAdapter)
	}
	if dcAdapter := discord.New(cfg.Channels.Discord, channelMgr, eng.Log()); dcAdapter != nil {
		channelMgr.Register(dcAdapter)
	}
	if sgAdapter := signalch.New(cfg.Channels.Signal, channelMgr, eng.Log()); sgAdapter != nil {
		channelMgr.Register(sgAdapter)
	}
	channelMgr.StartAll(channelCtx)

	// Spawn the sandboxed Agent child process.
	agentName := cfg.Identity.Name
	if agentName == "" {
		agentName = types.DefaultIdentity.Name
	}
	am := newAgentManager(grpcAddr, agentName, cfg.Workspace)
	if amErr := am.spawnAgent(); amErr != nil {
		eng.Log().Error("agent_spawn_failed", "error", amErr)
		// Fall through — the engine still serves the web UI even without the CLI agent.
	}

	// Read canary verification result from the Agent process. The agent
	// writes sandbox.status after applying the sandbox and running the
	// canary probe. Wait briefly for it to appear.
	go func() {
		time.Sleep(2 * time.Second)
		canary := sandbox.ReadCanaryResult(cfg.Workspace)
		switch {
		case canary.Verified:
			eng.SetSandboxStatus(true, canary.Mechanism, sbStatus.Version,
				sbStatus.Filesystem, sbStatus.Network, "")
			eng.Log().Info("sandbox_verified",
				"status", canary.Status,
				"canary_path", canary.CanaryPath,
				"canary_blocked", canary.CanaryBlocked,
				"mechanism", canary.Mechanism,
				"platform", canary.Platform)
		case canary.Status == "unsandboxed":
			eng.SetSandboxStatus(false, canary.Mechanism, 0,
				false, false, "canary probe succeeded — sandbox not active")
			eng.Log().Warn("sandbox_not_verified",
				"status", canary.Status,
				"canary_path", canary.CanaryPath,
				"platform", canary.Platform)
		case canary.Status != "unknown":
			eng.Log().Warn("sandbox_inconclusive",
				"status", canary.Status,
				"error", canary.Error)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case <-am.done:
		// Agent exited (user typed /quit or final crash).
	case <-sigCh:
		am.stopAgent()
	}

	channelCancel()
	channelMgr.StopAll()
	if webServer != nil {
		webServer.Stop()
	}
	eng.Stop()
	_ = registry.RemovePID(cfg.Workspace)
	return nil
}

// agentManager spawns and supervises the sandboxed Agent child process.
type agentManager struct {
	grpcAddr  string
	agentName string
	workspace string

	mu      sync.Mutex
	cmd     *exec.Cmd
	done    chan struct{}
	crashes []time.Time
}

func newAgentManager(grpcAddr, agentName, workspace string) *agentManager {
	return &agentManager{
		grpcAddr:  grpcAddr,
		agentName: agentName,
		workspace: workspace,
		done:      make(chan struct{}),
	}
}

// spawnAgent starts the Agent process with sandbox applied.
func (am *agentManager) spawnAgent() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find own executable: %w", err)
	}

	am.cmd = exec.Command(executable, "internal-agent",
		"--grpc", am.grpcAddr,
		"--name", am.agentName,
		"--workspace", am.workspace)

	// Agent opens /dev/tty directly for bubbletea TUI, so stdout
	// is not needed. Send it to devnull to avoid pipe buffer issues.
	devNull, openErr := os.Open(os.DevNull)
	if openErr == nil {
		am.cmd.Stdout = devNull
	}
	am.cmd.Stderr = os.Stderr
	// stdin is inherited so the agent can open /dev/tty.

	// Apply sandbox wrapping (macOS sandbox-exec, Windows Job Objects).
	// On Linux, the agent self-sandboxes via Landlock on startup.
	sb := sandbox.New()
	if sb.Available() {
		_ = sb.WrapCommand(am.cmd, sandbox.Config{
			AllowedReadPaths:  []string{executable},
			AllowedTCPConnect: []string{am.grpcAddr},
			AllowProcessSpawn: false,
		})
	}

	if err := am.cmd.Start(); err != nil {
		return fmt.Errorf("agent spawn: %w", err)
	}

	go am.monitor()
	return nil
}

// monitor watches the Agent process and respawns on crash (max 5 in 60s).
func (am *agentManager) monitor() {
	defer close(am.done)

	for {
		am.mu.Lock()
		cmd := am.cmd
		am.mu.Unlock()

		if cmd == nil || cmd.Process == nil {
			return
		}

		err := cmd.Wait()
		if err == nil {
			return // Clean exit (user typed /quit).
		}

		// Agent crashed. Check restart budget.
		now := time.Now()
		am.mu.Lock()
		am.crashes = append(am.crashes, now)

		cutoff := now.Add(-60 * time.Second)
		recentCrashes := 0
		for _, t := range am.crashes {
			if t.After(cutoff) {
				recentCrashes++
			}
		}
		am.mu.Unlock()

		if recentCrashes >= 5 {
			fmt.Fprintf(os.Stderr, "Agent has crashed %d times in 60 seconds. Giving up.\n", recentCrashes)
			return
		}

		fmt.Fprintf(os.Stderr, "Agent crashed (%s). Restarting in 1 second...\n", err)
		time.Sleep(time.Second)

		if spawnErr := am.spawnAgent(); spawnErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to restart agent: %s\n", spawnErr)
			return
		}
	}
}

// stopAgent sends SIGTERM to the Agent and waits up to 5 seconds.
func (am *agentManager) stopAgent() {
	am.mu.Lock()
	cmd := am.cmd
	am.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)

	waited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(waited)
	}()

	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
	}
}
