// Package main is the entry point for the openparallax agent binary.
package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:          "openparallax",
	Short:        "OpenParallax \u2014 Security-first autonomous personal AI agent",
	Long:         "OpenParallax is an autonomous personal AI agent secured by an adversarial evaluation architecture.",
	SilenceUsage: true,
}

func init() {
	rootCmd.AddCommand(getVectorExtCmd())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// findConfig looks for config.yaml in common locations, including
// agent-named workspaces under ~/.openparallax/.
func findConfig() string {
	candidates := []string{
		"config.yaml",
	}
	if home, err := os.UserHomeDir(); err == nil {
		opDir := filepath.Join(home, ".openparallax")
		candidates = append(candidates,
			filepath.Join(opDir, "workspace", "config.yaml"),
		)
		// Search agent-named workspaces (e.g. ~/.openparallax/atlas/config.yaml).
		if entries, dirErr := os.ReadDir(opDir); dirErr == nil {
			for _, e := range entries {
				if e.IsDir() {
					candidates = append(candidates,
						filepath.Join(opDir, e.Name(), "config.yaml"),
					)
				}
			}
		}
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}
