package main

import (
	"fmt"
	"path/filepath"

	"github.com/openparallax/openparallax/chronicle"
	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/spf13/cobra"
)

var chronicleConfigPath string

var chronicleCmd = &cobra.Command{
	Use:          "chronicle",
	Short:        "Manage Chronicle snapshots and rollbacks",
	SilenceUsage: true,
	RunE:         runChronicleList,
}

var chronicleDiffCmd = &cobra.Command{
	Use:          "diff [snapshot-id]",
	Short:        "Show changes since a snapshot",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runChronicleDiff,
}

var chronicleVerifyCmd = &cobra.Command{
	Use:          "verify",
	Short:        "Verify snapshot integrity chain",
	SilenceUsage: true,
	RunE:         runChronicleVerify,
}

var chronicleRollbackCmd = &cobra.Command{
	Use:   "rollback <snapshot-id>",
	Short: "Restore files from a snapshot to their pre-action state",
	Long: `Rollback restores workspace files to the state captured in the given
snapshot. The snapshot ID can be found via "openparallax chronicle" (list).

This operation overwrites current file contents with the backed-up versions.
Files that were created after the snapshot are not affected — only files
that existed in the snapshot are restored.`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runChronicleRollback,
}

func init() {
	chronicleCmd.Flags().StringVarP(&chronicleConfigPath, "config", "c", "", "path to config.yaml")
	chronicleCmd.AddCommand(chronicleDiffCmd)
	chronicleCmd.AddCommand(chronicleVerifyCmd)
	chronicleCmd.AddCommand(chronicleRollbackCmd)
	rootCmd.AddCommand(chronicleCmd)
}

func openChronicle(cfgPath string) (*chronicle.Chronicle, error) {
	if cfgPath == "" {
		resolved, err := resolveConfig(nil, "")
		if err != nil {
			return nil, err
		}
		cfgPath = resolved
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	db, err := storage.Open(filepath.Join(cfg.Workspace, ".openparallax", "openparallax.db"))
	if err != nil {
		return nil, err
	}
	return chronicle.New(cfg.Workspace, cfg.Chronicle, db)
}

func runChronicleList(cmd *cobra.Command, args []string) error {
	chron, err := openChronicle(chronicleConfigPath)
	if err != nil {
		return err
	}

	snapshots := chron.List()
	if len(snapshots) == 0 {
		fmt.Println("No snapshots found.")
		return nil
	}

	for _, s := range snapshots {
		fmt.Printf("[%s] %s  %s  files: %d\n",
			s.Timestamp.Format("2006-01-02 15:04:05"),
			s.ID[:8], s.ActionSummary, len(s.FilesBackedUp))
	}
	return nil
}

func runChronicleDiff(cmd *cobra.Command, args []string) error {
	chron, err := openChronicle(chronicleConfigPath)
	if err != nil {
		return err
	}

	diff, err := chron.Diff(args[0])
	if err != nil {
		return err
	}

	if len(diff.Changes) == 0 {
		fmt.Println("No changes since snapshot.")
		return nil
	}

	for _, c := range diff.Changes {
		fmt.Printf("  %s  %s\n", c.ChangeType, c.Path)
	}
	return nil
}

func runChronicleRollback(cmd *cobra.Command, args []string) error {
	chron, err := openChronicle(chronicleConfigPath)
	if err != nil {
		return err
	}

	snapshotID := args[0]

	// Show what will be restored before doing it.
	diff, diffErr := chron.Diff(snapshotID)
	if diffErr != nil {
		return fmt.Errorf("cannot inspect snapshot: %w", diffErr)
	}

	if len(diff.Changes) == 0 {
		fmt.Println("No changes to rollback — workspace matches snapshot.")
		return nil
	}

	fmt.Printf("Rolling back %d file(s) from snapshot %s:\n", len(diff.Changes), snapshotID[:8])
	for _, c := range diff.Changes {
		fmt.Printf("  %s  %s\n", c.ChangeType, c.Path)
	}

	if rollbackErr := chron.Rollback(snapshotID); rollbackErr != nil {
		return fmt.Errorf("rollback failed: %w", rollbackErr)
	}

	fmt.Println("\n  ✓ Rollback complete.")
	return nil
}

func runChronicleVerify(cmd *cobra.Command, args []string) error {
	chron, err := openChronicle(chronicleConfigPath)
	if err != nil {
		return err
	}

	if verifyErr := chron.VerifyIntegrity(); verifyErr != nil {
		return fmt.Errorf("integrity check FAILED: %w", verifyErr)
	}
	fmt.Println("Chronicle integrity: OK")
	return nil
}
