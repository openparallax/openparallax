package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var detachCmd = &cobra.Command{
	Use:          "detach <channel> [name]",
	Short:        "Detach a channel from a running agent",
	Args:         cobra.RangeArgs(1, 2),
	SilenceUsage: true,
	RunE:         runDetach,
}

func init() {
	rootCmd.AddCommand(detachCmd)
}

func runDetach(_ *cobra.Command, args []string) error {
	channel := args[0]
	return fmt.Errorf("detach %q: not yet implemented — use Ctrl+C in the channel process", channel)
}
