// Package main is the entry point for the openparallax-shield standalone binary.
package main

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "openparallax-shield",
	Short: "OpenParallax Shield \u2014 Security evaluation middleware for AI agents",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
