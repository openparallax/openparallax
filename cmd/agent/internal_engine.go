package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
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
	if pidErr := registry.WritePID(cfg.Workspace, os.Getpid()); pidErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to write PID file: %v\n", pidErr)
	}

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
	am := newAgentManager(grpcAddr, agentName, cfg.Workspace, llm.APIHost(chatModel.LLMConfig()), eng.SetAgentAuthToken, cfg.Agents.CrashRestartBudget, cfg.Agents.CrashWindowSeconds)
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
			am.mu.Lock()
			exitCode, exitSig := am.exitCode, am.exitSig
			am.mu.Unlock()
			if am.cleanExit {
				eng.Log().Info("agent_exited",
					"message", "CLI agent exited — web UI remains available",
					"exit_code", exitCode, "signal", exitSig)
			} else {
				eng.Log().Warn("agent_crashed",
					"message", "CLI agent exited — web UI remains available for diagnostics",
					"exit_code", exitCode, "signal", exitSig)
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
	if rmErr := registry.RemovePID(cfg.Workspace); rmErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to remove PID file: %v\n", rmErr)
	}
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
	crashBudget    int
	crashWindowS   int

	mu        sync.Mutex
	cmd       *exec.Cmd
	done      chan struct{}
	crashes   []time.Time
	cleanExit bool   // true when agent exited with code 0 (e.g. /quit)
	exitCode  string // exit code from last Wait(), populated by monitor
	exitSig   string // signal name if killed, populated by monitor
}

func newAgentManager(grpcAddr, agentName, workspace, llmHost string, onTokenChanged func(string), crashBudget, crashWindowS int) *agentManager {
	if crashBudget <= 0 {
		crashBudget = 5
	}
	if crashWindowS <= 0 {
		crashWindowS = 60
	}
	return &agentManager{
		grpcAddr:       grpcAddr,
		agentName:      agentName,
		workspace:      workspace,
		llmHost:        llmHost,
		done:           make(chan struct{}),
		onTokenChanged: onTokenChanged,
		crashBudget:    crashBudget,
		crashWindowS:   crashWindowS,
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

	// Agent is headless — discard stdout to avoid pipe issues. Stderr goes
	// to a dedicated log file so panic stacks survive terminal close. Without
	// this, agent crashes are undebuggable when --verbose is not set.
	devNull, openErr := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if openErr == nil {
		am.cmd.Stdout = devNull
	}
	stderrPath := filepath.Join(am.workspace, ".openparallax", "agent.stderr.log")
	rotateAgentStderr(stderrPath)
	stderrFile, stderrErr := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if stderrErr == nil {
		am.cmd.Stderr = stderrFile
	} else {
		am.cmd.Stderr = os.Stderr
	}

	// Apply sandbox wrapping (macOS sandbox-exec, Windows Job Objects).
	// On Linux, the agent self-sandboxes via Landlock on startup.
	sb := sandbox.New()
	if sb.Available() {
		if wrapErr := sb.WrapCommand(am.cmd, sandbox.Config{
			AllowedReadPaths:  []string{executable, am.workspace},
			AllowedWritePaths: []string{},
			AllowedTCPConnect: []string{am.grpcAddr, am.llmHost},
			AllowProcessSpawn: false,
		}); wrapErr != nil {
			fmt.Fprintf(os.Stderr, "warning: sandbox wrap failed: %s\n", wrapErr)
		}
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
		code, sig := agentExitInfo(err)
		am.mu.Lock()
		am.exitCode = code
		am.exitSig = sig
		am.mu.Unlock()

		if err == nil {
			am.mu.Lock()
			am.cleanExit = true
			am.mu.Unlock()
			return // Clean exit (user typed /quit).
		}

		// If stopAgent() set cleanExit before signaling, this Wait() is the
		// shutdown observer — exit silently without treating as a crash.
		am.mu.Lock()
		shuttingDown := am.cleanExit
		am.mu.Unlock()
		if shuttingDown {
			return
		}

		// Agent crashed. Check restart budget.
		now := time.Now()
		am.mu.Lock()
		am.crashes = append(am.crashes, now)

		cutoff := now.Add(-time.Duration(am.crashWindowS) * time.Second)
		recentCrashes := 0
		for _, t := range am.crashes {
			if t.After(cutoff) {
				recentCrashes++
			}
		}
		am.mu.Unlock()

		if recentCrashes >= am.crashBudget {
			fmt.Fprintf(os.Stderr, "Agent has crashed %d times in %d seconds. Giving up.\n", recentCrashes, am.crashWindowS)
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

// stopAgent sends SIGTERM to the Agent and waits up to 5 seconds for the
// monitor goroutine to observe the exit. Marks cleanExit so monitor does
// not interpret the exit as a crash and respawn.
//
// Only the monitor goroutine calls cmd.Wait(). Calling Wait() twice on the
// same cmd produces "waitid: no child processes" because the first call has
// already reaped the child — and monitor would then see that error and
// trigger a spurious restart that races with shutdown.
func (am *agentManager) stopAgent() {
	am.mu.Lock()
	cmd := am.cmd
	done := am.done
	am.cleanExit = true
	am.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)

	select {
	case <-done:
		// Monitor goroutine observed the exit and closed done.
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		// Wait briefly for monitor to reap.
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
}

// rotateAgentStderr renames the agent stderr log to .1 if it exceeds 10 MB.
// Keeps one backup; no multi-file rotation. Sufficient for panic stacks.
func rotateAgentStderr(path string) {
	const maxSize = 10 << 20 // 10 MB
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxSize {
		return
	}
	_ = os.Rename(path, path+".1")
}

// agentExitInfo extracts the exit code and signal name from a cmd.Wait()
// error. Returns ("0", "") for a nil error (clean exit). For signal kills
// returns ("-1", "SIGTERM") etc. For non-exit errors returns the error text.
func agentExitInfo(err error) (code string, sig string) {
	if err == nil {
		return "0", ""
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		ws, ok := exitErr.Sys().(syscall.WaitStatus)
		if ok {
			if ws.Signaled() {
				return "-1", ws.Signal().String()
			}
			return fmt.Sprintf("%d", ws.ExitStatus()), ""
		}
		return fmt.Sprintf("%d", exitErr.ExitCode()), ""
	}
	return err.Error(), ""
}
