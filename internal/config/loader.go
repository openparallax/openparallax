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
