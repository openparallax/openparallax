package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
)

var stopConfigPath string

var stopCmd = &cobra.Command{
	Use:          "stop",
	Short:        "Gracefully stop the running engine",
	SilenceUsage: true,
	RunE:         runStop,
}

func init() {
	stopCmd.Flags().StringVarP(&stopConfigPath, "config", "c", "", "path to config.yaml")
	rootCmd.AddCommand(stopCmd)
}

func runStop(_ *cobra.Command, _ []string) error {
	pidFile := findPidFile()
	if pidFile == "" {
		return fmt.Errorf("no running engine found")
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("cannot read pid file: %w", err)
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return fmt.Errorf("invalid pid file: %w", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found: %w", err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to signal process: %w", err)
	}

	fmt.Printf("Sent stop signal to engine (PID %d)\n", pid)
	_ = os.Remove(pidFile)
	return nil
}

func findPidFile() string {
	cfgPath := stopConfigPath
	if cfgPath == "" {
		cfgPath = findConfig()
	}
	if cfgPath == "" {
		return ""
	}
	dir := filepath.Dir(cfgPath)
	pidPath := filepath.Join(dir, ".openparallax", "engine.pid")
	if _, err := os.Stat(pidPath); err == nil {
		return pidPath
	}
	return ""
}
