package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/registry"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:          "status [name]",
	Short:        "Show workspace status",
	Long:         "Displays workspace path, memory file sizes, snapshot count, session count, and audit entry count.\nPass an agent name to show a specific agent, or omit for the default agent.",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runStatus,
}

var statusConfigPath string

func init() {
	statusCmd.Flags().StringVarP(&statusConfigPath, "config", "c", "", "path to config.yaml")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(_ *cobra.Command, args []string) error {
	cfgPath, err := resolveStatusConfig(args)
	if err != nil {
		return err
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	agentName := cfg.Identity.Name
	if agentName == "" {
		agentName = "Atlas"
	}

	running := registry.IsRunning(cfg.Workspace)
	statusText := "stopped"
	if running {
		statusText = "running"
	}

	fmt.Printf("Agent:     %s (%s)\n", agentName, statusText)
	fmt.Printf("Workspace: %s\n", cfg.Workspace)
	chat, _ := cfg.ChatModel()
	fmt.Printf("Provider:  %s / %s\n", chat.Provider, chat.Model)
	if cfg.Web.Enabled {
		webPort := cfg.Web.Port
		if webPort == 0 {
			webPort = 3000
		}
		fmt.Printf("Web UI:    http://127.0.0.1:%d\n", webPort)
	}
	fmt.Println()

	// Memory files
	fmt.Println("Memory files:")
	for _, ft := range types.AllMemoryFiles {
		path := filepath.Join(cfg.Workspace, string(ft))
		info, statErr := os.Stat(path)
		if statErr != nil {
			fmt.Printf("  %-16s  (not found)\n", ft)
		} else {
			fmt.Printf("  %-16s  %s\n", ft, formatBytes(info.Size()))
		}
	}
	fmt.Println()

	// Database stats
	dbPath := filepath.Join(cfg.Workspace, ".openparallax", "openparallax.db")
	db, dbErr := storage.Open(dbPath)
	if dbErr != nil {
		fmt.Printf("Database: error opening (%s)\n", dbErr)
		return nil
	}
	defer func() { _ = db.Close() }()

	sessionCount, _ := db.SessionCount()
	snapshotCount, _ := db.SnapshotCount()
	auditCount, _ := db.AuditEntryCount()

	fmt.Printf("Sessions:   %d\n", sessionCount)
	fmt.Printf("Snapshots:  %d\n", snapshotCount)
	fmt.Printf("Audit logs: %d\n", auditCount)

	// Database file size
	if info, statErr := os.Stat(dbPath); statErr == nil {
		fmt.Printf("DB size:    %s\n", formatBytes(info.Size()))
	}

	return nil
}

// formatBytes converts a byte count to a human-readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// resolveStatusConfig finds the config path from args, flags, or auto-detection.
func resolveStatusConfig(args []string) (string, error) {
	if len(args) > 0 {
		regPath, err := registry.DefaultPath()
		if err != nil {
			return "", err
		}
		reg, err := registry.Load(regPath)
		if err != nil {
			return "", fmt.Errorf("load registry: %w", err)
		}
		rec, ok := reg.Lookup(args[0])
		if !ok {
			return "", fmt.Errorf("agent %q not found", args[0])
		}
		return rec.ConfigPath, nil
	}

	cfgPath := statusConfigPath
	if cfgPath == "" {
		cfgPath = findConfig()
	}
	if cfgPath == "" {
		return "", fmt.Errorf("workspace not found: run 'openparallax init' first, or use --config to specify a config file")
	}
	return cfgPath, nil
}
