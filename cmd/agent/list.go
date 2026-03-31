package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/registry"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Short:        "List all agents on this machine",
	SilenceUsage: true,
	RunE:         runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(_ *cobra.Command, _ []string) error {
	regPath, err := registry.DefaultPath()
	if err != nil {
		return err
	}
	reg, err := registry.Load(regPath)
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}
	if len(reg.Agents) == 0 {
		fmt.Println("No agents registered. Run 'openparallax init' to create one.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tSTATUS\tPORT\tPROVIDER\tMODEL\tWORKSPACE")
	for _, a := range reg.Agents {
		status := "stopped"
		if registry.IsRunning(a.Workspace) {
			status = "running"
		}
		provider, model := readAgentProviderModel(a.ConfigPath)
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n",
			a.Name, status, a.WebPort, provider, model, a.Workspace)
	}
	return w.Flush()
}

// readAgentProviderModel loads the agent's config to get provider and model.
func readAgentProviderModel(cfgPath string) (string, string) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return "-", "-"
	}
	return cfg.LLM.Provider, cfg.LLM.Model
}
