// Package config handles loading, validating, and resolving the agent configuration.
package config

import "github.com/openparallax/openparallax/internal/types"

// defaultConfig returns an AgentConfig populated with default values.
// yaml.Unmarshal then overlays the on-disk file on top of this struct,
// so any field the user did not set retains its default.
func defaultConfig() types.AgentConfig {
	return types.AgentConfig{
		Workspace: ".",

		Shield: types.ShieldConfig{
			PolicyFile:        "policies/default.yaml",
			OnnxThreshold:     0.85,
			HeuristicEnabled:  true,
			ClassifierEnabled: true,
			ClassifierMode:    "local",
			// Action types where ONNX has been observed to over-fire on benign
			// payloads. Heuristics still run on these types — only the ML
			// classifier is skipped. Override in workspace config to change.
			ClassifierSkipTypes: []string{
				"write_file",
				"delete_file",
				"move_file",
				"copy_file",
				"send_email",
				"send_message",
				"http_request",
			},
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
			MaxToolRounds:       25,
			ContextWindow:       128000,
			CompactionThreshold: 70,
			MaxResponseTokens:   4096,
		},

		General: types.GeneralConfig{
			FailClosed:        true,
			RateLimit:         30,
			VerdictTTLSeconds: 60,
			DailyBudget:       100,
		},
	}
}
