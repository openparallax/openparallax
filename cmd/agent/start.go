package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/registry"
	"github.com/spf13/cobra"
)

var (
	startConfigPath string
	startVerbose    bool
	startDaemon     bool
	startTUI        bool
	startPort       int
)

var startCmd = &cobra.Command{
	Use:          "start [name]",
	Short:        "Start the agent and all configured channels",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runStart,
}

func init() {
	startCmd.Flags().StringVarP(&startConfigPath, "config", "c", "", "path to config.yaml")
	startCmd.Flags().BoolVarP(&startVerbose, "verbose", "v", false, "enable verbose pipeline logging")
	startCmd.Flags().BoolVarP(&startDaemon, "daemon", "d", false, "start in background (daemon mode)")
	startCmd.Flags().BoolVar(&startTUI, "tui", false, "auto-attach TUI in foreground")
	startCmd.Flags().IntVar(&startPort, "port", 0, "override web UI port")
	rootCmd.AddCommand(startCmd)
}

func runStart(_ *cobra.Command, args []string) error {
	cfgPath := startConfigPath
	var workspace string

	// Resolve config path from agent name, --config flag, or registry.
	if cfgPath == "" && len(args) > 0 {
		regPath, err := registry.DefaultPath()
		if err != nil {
			return err
		}
		reg, err := registry.Load(regPath)
		if err != nil {
			return fmt.Errorf("load registry: %w", err)
		}
		rec, ok := reg.Lookup(args[0])
		if !ok {
			return fmt.Errorf("agent %q not found — run 'openparallax list' to see available agents", args[0])
		}
		cfgPath = rec.ConfigPath
		workspace = rec.Workspace
	}
	if cfgPath == "" {
		cfgPath = findConfig()
	}
	if cfgPath == "" {
		return fmt.Errorf("workspace not found: run 'openparallax init' first, or use --config to specify a config file")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if workspace == "" {
		workspace = cfg.Workspace
	}

	// Check if already running.
	if registry.IsRunning(workspace) {
		pid, _ := registry.ReadPID(workspace)
		webPort := cfg.Web.Port
		if webPort == 0 {
			webPort = 3000
		}
		return fmt.Errorf("agent is already running (PID %d) on port %d", pid, webPort)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	pm := newProcessManager(cfgPath, startVerbose)
	port, webStatus, err := pm.startEngine(ctx)
	if err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}

	// Write PID file for the engine process.
	if pm.cmd != nil && pm.cmd.Process != nil {
		_ = registry.WritePID(workspace, pm.cmd.Process.Pid)
	}

	grpcAddr := fmt.Sprintf("localhost:%d", port)
	fmt.Printf("Engine started on %s (LLM: %s/%s)\n", grpcAddr, cfg.LLM.Provider, cfg.LLM.Model)
	if webStatus.ok {
		fmt.Printf("Web UI available at http://127.0.0.1:%d\n", webStatus.port)
	} else if webStatus.err != "" {
		fmt.Printf("Warning: Web UI failed to start on port %d: %s\n", webStatus.port, webStatus.err)
	}
	if startVerbose {
		logPath := filepath.Join(cfg.Workspace, ".openparallax", "engine.log")
		fmt.Printf("Verbose log: %s\n", logPath)
	}

	// Daemon mode: print info and exit immediately.
	if startDaemon {
		fmt.Println("Running in background.")
		return nil
	}

	// Auto-attach TUI if requested.
	if startTUI {
		go func() {
			grpcAddr := fmt.Sprintf("localhost:%d", port)
			name := cfg.Identity.Name
			if name == "" {
				name = "Atlas"
			}
			if tuiErr := attachTUI(ctx, grpcAddr, name, workspace); tuiErr != nil {
				fmt.Fprintf(os.Stderr, "TUI exited: %s\n", tuiErr)
			}
			// When TUI exits (user quit), shut down if no daemon mode.
			sigCh <- syscall.SIGTERM
		}()
	}

	// Foreground mode: wait for engine to exit or signal.
	select {
	case <-pm.done:
		_ = registry.RemovePID(workspace)
		return nil
	case <-sigCh:
		if !startTUI {
			fmt.Println("\nShutting down...")
		}
		cancel()
		pm.stopEngine()
		_ = registry.RemovePID(workspace)
		return nil
	}
}

// processManager spawns and supervises the engine child process.
type processManager struct {
	configPath string
	verbose    bool
	cmd        *exec.Cmd
	logFile    *os.File
	done       chan struct{}
	mu         sync.Mutex
	crashes    []time.Time
}

func newProcessManager(configPath string, verbose bool) *processManager {
	return &processManager{
		configPath: configPath,
		verbose:    verbose,
		done:       make(chan struct{}),
	}
}

// webStatus holds the web server startup result reported by the engine.
type webStatus struct {
	ok   bool
	port int
	err  string
}

// startEngine spawns the engine process and returns the gRPC port and web
// server status. If the engine crashes, it is restarted (max 5 in 60s).
func (pm *processManager) startEngine(ctx context.Context) (int, webStatus, error) {
	port, ws, err := pm.spawnEngine(ctx)
	if err != nil {
		return 0, webStatus{}, err
	}

	// Monitor for crashes in the background.
	go pm.monitor(ctx)

	return port, ws, nil
}

// engineStartResult carries both gRPC port and web server status from stdout.
type engineStartResult struct {
	grpcPort int
	web      webStatus
}

// spawnEngine starts one engine process and reads status from its stdout.
// The engine writes "PORT:<grpc_port>\n" followed by "WEB:<port>\n" or
// "WEB_FAILED:<port>:<error>\n".
func (pm *processManager) spawnEngine(ctx context.Context) (int, webStatus, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	executable, err := os.Executable()
	if err != nil {
		return 0, webStatus{}, fmt.Errorf("cannot find own executable: %w", err)
	}

	cmdArgs := []string{"internal-engine", "--config", pm.configPath}
	if pm.verbose {
		cmdArgs = append(cmdArgs, "--verbose")
	}
	pm.cmd = exec.CommandContext(ctx, executable, cmdArgs...)

	// Daemon mode: detach from terminal.
	if startDaemon {
		pm.cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}

	if pm.verbose {
		logPath := filepath.Join(filepath.Dir(pm.configPath), ".openparallax", "engine.log")
		logFile, logErr := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if logErr == nil {
			pm.cmd.Stderr = logFile
			pm.logFile = logFile
		}
	}

	stdout, err := pm.cmd.StdoutPipe()
	if err != nil {
		return 0, webStatus{}, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := pm.cmd.Start(); err != nil {
		return 0, webStatus{}, fmt.Errorf("engine spawn: %w", err)
	}

	// Read startup status from stdout. The engine writes:
	//   PORT:<grpc_port>
	//   WEB:<web_port>       (success)
	//   WEB_FAILED:<port>:<error>  (failure)
	scanner := bufio.NewScanner(stdout)
	resultCh := make(chan engineStartResult, 1)
	errCh := make(chan error, 1)
	go func() {
		var result engineStartResult
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "PORT:"):
				p, parseErr := strconv.Atoi(strings.TrimPrefix(line, "PORT:"))
				if parseErr != nil {
					errCh <- fmt.Errorf("invalid port from engine: %s", line)
					return
				}
				result.grpcPort = p
			case strings.HasPrefix(line, "WEB:"):
				p, _ := strconv.Atoi(strings.TrimPrefix(line, "WEB:"))
				result.web = webStatus{ok: true, port: p}
				resultCh <- result
				return
			case strings.HasPrefix(line, "WEB_FAILED:"):
				parts := strings.SplitN(strings.TrimPrefix(line, "WEB_FAILED:"), ":", 2)
				p, _ := strconv.Atoi(parts[0])
				msg := ""
				if len(parts) > 1 {
					msg = parts[1]
				}
				result.web = webStatus{ok: false, port: p, err: msg}
				resultCh <- result
				return
			case strings.HasPrefix(line, "WEB_DISABLED"):
				resultCh <- result
				return
			}
		}
		// Engine exited before reporting web status.
		if result.grpcPort > 0 {
			resultCh <- result
		} else {
			errCh <- fmt.Errorf("engine process exited without reporting port")
		}
	}()

	select {
	case r := <-resultCh:
		return r.grpcPort, r.web, nil
	case err := <-errCh:
		_ = pm.cmd.Process.Kill()
		return 0, webStatus{}, err
	case <-time.After(30 * time.Second):
		_ = pm.cmd.Process.Kill()
		return 0, webStatus{}, fmt.Errorf("engine did not start within 30 seconds")
	}
}

// monitor watches the engine process and restarts it on crash.
func (pm *processManager) monitor(ctx context.Context) {
	defer close(pm.done)

	for {
		pm.mu.Lock()
		cmd := pm.cmd
		pm.mu.Unlock()

		if cmd == nil || cmd.Process == nil {
			return
		}

		err := cmd.Wait()
		if ctx.Err() != nil {
			return // Context cancelled — intentional shutdown.
		}
		if err == nil {
			return // Clean exit.
		}

		// Exit code 75 = restart requested (not a crash).
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 75 {
			fmt.Fprintln(os.Stderr, "Engine restart requested. Restarting...")
			if _, _, spawnErr := pm.spawnEngine(ctx); spawnErr != nil {
				fmt.Fprintf(os.Stderr, "Failed to restart engine: %s\n", spawnErr)
				return
			}
			continue
		}

		// Engine crashed. Check restart budget.
		now := time.Now()
		pm.mu.Lock()
		pm.crashes = append(pm.crashes, now)

		// Count crashes in the last 60 seconds.
		cutoff := now.Add(-60 * time.Second)
		recentCrashes := 0
		for _, t := range pm.crashes {
			if t.After(cutoff) {
				recentCrashes++
			}
		}
		pm.mu.Unlock()

		if recentCrashes >= 5 {
			fmt.Fprintf(os.Stderr, "Engine has crashed %d times in 60 seconds. Giving up.\n", recentCrashes)
			return
		}

		fmt.Fprintf(os.Stderr, "Engine crashed (%s). Restarting in 1 second...\n", err)
		time.Sleep(time.Second)

		if _, _, spawnErr := pm.spawnEngine(ctx); spawnErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to restart engine: %s\n", spawnErr)
			return
		}
	}
}

// stopEngine sends SIGTERM to the engine process and waits.
func (pm *processManager) stopEngine() {
	pm.mu.Lock()
	cmd := pm.cmd
	pm.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
	}

	if pm.logFile != nil {
		_ = pm.logFile.Close()
	}
}
