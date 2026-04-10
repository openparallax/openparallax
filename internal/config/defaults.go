// Package config handles loading, validating, and resolving the agent configuration.
package config

import "github.com/openparallax/openparallax/internal/types"

// DefaultConfig returns an AgentConfig populated with default values.
// yaml.Unmarshal then overlays the on-disk file on top of this struct,
// so any field the user did not set retains its default. This is the
// single source of truth for every config default — no other file
// should hardcode fallback values for these fields.
func DefaultConfig() types.AgentConfig {
	return types.AgentConfig{
		Workspace: ".",

		Shield: types.ShieldConfig{
			PolicyFile:       "security/shield/default.yaml",
			OnnxThreshold:    0.85,
			HeuristicEnabled: true,
		},

		Chronicle: types.ChronicleConfig{
			MaxSnapshots: 100,
			MaxAgeDays:   30,
		},

		Web: types.WebConfig{
			Enabled: true,
			Port:    3100,
			Auth:    true,
		},

		Agents: types.AgentsConfig{
			MaxToolRounds:             25,
			ContextWindow:             128000,
			CompactionThreshold:       70,
			MaxResponseTokens:         4096,
			ShellTimeoutSeconds:       30,
			BrowserNavTimeoutSeconds:  30,
			BrowserIdleMinutes:        5,
			SubAgentTimeoutSeconds:    900,
			MaxConcurrentSubAgents:    10,
			MaxSubAgentRounds:         20,
			CrashRestartBudget:        5,
			CrashWindowSeconds:        60,
			MaxConsecutiveNavFailures: 3,
		},

		General: types.GeneralConfig{
			FailClosed:        true,
			RateLimit:         30,
			VerdictTTLSeconds: 60,
			DailyBudget:       100,
		},

		Security: types.SecurityConfig{
			IFCPolicy: "security/ifc/default.yaml",
		},
	}
}
