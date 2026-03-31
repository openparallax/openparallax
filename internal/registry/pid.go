package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// WritePID writes the engine PID to the workspace PID file.
func WritePID(workspace string, pid int) error {
	pidPath := pidFilePath(workspace)
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err != nil {
		return fmt.Errorf("create pid dir: %w", err)
	}
	return os.WriteFile(pidPath, []byte(strconv.Itoa(pid)+"\n"), 0o644)
}

// ReadPID reads the engine PID from the workspace PID file.
func ReadPID(workspace string) (int, error) {
	data, err := os.ReadFile(pidFilePath(workspace))
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid pid file: %w", err)
	}
	return pid, nil
}

// RemovePID deletes the engine PID file.
func RemovePID(workspace string) error {
	return os.Remove(pidFilePath(workspace))
}

// IsRunning checks if an engine process is alive for the given workspace.
// If the PID file exists but the process is dead, the stale PID file is removed.
func IsRunning(workspace string) bool {
	pid, err := ReadPID(workspace)
	if err != nil {
		return false
	}
	if !isProcessAlive(pid) {
		_ = RemovePID(workspace)
		return false
	}
	return true
}

// isProcessAlive checks if a process with the given PID is running by sending signal 0.
func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks for existence without actually sending a signal.
	return proc.Signal(syscall.Signal(0)) == nil
}

func pidFilePath(workspace string) string {
	return filepath.Join(workspace, ".openparallax", "engine.pid")
}
