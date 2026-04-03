package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/openparallax/openparallax/shield"
	"github.com/spf13/cobra"
)

var evaluateCmd = &cobra.Command{
	Use:   "evaluate",
	Short: "Evaluate a single action against the Shield pipeline",
	RunE:  runEvaluate,
}

var (
	evalAction  string
	evalPath    string
	evalContent string
	evalConfig  string
)

func init() {
	evaluateCmd.Flags().StringVar(&evalAction, "action", "", "Action type (e.g., read_file, execute_command)")
	evaluateCmd.Flags().StringVar(&evalPath, "path", "", "File path for file actions")
	evaluateCmd.Flags().StringVar(&evalContent, "content", "", "Content or command text")
	evaluateCmd.Flags().StringVar(&evalConfig, "config", "shield.yaml", "Path to shield.yaml")
	_ = evaluateCmd.MarkFlagRequired("action")
	rootCmd.AddCommand(evaluateCmd)
}

func runEvaluate(_ *cobra.Command, _ []string) error {
	cfg, err := loadShieldConfig(evalConfig)
	if err != nil {
		return err
	}

	pipeline, err := shield.NewPipeline(cfg.toPipelineConfig())
	if err != nil {
		return fmt.Errorf("pipeline init: %w", err)
	}

	payload := map[string]any{}
	if evalPath != "" {
		payload["path"] = evalPath
	}
	if evalContent != "" {
		payload["command"] = evalContent
	}

	action := &shield.ActionRequest{
		Type:      shield.ActionType(evalAction),
		Payload:   payload,
		Timestamp: time.Now(),
	}

	verdict := pipeline.Evaluate(context.Background(), action)

	out, _ := json.MarshalIndent(verdict, "", "  ")
	fmt.Fprintln(os.Stdout, string(out))

	if verdict.Decision == shield.VerdictBlock {
		os.Exit(1)
	}
	return nil
}
