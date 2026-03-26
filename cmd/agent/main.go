// Package main is the entry point for the openparallax agent binary.
package main

import (
	"fmt"
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

func stubCommand(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:          use,
		Short:        short,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("The %s command is not yet available. It will be implemented in a future release.\n", use)
			return nil
		},
	}
}

func init() {
	rootCmd.AddCommand(stubCommand("chat", "Start an interactive CLI chat session"))
	rootCmd.AddCommand(stubCommand("chronicle", "Manage Chronicle snapshots and rollbacks"))
	rootCmd.AddCommand(stubCommand("skills", "List loaded skills"))
	rootCmd.AddCommand(stubCommand("memory", "View and search workspace memory"))
	rootCmd.AddCommand(stubCommand("config", "View and modify configuration"))
	rootCmd.AddCommand(stubCommand("get-classifier", "Download the ONNX classifier model and sidecar binary"))
	rootCmd.AddCommand(&cobra.Command{
		Use:          "internal-agent",
		Short:        "Run the agent process (internal use only)",
		Hidden:       true,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Internal agent process is not yet available.")
			return nil
		},
	})
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
