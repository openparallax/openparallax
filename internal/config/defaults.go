// Package config handles loading, validating, and resolving the agent configuration.
package config

import "github.com/spf13/viper"

// applyDefaults sets all default configuration values.
func applyDefaults(v *viper.Viper) {
	v.SetDefault("workspace", ".")

	v.SetDefault("shield.policy_file", "policies/default.yaml")
	v.SetDefault("shield.onnx_threshold", 0.85)
	v.SetDefault("shield.heuristic_enabled", true)

	v.SetDefault("chronicle.max_snapshots", 100)
	v.SetDefault("chronicle.max_age_days", 30)

	v.SetDefault("web.enabled", true)
	v.SetDefault("web.port", 3100)
	v.SetDefault("web.auth", true)

	v.SetDefault("agents.max_tool_rounds", 25)
	v.SetDefault("agents.context_window", 128000)
	v.SetDefault("agents.compaction_threshold", 70)
	v.SetDefault("agents.max_response_tokens", 4096)

	v.SetDefault("general.fail_closed", true)
	v.SetDefault("general.rate_limit", 30)
	v.SetDefault("general.verdict_ttl_seconds", 60)
	v.SetDefault("general.daily_budget", 100)
}
