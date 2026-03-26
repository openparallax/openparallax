package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/openparallax/openparallax/internal/channels/cli"
	"github.com/openparallax/openparallax/internal/config"
	"github.com/spf13/cobra"
)

var startConfigPath string

var startCmd = &cobra.Command{
	Use:          "start",
	Short:        "Start the agent and all configured channels",
	SilenceUsage: true,
	RunE:         runStart,
}

func init() {
	startCmd.Flags().StringVarP(&startConfigPath, "config", "c", "", "path to config.yaml")
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	cfgPath := startConfigPath
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// Spawn the engine process.
	pm := newProcessManager(cfgPath)
	port, err := pm.startEngine(ctx)
	if err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}

	grpcAddr := fmt.Sprintf("localhost:%d", port)
	fmt.Printf("Engine started on %s (LLM: %s/%s)\n", grpcAddr, cfg.LLM.Provider, cfg.LLM.Model)

	// Start the CLI adapter.
	agentName := cfg.Identity.Name
	if agentName == "" {
		agentName = "Atlas"
	}

	adapter := cli.New(grpcAddr, agentName, cfg.Workspace)

	// Run CLI in a goroutine so we can handle signals.
	cliDone := make(chan error, 1)
	go func() {
		cliDone <- adapter.Run(ctx)
	}()

	select {
	case err := <-cliDone:
		// CLI exited (user typed /quit or error).
		cancel()
		pm.stopEngine()
		return err
	case <-sigCh:
		// Received SIGTERM/SIGINT.
		fmt.Println("\nShutting down...")
		cancel()
		pm.stopEngine()
		return nil
	}
}

// processManager spawns and supervises the engine child process.
type processManager struct {
	configPath string
	cmd        *exec.Cmd
	mu         sync.Mutex
	crashes    []time.Time
}

func newProcessManager(configPath string) *processManager {
	return &processManager{configPath: configPath}
}

// startEngine spawns the engine process and returns the port it's listening on.
// If the engine crashes, it is restarted (max 5 times in 60 seconds).
func (pm *processManager) startEngine(ctx context.Context) (int, error) {
	port, err := pm.spawnEngine(ctx)
	if err != nil {
		return 0, err
	}

	// Monitor for crashes in the background.
	go pm.monitor(ctx)

	return port, nil
}

// spawnEngine starts one engine process and reads the port from its stdout.
func (pm *processManager) spawnEngine(ctx context.Context) (int, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	executable, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("cannot find own executable: %w", err)
	}

	pm.cmd = exec.CommandContext(ctx, executable, "internal-engine", "--config", pm.configPath)
	pm.cmd.Stderr = os.Stderr

	stdout, err := pm.cmd.StdoutPipe()
	if err != nil {
		return 0, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := pm.cmd.Start(); err != nil {
		return 0, fmt.Errorf("engine spawn: %w", err)
	}

	// Read the port from stdout. The engine writes "PORT:<port>\n".
	scanner := bufio.NewScanner(stdout)
	portCh := make(chan int, 1)
	errCh := make(chan error, 1)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "PORT:") {
				p, parseErr := strconv.Atoi(strings.TrimPrefix(line, "PORT:"))
				if parseErr != nil {
					errCh <- fmt.Errorf("invalid port from engine: %s", line)
					return
				}
				portCh <- p
				return
			}
		}
		errCh <- fmt.Errorf("engine process exited without reporting port")
	}()

	select {
	case port := <-portCh:
		return port, nil
	case err := <-errCh:
		_ = pm.cmd.Process.Kill()
		return 0, err
	case <-time.After(30 * time.Second):
		_ = pm.cmd.Process.Kill()
		return 0, fmt.Errorf("engine did not start within 30 seconds")
	}
}

// monitor watches the engine process and restarts it on crash.
func (pm *processManager) monitor(ctx context.Context) {
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

		if _, spawnErr := pm.spawnEngine(ctx); spawnErr != nil {
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
}
