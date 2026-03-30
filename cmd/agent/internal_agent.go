package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/openparallax/openparallax/internal/channels/cli"
	"github.com/openparallax/openparallax/internal/sandbox"
	"github.com/spf13/cobra"
)

var (
	agentGRPCAddr  string
	agentName      string
	agentWorkspace string
)

var internalAgentCmd = &cobra.Command{
	Use:          "internal-agent",
	Short:        "Run the sandboxed agent process (internal use only)",
	Hidden:       true,
	SilenceUsage: true,
	RunE:         runInternalAgent,
}

func init() {
	internalAgentCmd.Flags().StringVar(&agentGRPCAddr, "grpc", "", "engine gRPC address")
	internalAgentCmd.Flags().StringVar(&agentName, "name", "Atlas", "agent display name")
	internalAgentCmd.Flags().StringVar(&agentWorkspace, "workspace", "", "workspace path")
	rootCmd.AddCommand(internalAgentCmd)
}

// runInternalAgent starts the sandboxed agent process. It applies kernel-level
// sandboxing before connecting to the Engine via gRPC and running the CLI TUI.
func runInternalAgent(_ *cobra.Command, _ []string) error {
	if agentGRPCAddr == "" {
		return fmt.Errorf("--grpc flag is required")
	}

	// Apply sandbox FIRST, before any untrusted operations.
	sb := sandbox.New()
	if sb.Available() {
		err := sb.ApplySelf(sandbox.Config{
			AllowedReadPaths:  []string{},
			AllowedWritePaths: []string{},
			AllowedTCPConnect: []string{agentGRPCAddr},
			AllowProcessSpawn: false,
		})
		if err != nil {
			// Log to stderr and continue — sandbox is defense in depth.
			fmt.Fprintf(os.Stderr, "sandbox: failed to apply: %s\n", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		cancel()
	}()

	// Open /dev/tty for direct terminal access. The parent process's
	// stdout is piped for PORT communication, so the agent must bypass
	// the inherited stdout and talk to the terminal directly.
	var teaOpts []tea.ProgramOption
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err == nil {
		defer func() { _ = tty.Close() }()
		teaOpts = append(teaOpts, tea.WithInput(tty), tea.WithOutput(tty))
	}

	adapter := cli.New(agentGRPCAddr, agentName, agentWorkspace)
	adapter.WithTeaOptions(teaOpts...)
	return adapter.Run(ctx)
}
