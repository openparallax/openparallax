package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/openparallax/openparallax/internal/config"
	"github.com/spf13/cobra"
)

var configCmdPath string

var configCmd = &cobra.Command{
	Use:          "config [name]",
	Short:        "Show current configuration (secrets masked)",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runConfig,
}

func init() {
	configCmd.Flags().StringVarP(&configCmdPath, "config", "c", "", "path to config.yaml")
	rootCmd.AddCommand(configCmd)
}

func runConfig(_ *cobra.Command, args []string) error {
	cfgPath, err := resolveConfig(args, configCmdPath)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("cannot read config: %w", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	_ = cfg

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "api_key") || strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "token") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, "\"'")
				if len(val) > 8 {
					masked := val[:4] + "****" + val[len(val)-4:]
					fmt.Printf("%s: %s\n", parts[0], masked)
				} else {
					fmt.Printf("%s: ********\n", parts[0])
				}
				continue
			}
		}
		fmt.Println(line)
	}

	return nil
}
