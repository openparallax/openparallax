package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/openparallax/openparallax/internal/config"
	"github.com/spf13/cobra"
)

var configCmdPath string

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or edit agent configuration",
}

var configShowCmd = &cobra.Command{
	Use:          "show [name]",
	Short:        "Print current configuration (secrets masked)",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runConfig,
}

var configEditCmd = &cobra.Command{
	Use:   "edit [name]",
	Short: "Open config.yaml in your default text editor",
	Long: `Opens the agent's config.yaml in your preferred text editor.

Editor resolution order:
  1. $VISUAL environment variable
  2. $EDITOR environment variable
  3. System default: nano (Linux), open (macOS), notepad (Windows)

Examples:
  openparallax config edit          # Edit the default agent's config
  openparallax config edit atlas    # Edit the "atlas" agent's config`,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runConfigEdit,
}

func init() {
	configShowCmd.Flags().StringVarP(&configCmdPath, "config", "c", "", "path to config.yaml")
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configEditCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigEdit(_ *cobra.Command, args []string) error {
	cfgPath, err := resolveConfig(args, "")
	if err != nil {
		return err
	}

	if _, statErr := os.Stat(cfgPath); statErr != nil {
		return fmt.Errorf("config file not found: %s", cfgPath)
	}

	editor := resolveEditor()
	fmt.Printf("Opening %s in %s...\n", cfgPath, editor)

	cmd := exec.Command(editor, cfgPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// resolveEditor returns the user's preferred text editor, falling back
// to a sensible system default. On Linux the fallback chain is
// $VISUAL → $EDITOR → nano. On macOS it's `open` (opens in the
// default GUI editor). On Windows it's notepad.
func resolveEditor() string {
	if v := os.Getenv("VISUAL"); v != "" {
		return v
	}
	if v := os.Getenv("EDITOR"); v != "" {
		return v
	}
	switch runtime.GOOS {
	case "darwin":
		return "open"
	case "windows":
		return "notepad"
	default:
		// Prefer nano for non-tech users; fall back to vi if nano
		// isn't installed (vi is POSIX-mandated).
		if _, err := exec.LookPath("nano"); err == nil {
			return "nano"
		}
		return "vi"
	}
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
