package main

import (
	"fmt"
	"path/filepath"

	"github.com/openparallax/openparallax/audit"
	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/spf13/cobra"
)

var ifcConfigPath string

var ifcCmd = &cobra.Command{
	Use:   "ifc",
	Short: "Manage Information Flow Control",
	Long:  `Manage the IFC activity table — view tracked file classifications and sweep stale entries.`,
}

var ifcListCmd = &cobra.Command{
	Use:          "list [name]",
	Short:        "List all IFC-tracked file paths",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runIFCList,
}

var ifcSweepCmd = &cobra.Command{
	Use:          "sweep [name]",
	Short:        "Remove IFC classifications for deleted files",
	Long:         `Sweep the IFC activity table, removing entries where the file no longer exists on disk. To release a classification, delete the file and run this command.`,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runIFCSweep,
}

func init() {
	ifcListCmd.Flags().StringVarP(&ifcConfigPath, "config", "c", "", "path to config.yaml")
	ifcSweepCmd.Flags().StringVarP(&ifcConfigPath, "config", "c", "", "path to config.yaml")
	ifcCmd.AddCommand(ifcListCmd)
	ifcCmd.AddCommand(ifcSweepCmd)
	rootCmd.AddCommand(ifcCmd)
}

func runIFCList(_ *cobra.Command, args []string) error {
	cfgPath, err := resolveConfig(args, ifcConfigPath)
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	db, err := storage.Open(filepath.Join(cfg.Workspace, ".openparallax", "openparallax.db"))
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	entries, err := db.ListIFCActivity()
	if err != nil {
		return fmt.Errorf("list ifc activity: %w", err)
	}
	if len(entries) == 0 {
		fmt.Println("No IFC-tracked paths.")
		return nil
	}

	fmt.Printf("IFC-tracked paths (%d):\n\n", len(entries))
	for _, e := range entries {
		fmt.Printf("  %-12s  %s\n", sensitivityName(e.Sensitivity), e.Path)
		fmt.Printf("  %-12s  sourced from %s (%s)\n", "", e.SourcePath, e.CreatedAt)
	}
	return nil
}

func runIFCSweep(_ *cobra.Command, args []string) error {
	cfgPath, err := resolveConfig(args, ifcConfigPath)
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	db, err := storage.Open(filepath.Join(cfg.Workspace, ".openparallax", "openparallax.db"))
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	removed, err := db.SweepIFCActivity(cfg.Workspace)
	if err != nil {
		return fmt.Errorf("sweep ifc activity: %w", err)
	}

	if len(removed) == 0 {
		fmt.Println("No stale entries found.")
		return nil
	}

	fmt.Printf("Removed %d stale entries:\n", len(removed))
	for _, e := range removed {
		fmt.Printf("  %s (was: %s, tagged %s)\n", e.Path, sensitivityName(e.Sensitivity), e.CreatedAt)
	}

	// Audit log the sweep.
	auditPath := filepath.Join(cfg.Workspace, ".openparallax", "audit.jsonl")
	logger, logErr := audit.NewLogger(auditPath)
	if logErr == nil {
		logger.SetDB(db)
		_ = logger.Log(audit.Entry{
			EventType: types.AuditIFCSweep,
			Details:   fmt.Sprintf("removed %d stale entries", len(removed)),
		})
	}

	return nil
}

func sensitivityName(level int) string {
	switch level {
	case 0:
		return "public"
	case 1:
		return "internal"
	case 2:
		return "confidential"
	case 3:
		return "restricted"
	case 4:
		return "critical"
	default:
		return fmt.Sprintf("level-%d", level)
	}
}
