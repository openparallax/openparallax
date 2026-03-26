// Package main is the entry point for the openparallax agent binary.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "openparallax",
	Short: "OpenParallax \u2014 Security-first autonomous personal AI agent",
	Long:  "OpenParallax is an autonomous personal AI agent secured by an adversarial evaluation architecture.",
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the agent and all configured channels",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("The start command is not yet available. It will be implemented in a future release.")
		return nil
	},
}

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive CLI chat session",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("The chat command is not yet available. It will be implemented in a future release.")
		return nil
	},
}

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Query and verify the audit log",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("The audit command is not yet available. It will be implemented in a future release.")
		return nil
	},
}

var chronicleCmd = &cobra.Command{
	Use:   "chronicle",
	Short: "Manage Chronicle snapshots and rollbacks",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("The chronicle command is not yet available. It will be implemented in a future release.")
		return nil
	},
}

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "List loaded skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("The skills command is not yet available. It will be implemented in a future release.")
		return nil
	},
}

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "View and search workspace memory",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("The memory command is not yet available. It will be implemented in a future release.")
		return nil
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View and modify configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("The config command is not yet available. It will be implemented in a future release.")
		return nil
	},
}

var getClassifierCmd = &cobra.Command{
	Use:   "get-classifier",
	Short: "Download the ONNX classifier model and sidecar binary",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("The get-classifier command is not yet available. It will be implemented in a future release.")
		return nil
	},
}

// Hidden internal commands used by the process manager to spawn child processes.
var internalAgentCmd = &cobra.Command{
	Use:    "internal-agent",
	Short:  "Run the agent process (internal use only)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Internal agent process is not yet available.")
		return nil
	},
}

var internalEngineCmd = &cobra.Command{
	Use:    "internal-engine",
	Short:  "Run the engine process (internal use only)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Internal engine process is not yet available.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(chatCmd)
	rootCmd.AddCommand(auditCmd)
	rootCmd.AddCommand(chronicleCmd)
	rootCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(memoryCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(getClassifierCmd)
	rootCmd.AddCommand(internalAgentCmd)
	rootCmd.AddCommand(internalEngineCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
