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

	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/internal/channels"
	"github.com/openparallax/openparallax/internal/channels/discord"
	"github.com/openparallax/openparallax/internal/channels/imessage"
	signalch "github.com/openparallax/openparallax/internal/channels/signal"
	"github.com/openparallax/openparallax/internal/channels/telegram"
	"github.com/openparallax/openparallax/internal/channels/whatsapp"
	"github.com/openparallax/openparallax/internal/engine"
	"github.com/openparallax/openparallax/internal/engine/executors"
	"github.com/openparallax/openparallax/internal/heartbeat"
	"github.com/openparallax/openparallax/internal/registry"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/internal/web"
	"github.com/openparallax/openparallax/llm"
	"github.com/openparallax/openparallax/sandbox"
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
	if !cfg.Web.Enabled {
		_, _ = fmt.Fprintln(os.Stdout, "WEB_DISABLED")
	} else {
		webPort := cfg.Web.Port
		if webPort == 0 {
			webPort = 3000
		}
		webHost := cfg.Web.Host
		if webHost == "" {
			webHost = "127.0.0.1"
		}
		webServer = web.NewServer(eng, eng.Log(), web.ServerConfig{
			Host:           webHost,
			Port:           webPort,
			PasswordHash:   cfg.Web.PasswordHash,
			AllowedOrigins: cfg.Web.AllowedOrigins,
		})

		// Bind the port first so we know it works before opening the browser.
		if listenErr := webServer.Listen(); listenErr != nil {
			eng.Log().Error("web_server_listen_failed", "error", listenErr, "port", webPort)
			// Report via stdout so the parent process can display the error
			// (stderr is hidden under bubbletea alt screen).
			_, _ = fmt.Fprintf(os.Stdout, "WEB_FAILED:%d:%s\n", webPort, listenErr)
			webServer = nil
		} else {
			go func() {
				if serveErr := webServer.Serve(); serveErr != nil {
					eng.Log().Error("web_server_failed", "error", serveErr)
				}
			}()

			// Give Serve() a moment to start the accept loop before
			// opening the browser and signaling the parent.
			time.Sleep(100 * time.Millisecond)

			_, _ = fmt.Fprintf(os.Stdout, "WEB:%d\n", webPort)

			url := fmt.Sprintf("http://127.0.0.1:%d", webPort)
			eng.Log().Info("web_ui_available", "url", url)

			if browserPath := executors.DetectBrowser(); browserPath != "" {
				parts := strings.Fields(browserPath)
				var browserCmd *exec.Cmd
				if len(parts) > 1 {
					browserCmd = exec.Command(parts[0], append(parts[1:], url)...)
				} else {
					browserCmd = exec.Command(browserPath, url)
				}
				// Prevent browser from inheriting the engine's stdout pipe.
				if dn, e := os.OpenFile(os.DevNull, os.O_WRONLY, 0); e == nil {
					browserCmd.Stdout = dn
					browserCmd.Stderr = dn
				}
				_ = browserCmd.Start()
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
	if imAdapter := imessage.New(cfg.Channels.IMessage, channelMgr, eng.Log()); imAdapter != nil {
		channelMgr.Register(imAdapter)
	}
	channelMgr.StartAll(channelCtx)
	eng.SetApprovalNotifier(channelMgr)
	eng.SetChannelController(channelMgr)

	// Start heartbeat scheduler for HEARTBEAT.md cron entries.
	hbLoop := heartbeat.NewLoop(cfg.Workspace, func(task string) {
		eng.ProcessHeartbeatTask(channelCtx, task)
	}, eng.Log())
	hbLoop.Start(channelCtx)
	if count := hbLoop.EntryCount(); count > 0 {
		eng.Log().Info("heartbeat_started", "entries", count)
	}

	// Spawn the sandboxed Agent child process.
	agentName := cfg.Identity.Name
	if agentName == "" {
		agentName = types.DefaultIdentity.Name
	}
	chatModel, _ := cfg.ChatModel()
	am := newAgentManager(grpcAddr, agentName, cfg.Workspace, llm.APIHost(chatModel.LLMConfig()), eng.SetAgentAuthToken)
	if amErr := am.spawnAgent(); amErr != nil {
		eng.Log().Error("agent_spawn_failed", "error", amErr)
		// Fall through — the engine still serves the web UI even without the CLI agent.
	}

	// Sandbox status is set from Probe() above. The agent verifies
	// enforcement via canary probes internally and refuses to start
	// if required probes fail.

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case <-am.done:
		// Agent exited. Keep the web server running so the user can
		// continue using the web UI. Only shut down on signal.
		if webServer != nil {
			if am.cleanExit {
				eng.Log().Info("agent_exited",
					"message", "CLI agent exited — web UI remains available")
			} else {
				eng.Log().Warn("agent_crashed",
					"message", "CLI agent exited — web UI remains available for diagnostics")
			}
			<-sigCh
		}
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
	llmHost   string
	authToken string
	// onTokenChanged is invoked after a successful spawn (initial OR
	// restart) so the engine can keep its expected token in sync. Without
	// this, the engine would still expect the very first token after a
	// crash-and-respawn, and the new agent's fresh token would fail auth.
	onTokenChanged func(token string)

	mu        sync.Mutex
	cmd       *exec.Cmd
	done      chan struct{}
	crashes   []time.Time
	cleanExit bool // true when agent exited with code 0 (e.g. /quit)
}

func newAgentManager(grpcAddr, agentName, workspace, llmHost string, onTokenChanged func(string)) *agentManager {
	return &agentManager{
		grpcAddr:       grpcAddr,
		agentName:      agentName,
		workspace:      workspace,
		llmHost:        llmHost,
		done:           make(chan struct{}),
		onTokenChanged: onTokenChanged,
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

	// Generate ephemeral auth token for this agent instance.
	token, tokenErr := crypto.RandomHex(16)
	if tokenErr != nil {
		return fmt.Errorf("generate agent auth token: %w", tokenErr)
	}
	am.authToken = token

	am.cmd = exec.Command(executable, "internal-agent",
		"--grpc", am.grpcAddr,
		"--name", am.agentName,
		"--workspace", am.workspace)
	am.cmd.Env = append(os.Environ(), "OPENPARALLAX_AGENT_TOKEN="+token)

	// Agent is headless — discard stdout/stderr to avoid pipe issues.
	devNull, openErr := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if openErr == nil {
		am.cmd.Stdout = devNull
	}
	am.cmd.Stderr = os.Stderr

	// Apply sandbox wrapping (macOS sandbox-exec, Windows Job Objects).
	// On Linux, the agent self-sandboxes via Landlock on startup.
	sb := sandbox.New()
	if sb.Available() {
		_ = sb.WrapCommand(am.cmd, sandbox.Config{
			AllowedReadPaths:  []string{executable, am.workspace},
			AllowedWritePaths: []string{},
			AllowedTCPConnect: []string{am.grpcAddr, am.llmHost},
			AllowProcessSpawn: false,
		})
	}

	if err := am.cmd.Start(); err != nil {
		return fmt.Errorf("agent spawn: %w", err)
	}

	// Sync the engine's expected token to the freshly-generated one
	// before any agent connection attempt can race against it. The
	// callback is invoked under am.mu so a concurrent monitor restart
	// cannot interleave a stale token update.
	if am.onTokenChanged != nil {
		am.onTokenChanged(token)
	}

	am.done = make(chan struct{})
	go am.monitor()
	return nil
}

// monitor watches the Agent process and respawns on crash (max 5 in 60s).
func (am *agentManager) monitor() {
	am.mu.Lock()
	done := am.done
	am.mu.Unlock()
	defer close(done)

	for {
		am.mu.Lock()
		cmd := am.cmd
		am.mu.Unlock()

		if cmd == nil || cmd.Process == nil {
			return
		}

		err := cmd.Wait()
		if err == nil {
			am.mu.Lock()
			am.cleanExit = true
			am.mu.Unlock()
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
