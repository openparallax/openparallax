package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var restartConfigPath string

var restartCmd = &cobra.Command{
	Use:          "restart [name]",
	Short:        "Restart the engine (stop then start)",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runRestart,
}

func init() {
	restartCmd.Flags().StringVarP(&restartConfigPath, "config", "c", "", "path to config.yaml")
	rootCmd.AddCommand(restartCmd)
}

func runRestart(_ *cobra.Command, args []string) error {
	cfgPath, err := resolveConfig(args, restartConfigPath)
	if err != nil {
		return err
	}

	// Stop existing engine if running.
	stopConfigPath = cfgPath
	_ = runStop(nil, nil)

	// Start fresh.
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find executable: %w", err)
	}

	cmd := exec.Command(executable, "start", "-c", cfgPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	fmt.Println("Restarting engine...")
	return cmd.Run()
}
