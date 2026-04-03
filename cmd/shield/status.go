package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display Shield configuration and readiness",
	RunE:  runStatus,
}

var statusConfig string

func init() {
	statusCmd.Flags().StringVar(&statusConfig, "config", "shield.yaml", "Path to shield.yaml")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(_ *cobra.Command, _ []string) error {
	cfg, err := loadShieldConfig(statusConfig)
	if err != nil {
		return err
	}

	status := map[string]any{
		"listen":               cfg.Listen,
		"policy_file":          cfg.Policy.File,
		"classifier_threshold": cfg.Classifier.Threshold,
		"heuristic_enabled":    cfg.Heuristic.Enabled,
		"evaluator_configured": cfg.Evaluator != nil && cfg.Evaluator.Provider != "",
		"fail_closed":          cfg.FailClosed,
		"rate_limit":           cfg.RateLimit,
		"daily_budget":         cfg.DailyBudget,
		"verdict_ttl_seconds":  cfg.VerdictTTL,
		"audit_file":           cfg.Audit.File,
	}

	out, _ := json.MarshalIndent(status, "", "  ")
	_, _ = fmt.Fprintln(os.Stdout, string(out))
	return nil
}
