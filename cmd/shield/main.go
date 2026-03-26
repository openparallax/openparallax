// Package main is the entry point for the openparallax-shield standalone binary.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "openparallax-shield",
	Short: "OpenParallax Shield \u2014 Security evaluation middleware for AI agents",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start Shield as an MCP aggregating proxy",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("The serve command is not yet available. It will be implemented in a future release.")
		return nil
	},
}

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start Shield as an HTTP-only evaluation service",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("The daemon command is not yet available. It will be implemented in a future release.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(daemonCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
