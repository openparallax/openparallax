// Package config handles loading, validating, and resolving the agent configuration.
package config

import "github.com/spf13/viper"

// applyDefaults sets all default configuration values.
func applyDefaults(v *viper.Viper) {
	v.SetDefault("workspace", ".")

	v.SetDefault("shield.policy_file", "policies/default.yaml")
	v.SetDefault("shield.onnx_threshold", 0.85)
	v.SetDefault("shield.heuristic_enabled", true)
	v.SetDefault("shield.classifier_enabled", true)
	v.SetDefault("shield.classifier_mode", "local")
	// Action types where ONNX has been observed to over-fire on benign payloads.
	// Heuristics still run on these types — only the ML classifier is skipped.
	// Override in workspace config to change.
	v.SetDefault("shield.classifier_skip_types", []string{
		"write_file",
		"delete_file",
		"move_file",
		"copy_file",
		"send_email",
		"send_message",
		"http_request",
	})

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
