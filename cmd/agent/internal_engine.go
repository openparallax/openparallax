package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/openparallax/openparallax/internal/engine"
	"github.com/openparallax/openparallax/internal/engine/executors"
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

	// Start web server if configured.
	var webServer *web.Server
	cfg := eng.Config()
	if cfg.Web.Enabled {
		webPort := cfg.Web.Port
		if webPort == 0 {
			webPort = 3000
		}
		webServer = web.NewServer(eng, eng.Log(), webPort)
		go func() {
			if err := webServer.Start(); err != nil {
				eng.Log().Error("web_server_failed", "error", err)
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

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	if webServer != nil {
		webServer.Stop()
	}
	eng.Stop()
	return nil
}
