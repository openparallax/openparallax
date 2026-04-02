package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/openparallax/openparallax/internal/channels/cli"
	"github.com/openparallax/openparallax/internal/registry"
	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:   "attach <channel> [name]",
	Short: "Attach a channel to a running agent",
	Long: `Attach a UI channel to a running agent. Supported channels:
  tui        Interactive terminal UI
  telegram   Telegram bot
  discord    Discord bot
  signal     Signal messenger`,
	Args:         cobra.RangeArgs(1, 2),
	SilenceUsage: true,
	RunE:         runAttach,
}

func init() {
	rootCmd.AddCommand(attachCmd)
}

func runAttach(_ *cobra.Command, args []string) error {
	channel := args[0]

	// Resolve agent.
	regPath, err := registry.DefaultPath()
	if err != nil {
		return err
	}
	reg, err := registry.Load(regPath)
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	var rec registry.AgentRecord
	if len(args) > 1 {
		r, ok := reg.Lookup(args[1])
		if !ok {
			return fmt.Errorf("agent %q not found", args[1])
		}
		rec = r
	} else {
		r, err := reg.FindSingle()
		if err != nil {
			return err
		}
		rec = r
	}

	if !registry.IsRunning(rec.Workspace) {
		return fmt.Errorf("agent %q is not running — start it first: openparallax start %s", rec.Name, rec.Slug)
	}

	grpcAddr := fmt.Sprintf("localhost:%d", rec.GRPCPort)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		cancel()
	}()

	switch channel {
	case "tui":
		return attachTUI(ctx, grpcAddr, rec.Name, rec.Workspace)
	default:
		return fmt.Errorf("unsupported channel: %s (supported: tui)", channel)
	}
}

// attachTUI starts the interactive terminal UI connected to a running engine.
func attachTUI(ctx context.Context, grpcAddr, agentName, workspace string) error {
	adapter := cli.New(grpcAddr, agentName, workspace)
	return adapter.Run(ctx)
}
