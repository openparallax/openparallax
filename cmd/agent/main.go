// Package main is the entry point for the openparallax agent binary.
package main

import (
	"os"
	"path/filepath"

	"github.com/openparallax/openparallax/internal/registry"
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

// findConfig looks for config.yaml in common locations. It checks the global
// agent registry first (for single-agent convenience), then falls back to
// scanning known directories.
func findConfig() string {
	// Check the global agent registry first.
	if regPath, err := registry.DefaultPath(); err == nil {
		if reg, loadErr := registry.Load(regPath); loadErr == nil && len(reg.Agents) > 0 {
			if rec, findErr := reg.FindSingle(); findErr == nil {
				if _, statErr := os.Stat(rec.ConfigPath); statErr == nil {
					return rec.ConfigPath
				}
			}
		}
	}

	// Fallback: scan known directories.
	candidates := []string{
		"config.yaml",
	}
	if home, err := os.UserHomeDir(); err == nil {
		opDir := filepath.Join(home, ".openparallax")

		// Trigger migration if agents.json is missing but workspaces exist.
		if regPath, regErr := registry.DefaultPath(); regErr == nil {
			if _, statErr := os.Stat(regPath); os.IsNotExist(statErr) {
				_ = registry.Migrate(regPath)
				// Retry registry lookup after migration.
				if reg, loadErr := registry.Load(regPath); loadErr == nil {
					if rec, findErr := reg.FindSingle(); findErr == nil {
						if _, cfgErr := os.Stat(rec.ConfigPath); cfgErr == nil {
							return rec.ConfigPath
						}
					}
				}
			}
		}

		candidates = append(candidates,
			filepath.Join(opDir, "workspace", "config.yaml"),
		)
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
