package main

import (
	"fmt"
	"os"

	"github.com/openparallax/openparallax/mcp"
	"github.com/openparallax/openparallax/shield"
	"gopkg.in/yaml.v3"
)

// ShieldConfig is the standalone Shield configuration loaded from shield.yaml.
type ShieldConfig struct {
	// Listen is the HTTP server address (e.g., "localhost:9090").
	Listen string `yaml:"listen"`

	// Policy configures Tier 0.
	Policy struct {
		File string `yaml:"file"`
	} `yaml:"policy"`

	// Classifier configures Tier 1.
	Classifier struct {
		Threshold float64 `yaml:"threshold"`
	} `yaml:"classifier"`

	// Heuristic configures heuristic pattern matching.
	Heuristic struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"heuristic"`

	// Evaluator configures Tier 2 LLM evaluation.
	Evaluator *shield.EvaluatorConfig `yaml:"evaluator"`

	// FailClosed causes all errors to result in BLOCK verdicts.
	FailClosed bool `yaml:"fail_closed"`

	// RateLimit is the maximum evaluations per minute.
	RateLimit int `yaml:"rate_limit"`

	// DailyBudget is the maximum Tier 2 evaluations per day.
	DailyBudget int `yaml:"daily_budget"`

	// VerdictTTL is the verdict validity duration in seconds.
	VerdictTTL int `yaml:"verdict_ttl"`

	// Audit configures audit logging.
	Audit struct {
		File string `yaml:"file"`
	} `yaml:"audit"`

	// MCP configures upstream MCP servers for proxy mode.
	MCP struct {
		Servers     []mcp.ServerConfig `yaml:"servers"`
		ToolMapping map[string]string  `yaml:"tool_mapping"`
	} `yaml:"mcp"`

	// LogLevel sets the logging verbosity.
	LogLevel string `yaml:"log_level"`
}

func loadShieldConfig(path string) (*ShieldConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg ShieldConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Listen == "" {
		cfg.Listen = "localhost:9090"
	}
	if cfg.Policy.File == "" {
		cfg.Policy.File = "policy.yaml"
	}
	if cfg.Classifier.Threshold == 0 {
		cfg.Classifier.Threshold = 0.85
	}
	if cfg.RateLimit == 0 {
		cfg.RateLimit = 100
	}
	if cfg.DailyBudget == 0 {
		cfg.DailyBudget = 100
	}
	if cfg.VerdictTTL == 0 {
		cfg.VerdictTTL = 60
	}

	return &cfg, nil
}

func (c *ShieldConfig) toPipelineConfig() shield.Config {
	return shield.Config{
		PolicyFile:       c.Policy.File,
		OnnxThreshold:    c.Classifier.Threshold,
		HeuristicEnabled: c.Heuristic.Enabled,
		Evaluator:        c.Evaluator,
		FailClosed:       c.FailClosed,
		RateLimit:        c.RateLimit,
		VerdictTTL:       c.VerdictTTL,
		DailyBudget:      c.DailyBudget,
	}
}
