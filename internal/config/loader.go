package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/spf13/viper"
)

// Load reads configuration from the specified YAML file, resolves environment
// variables, applies defaults, validates, and returns the complete config.
func Load(configPath string) (*types.AgentConfig, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	applyDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("%w: %s", types.ErrConfigNotFound, err)
	}

	var cfg types.AgentConfig
	decoderOpt := viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	))
	tagOpt := func(dc *mapstructure.DecoderConfig) { dc.TagName = "yaml" }
	if err := v.Unmarshal(&cfg, decoderOpt, tagOpt); err != nil {
		return nil, fmt.Errorf("%w: %s", types.ErrConfigInvalid, err)
	}

	cfg.Workspace = resolvePath(cfg.Workspace, filepath.Dir(configPath))

	// Derive LLM config from Models + Roles for backward-compatible access.
	deriveModelConfigs(&cfg)

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	if cfg.LLM.Provider == cfg.Shield.Evaluator.Provider &&
		cfg.LLM.Model == cfg.Shield.Evaluator.Model &&
		cfg.Shield.Evaluator.Provider != "" {
		fmt.Fprintf(os.Stderr,
			"Warning: Agent and Shield evaluator are using the same model (%s/%s). "+
				"Cross-model evaluation is recommended for stronger security.\n",
			cfg.LLM.Provider, cfg.LLM.Model)
	}

	return &cfg, nil
}

// deriveModelConfigs populates legacy LLM/Shield/Memory/Agent config fields
// from the new Models+Roles structure so existing code can access them.
func deriveModelConfigs(cfg *types.AgentConfig) {
	lookup := make(map[string]types.ModelEntry, len(cfg.Models))
	for _, m := range cfg.Models {
		lookup[m.Name] = m
	}

	if m, ok := lookup[cfg.Roles.Chat]; ok {
		cfg.LLM.Provider = m.Provider
		cfg.LLM.Model = m.Model
		cfg.LLM.APIKeyEnv = m.APIKeyEnv
		cfg.LLM.BaseURL = m.BaseURL
	}

	if m, ok := lookup[cfg.Roles.Shield]; ok {
		cfg.Shield.Evaluator.Provider = m.Provider
		cfg.Shield.Evaluator.Model = m.Model
		cfg.Shield.Evaluator.APIKeyEnv = m.APIKeyEnv
		cfg.Shield.Evaluator.BaseURL = m.BaseURL
	}

	if m, ok := lookup[cfg.Roles.Embedding]; ok {
		cfg.Memory.Embedding.Provider = m.Provider
		cfg.Memory.Embedding.Model = m.Model
		cfg.Memory.Embedding.APIKeyEnv = m.APIKeyEnv
		cfg.Memory.Embedding.BaseURL = m.BaseURL
	}

	if m, ok := lookup[cfg.Roles.SubAgent]; ok {
		cfg.Agents.SubAgentModel = m.Model
	}

	// Image and video generation roles are resolved by the engine's
	// model registry at runtime — no legacy fields to derive.
}

// resolvePath resolves a path relative to the config file's directory.
// Expands ~ to home directory. Resolves relative paths.
func resolvePath(p string, configDir string) string {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			p = filepath.Join(home, p[1:])
		}
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(configDir, p)
	}
	return filepath.Clean(p)
}
