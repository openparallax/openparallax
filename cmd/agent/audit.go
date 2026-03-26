package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/openparallax/openparallax/internal/audit"
	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/spf13/cobra"
)

var (
	auditVerify     bool
	auditSessionID  string
	auditEventType  int
	auditConfigPath string
)

var auditCmd = &cobra.Command{
	Use:          "audit",
	Short:        "Query and verify the audit log",
	SilenceUsage: true,
	RunE:         runAudit,
}

func init() {
	auditCmd.Flags().BoolVar(&auditVerify, "verify", false, "verify hash chain integrity")
	auditCmd.Flags().StringVar(&auditSessionID, "session", "", "filter by session ID")
	auditCmd.Flags().IntVar(&auditEventType, "type", 0, "filter by event type")
	auditCmd.Flags().StringVarP(&auditConfigPath, "config", "c", "", "path to config.yaml")
	rootCmd.AddCommand(auditCmd)
}

func runAudit(cmd *cobra.Command, args []string) error {
	cfgPath := auditConfigPath
	if cfgPath == "" {
		cfgPath = findConfig()
	}
	if cfgPath == "" {
		return fmt.Errorf("workspace not found: run 'openparallax init' first, or use --config")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	auditPath := filepath.Join(cfg.Workspace, ".openparallax", "audit.jsonl")

	if auditVerify {
		if verifyErr := audit.VerifyIntegrity(auditPath); verifyErr != nil {
			return fmt.Errorf("integrity check FAILED: %w", verifyErr)
		}
		fmt.Println("Audit log integrity: OK")
		return nil
	}

	entries, err := audit.ReadEntries(auditPath, audit.Query{
		SessionID: auditSessionID,
		EventType: types.AuditEventType(auditEventType),
		Limit:     20,
	})
	if err != nil {
		return fmt.Errorf("failed to read audit log: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No audit entries found.")
		return nil
	}

	for _, e := range entries {
		ts := time.UnixMilli(e.Timestamp).Format("15:04:05")
		fmt.Printf("[%s] event=%d action=%s session=%s details=%s\n",
			ts, e.EventType, e.ActionType, e.SessionID, e.DetailsJSON)
	}

	return nil
}
