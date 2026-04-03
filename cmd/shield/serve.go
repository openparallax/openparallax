package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openparallax/openparallax/audit"
	"github.com/openparallax/openparallax/shield"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Shield HTTP evaluation server",
	RunE:  runServe,
}

var configFile string

func init() {
	serveCmd.Flags().StringVarP(&configFile, "config", "c", "shield.yaml", "Path to shield.yaml config file")
	rootCmd.AddCommand(serveCmd)
}

func runServe(_ *cobra.Command, _ []string) error {
	cfg, err := loadShieldConfig(configFile)
	if err != nil {
		return err
	}

	pipeline, err := shield.NewPipeline(cfg.toPipelineConfig())
	if err != nil {
		return fmt.Errorf("pipeline init: %w", err)
	}

	var auditLog *audit.Logger
	if cfg.Audit.File != "" {
		auditLog, err = audit.NewLogger(cfg.Audit.File)
		if err != nil {
			return fmt.Errorf("audit init: %w", err)
		}
		defer func() { _ = auditLog.Close() }()
	}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /evaluate", func(w http.ResponseWriter, r *http.Request) {
		var action shield.ActionRequest
		if decErr := json.NewDecoder(r.Body).Decode(&action); decErr != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if action.Timestamp.IsZero() {
			action.Timestamp = time.Now()
		}

		verdict := pipeline.Evaluate(r.Context(), &action)

		if auditLog != nil {
			details, _ := json.Marshal(map[string]any{
				"action":   action.Type,
				"decision": verdict.Decision,
				"tier":     verdict.Tier,
			})
			_ = auditLog.Log(audit.Entry{
				EventType:  audit.ActionEvaluated,
				ActionType: string(action.Type),
				Details:    string(details),
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(verdict)
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("GET /status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"listen":      cfg.Listen,
			"policy_file": cfg.Policy.File,
			"fail_closed": cfg.FailClosed,
			"rate_limit":  cfg.RateLimit,
			"daily_budget": cfg.DailyBudget,
		})
	})

	server := &http.Server{
		Addr:              cfg.Listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Fprintf(os.Stderr, "Shield listening on %s\n", cfg.Listen)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigCh
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	if listenErr := server.ListenAndServe(); listenErr != http.ErrServerClosed {
		return listenErr
	}
	return nil
}
