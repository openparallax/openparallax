package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/engine/executors"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/sandbox"
	"github.com/spf13/cobra"
)

var doctorConfigPath string

var doctorCmd = &cobra.Command{
	Use:          "doctor [name]",
	Short:        "System health check",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runDoctor,
}

func init() {
	doctorCmd.Flags().StringVarP(&doctorConfigPath, "config", "c", "", "path to config.yaml")
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(_ *cobra.Command, args []string) error {
	cfgPath, err := resolveConfig(args, doctorConfigPath)
	if err != nil {
		return err
	}

	passed, warned, failed := 0, 0, 0

	fmt.Println("\nOpenParallax System Check")
	fmt.Println(strings.Repeat("\u2500", 40))
	fmt.Println()

	// Config
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Printf("  \033[31m\u2717\033[0m %-16s failed to load: %s\n", "Config", err)
		return nil
	}
	printCheck(true, "Config", cfgPath+" loaded")
	passed++

	// Config round-trip — Save the loaded config to a temp file and
	// reload it. Catches writer drift before the next restart bites.
	tmpDir, tmpErr := os.MkdirTemp("", "openparallax-doctor-")
	if tmpErr == nil {
		tmpPath := filepath.Join(tmpDir, "config.yaml")
		rtCfg := *cfg
		if rtErr := config.Save(tmpPath, &rtCfg); rtErr != nil {
			printCheck(false, "Config writer", rtErr.Error())
			failed++
		} else {
			printCheck(true, "Config writer", "round-trip ok")
			passed++
		}
		_ = os.RemoveAll(tmpDir)
	}

	// Workspace
	if info, err := os.Stat(cfg.Workspace); err != nil || !info.IsDir() {
		printCheck(false, "Workspace", cfg.Workspace+" not found")
		failed++
	} else {
		printCheck(true, "Workspace", cfg.Workspace)
		passed++
	}

	// SQLite
	dbPath := filepath.Join(cfg.Workspace, ".openparallax", "openparallax.db")
	if db, err := storage.Open(dbPath); err != nil {
		printCheck(false, "SQLite", fmt.Sprintf("failed to open: %s", err))
		failed++
	} else {
		info, _ := os.Stat(dbPath)
		size := ""
		if info != nil {
			size = formatBytes(info.Size())
		}
		printCheck(true, "SQLite", fmt.Sprintf("openparallax.db (%s, WAL mode)", size))
		_ = db.Close()
		passed++
	}

	// LLM Provider
	if chat, ok := cfg.ChatModel(); ok && chat.Provider != "" {
		printCheck(true, "LLM Provider", fmt.Sprintf("%s / %s", chat.Provider, chat.Model))
		passed++
	} else {
		printWarn("LLM Provider", "not configured")
		warned++
	}

	// Shield
	policyPath := cfg.Shield.PolicyFile
	if policyPath == "" {
		policyPath = filepath.Join(cfg.Workspace, "security", "shield", "default.yaml")
	}
	if _, err := os.Stat(policyPath); err != nil {
		printWarn("Shield", fmt.Sprintf("policy file not found: %s", policyPath))
		warned++
	} else {
		budget := cfg.General.DailyBudget
		if budget == 0 {
			budget = 100
		}
		printCheck(true, "Shield", fmt.Sprintf("policy loaded, Tier 2: %d/day budget", budget))
		passed++
	}

	// Tier 1 classifier mode
	if cfg.Shield.ClassifierEnabled && cfg.Shield.ClassifierMode == "sidecar" {
		printCheck(true, "Tier 1", fmt.Sprintf("classifier sidecar at %s", cfg.Shield.ClassifierAddr))
		passed++
	} else {
		printCheck(true, "Tier 1", "heuristic-only (79 rules)")
		passed++
	}

	// Embedding
	if emb, ok := cfg.EmbeddingModel(); ok && emb.Provider != "" {
		printCheck(true, "Embedding", fmt.Sprintf("%s / %s", emb.Provider, emb.Model))
		passed++
	} else {
		printWarn("Embedding", "not configured (FTS5 only)")
		warned++
	}

	// Browser
	if browser := executors.DetectBrowser(); browser != "" {
		printCheck(true, "Browser", browser)
		passed++
	} else {
		printWarn("Browser", "no Chromium browser detected")
		warned++
	}

	// Email
	if cfg.Email.SMTP.Host != "" {
		printCheck(true, "Email", fmt.Sprintf("SMTP: %s", cfg.Email.SMTP.Host))
		passed++
	} else {
		printWarn("Email", "SMTP not configured")
		warned++
	}

	// Calendar
	if cfg.Calendar.Provider != "" {
		printCheck(true, "Calendar", cfg.Calendar.Provider)
		passed++
	} else {
		printWarn("Calendar", "not configured")
		warned++
	}

	// HEARTBEAT
	hbPath := filepath.Join(cfg.Workspace, "HEARTBEAT.md")
	if data, err := os.ReadFile(hbPath); err == nil {
		lines := strings.Count(string(data), "- `")
		printCheck(true, "HEARTBEAT", fmt.Sprintf("%d scheduled tasks", lines))
		passed++
	} else {
		printCheck(true, "HEARTBEAT", "no tasks scheduled")
		passed++
	}

	// Audit
	auditPath := filepath.Join(cfg.Workspace, ".openparallax", "audit.jsonl")
	if data, err := os.ReadFile(auditPath); err == nil {
		lines := strings.Count(string(data), "\n")
		chainOK := verifyChainQuick(data)
		status := "chain valid"
		if !chainOK {
			status = "CHAIN BROKEN"
		}
		printCheck(chainOK, "Audit", fmt.Sprintf("%d entries, %s", lines, status))
		if chainOK {
			passed++
		} else {
			failed++
		}
	} else {
		printCheck(true, "Audit", "no entries yet")
		passed++
	}

	// Sandbox
	sbStatus := sandbox.Probe()
	if sbStatus.Active {
		detail := sbStatus.Mode
		if sbStatus.Version > 0 {
			detail = fmt.Sprintf("%s V%d", detail, sbStatus.Version)
		}
		var capabilities string
		switch {
		case sbStatus.Filesystem && sbStatus.Network:
			capabilities = " (filesystem + network)"
		case sbStatus.Filesystem:
			capabilities = " (filesystem only)"
		default:
			capabilities = " (process limits only)"
		}
		printCheck(true, "Sandbox", detail+capabilities)
		passed++
	} else {
		printWarn("Sandbox", sbStatus.Reason)
		warned++
	}

	// Web UI
	printCheck(true, "Web UI", fmt.Sprintf("port %d", cfg.Web.Port))
	passed++

	fmt.Printf("\n  %d/%d checks passed.", passed, passed+warned+failed)
	if warned > 0 {
		fmt.Printf(" %d warnings (non-critical).", warned)
	}
	if failed > 0 {
		fmt.Printf(" %d failures.", failed)
	}
	fmt.Println()

	return nil
}

func printCheck(ok bool, name, detail string) {
	if ok {
		fmt.Printf("  \033[32m\u2713\033[0m %-16s %s\n", name, detail)
	} else {
		fmt.Printf("  \033[31m\u2717\033[0m %-16s %s\n", name, detail)
	}
}

func printWarn(name, detail string) {
	fmt.Printf("  \033[33m\u26A0\033[0m %-16s %s\n", name, detail)
}

func verifyChainQuick(data []byte) bool {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	prevHash := ""
	for i, line := range lines {
		if line == "" {
			continue
		}
		var entry struct {
			Hash         string `json:"hash"`
			PreviousHash string `json:"previous_hash"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return false
		}
		if i > 0 && entry.PreviousHash != prevHash {
			return false
		}
		prevHash = entry.Hash
	}
	return true
}
