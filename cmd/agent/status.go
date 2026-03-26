package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:          "status",
	Short:        "Show workspace status",
	Long:         "Displays workspace path, memory file sizes, snapshot count, session count, and audit entry count.",
	SilenceUsage: true,
	RunE:         runStatus,
}

var statusConfigPath string

func init() {
	statusCmd.Flags().StringVarP(&statusConfigPath, "config", "c", "", "path to config.yaml")
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfgPath := statusConfigPath
	if cfgPath == "" {
		cfgPath = findConfig()
	}
	if cfgPath == "" {
		return fmt.Errorf("workspace not found: run 'openparallax init' first, or use --config to specify a config file")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Workspace: %s\n\n", cfg.Workspace)

	// Memory files
	fmt.Println("Memory files:")
	for _, ft := range types.AllMemoryFiles {
		path := filepath.Join(cfg.Workspace, string(ft))
		info, err := os.Stat(path)
		if err != nil {
			fmt.Printf("  %-16s  (not found)\n", ft)
		} else {
			fmt.Printf("  %-16s  %s\n", ft, formatBytes(info.Size()))
		}
	}
	fmt.Println()

	// Database stats
	dbPath := filepath.Join(cfg.Workspace, ".openparallax", "openparallax.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		fmt.Printf("Database: error opening (%s)\n", err)
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

// findConfig looks for config.yaml in common locations.
func findConfig() string {
	// Check current directory
	candidates := []string{
		"config.yaml",
	}

	// Check home directory
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
