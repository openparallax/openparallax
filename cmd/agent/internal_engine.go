package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/openparallax/openparallax/internal/engine"
	"github.com/spf13/cobra"
)

var engineConfigPath string

var internalEngineCmd = &cobra.Command{
	Use:          "internal-engine",
	Short:        "Run the engine process (internal use only)",
	Hidden:       true,
	SilenceUsage: true,
	RunE:         runInternalEngine,
}

func init() {
	internalEngineCmd.Flags().StringVar(&engineConfigPath, "config", "", "path to config.yaml")
	rootCmd.AddCommand(internalEngineCmd)
}

// runInternalEngine starts the engine gRPC server and blocks until signaled.
// It writes the listening port to stdout so the parent process can read it.
func runInternalEngine(cmd *cobra.Command, args []string) error {
	if engineConfigPath == "" {
		return fmt.Errorf("--config flag is required")
	}

	eng, err := engine.New(engineConfigPath)
	if err != nil {
		return fmt.Errorf("engine init failed: %w", err)
	}

	port, err := eng.Start()
	if err != nil {
		return fmt.Errorf("engine start failed: %w", err)
	}

	// Write port to stdout so the parent process can read it.
	// This is the only stdout output — the parent reads one line.
	_, _ = fmt.Fprintf(os.Stdout, "PORT:%d\n", port)

	// Block until SIGTERM or SIGINT.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	eng.Stop()
	return nil
}
