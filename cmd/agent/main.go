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

// findConfig looks for config.yaml in common locations.
func findConfig() string {
	candidates := []string{
		"config.yaml",
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".openparallax", "workspace", "config.yaml"),
		)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}
