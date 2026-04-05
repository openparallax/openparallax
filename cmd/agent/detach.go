package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/openparallax/openparallax/internal/registry"
	"github.com/spf13/cobra"
)

var detachCmd = &cobra.Command{
	Use:   "detach <channel> [name]",
	Short: "Detach a channel from a running agent",
	Long: `Detach a running channel adapter from a running agent.

The channel is gracefully stopped and removed. The agent continues
running with the remaining channels.

Examples:
  openparallax detach telegram
  openparallax detach discord myagent`,
	Args:         cobra.RangeArgs(1, 2),
	SilenceUsage: true,
	RunE:         runDetach,
}

func init() {
	rootCmd.AddCommand(detachCmd)
}

func runDetach(_ *cobra.Command, args []string) error {
	channel := args[0]

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
		r, findErr := reg.FindSingle()
		if findErr != nil {
			return findErr
		}
		rec = r
	}

	if !registry.IsRunning(rec.Workspace) {
		return fmt.Errorf("agent %q is not running", rec.Name)
	}

	url := fmt.Sprintf("http://localhost:%d/api/channels/detach", rec.WebPort)
	body, _ := json.Marshal(map[string]string{"channel": channel})

	resp, err := http.Post(url, "application/json", bytes.NewReader(body)) //nolint:gosec // localhost only
	if err != nil {
		return fmt.Errorf("cannot reach agent: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result map[string]string
	if decErr := json.NewDecoder(resp.Body).Decode(&result); decErr != nil {
		return fmt.Errorf("invalid response: %w", decErr)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("detach failed: %s", result["error"])
	}

	fmt.Printf("  ✓ Detached %s from %s\n", channel, rec.Name)
	return nil
}
