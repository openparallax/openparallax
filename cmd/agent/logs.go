package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openparallax/openparallax/internal/config"
	"github.com/spf13/cobra"
)

var (
	logsConfigPath string
	logsLevel      string
	logsEvent      string
	logsLines      int
)

var logsCmd = &cobra.Command{
	Use:          "logs",
	Short:        "Tail the engine log",
	SilenceUsage: true,
	RunE:         runLogs,
}

func init() {
	logsCmd.Flags().StringVarP(&logsConfigPath, "config", "c", "", "path to config.yaml")
	logsCmd.Flags().StringVar(&logsLevel, "level", "", "filter by level (info, warn, error)")
	logsCmd.Flags().StringVar(&logsEvent, "event", "", "filter by event type")
	logsCmd.Flags().IntVarP(&logsLines, "lines", "n", 50, "number of lines to show")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(_ *cobra.Command, _ []string) error {
	cfgPath := logsConfigPath
	if cfgPath == "" {
		cfgPath = findConfig()
	}
	if cfgPath == "" {
		return fmt.Errorf("workspace not found: use --config")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logPath := filepath.Join(cfg.Workspace, ".openparallax", "engine.log")
	f, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("cannot open engine.log: %w", err)
	}
	defer func() { _ = f.Close() }()

	var entries []map[string]any
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if logsLevel != "" {
			if lvl, ok := entry["level"].(string); !ok || lvl != logsLevel {
				continue
			}
		}
		if logsEvent != "" {
			if evt, ok := entry["event"].(string); !ok || !strings.Contains(evt, logsEvent) {
				continue
			}
		}
		entries = append(entries, entry)
	}

	start := 0
	if len(entries) > logsLines {
		start = len(entries) - logsLines
	}

	for _, entry := range entries[start:] {
		ts, _ := entry["timestamp"].(string)
		if len(ts) > 23 {
			ts = ts[:23]
		}
		level, _ := entry["level"].(string)
		event, _ := entry["event"].(string)

		color := ""
		reset := "\033[0m"
		switch level {
		case "error":
			color = "\033[31m"
		case "warn":
			color = "\033[33m"
		default:
			color = ""
			reset = ""
		}

		data, _ := entry["data"].(map[string]any)
		detail := ""
		if data != nil {
			parts := make([]string, 0, len(data))
			for k, v := range data {
				parts = append(parts, fmt.Sprintf("%s=%v", k, v))
			}
			detail = strings.Join(parts, " ")
		}

		fmt.Printf("%s%s  %-5s  %-24s  %s%s\n", color, ts, level, event, detail, reset)
	}

	return nil
}
