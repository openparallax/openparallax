package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/openparallax/openparallax/internal/registry"
	"github.com/spf13/cobra"
)

var stopConfigPath string

var stopCmd = &cobra.Command{
	Use:          "stop [name]",
	Short:        "Gracefully stop the running engine",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runStop,
}

func init() {
	stopCmd.Flags().StringVarP(&stopConfigPath, "config", "c", "", "path to config.yaml")
	rootCmd.AddCommand(stopCmd)
}

func runStop(_ *cobra.Command, args []string) error {
	workspace, agentName, err := resolveStopTarget(args)
	if err != nil {
		return err
	}

	pid, err := registry.ReadPID(workspace)
	if err != nil {
		return fmt.Errorf("no running engine found for %s", agentName)
	}

	if !isProcessAlive(pid) {
		_ = registry.RemovePID(workspace)
		return fmt.Errorf("engine PID %d is no longer running (stale PID file removed)", pid)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found: %w", err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to signal process: %w", err)
	}

	// Wait for clean shutdown.
	done := make(chan struct{})
	go func() {
		for range 100 {
			if !isProcessAlive(pid) {
				close(done)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = process.Signal(syscall.SIGKILL)
	}

	_ = registry.RemovePID(workspace)
	fmt.Printf("Stopped %s (PID %d)\n", agentName, pid)
	return nil
}

// isProcessAlive checks if a process is running via signal 0.
func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// resolveStopTarget determines the workspace and agent name from args or config.
func resolveStopTarget(args []string) (workspace, agentName string, err error) {
	if len(args) > 0 {
		regPath, regErr := registry.DefaultPath()
		if regErr != nil {
			return "", "", regErr
		}
		reg, loadErr := registry.Load(regPath)
		if loadErr != nil {
			return "", "", fmt.Errorf("load registry: %w", loadErr)
		}
		rec, ok := reg.Lookup(args[0])
		if !ok {
			return "", "", fmt.Errorf("agent %q not found", args[0])
		}
		return rec.Workspace, rec.Name, nil
	}

	// Fallback: use config path.
	cfgPath := stopConfigPath
	if cfgPath == "" {
		cfgPath = findConfig()
	}
	if cfgPath == "" {
		return "", "", fmt.Errorf("no agent specified and no workspace found")
	}
	dir := filepath.Dir(cfgPath)
	return dir, filepath.Base(dir), nil
}
