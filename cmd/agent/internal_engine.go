package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/openparallax/openparallax/internal/engine"
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

// runInternalEngine starts the engine gRPC server and blocks until signaled.
// It writes the listening port to stdout so the parent process can read it.
func runInternalEngine(cmd *cobra.Command, args []string) error {
	if engineConfigPath == "" {
		return fmt.Errorf("--config flag is required")
	}

	eng, err := engine.New(engineConfigPath, engineVerbose)
	if err != nil {
		return fmt.Errorf("engine init failed: %w", err)
	}

	port, err := eng.Start()
	if err != nil {
		return fmt.Errorf("engine start failed: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "PORT:%d\n", port)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	eng.Stop()
	return nil
}
