package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openparallax/openparallax/internal/types"
	"gopkg.in/yaml.v3"
)

// Load reads configuration from the specified YAML file, applies defaults
// for any unset fields, validates, and returns the complete config.
//
// The decoder runs in strict mode (KnownFields(true)) so any unknown
// top-level key in the YAML — most importantly the legacy `llm:` key
// from the old schema — produces a clear parse error rather than being
// silently ignored.
func Load(configPath string) (*types.AgentConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", types.ErrConfigNotFound, err)
	}

	cfg := DefaultConfig()

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("%w: %s", types.ErrConfigInvalid, err)
	}

	cfg.Workspace = resolvePath(cfg.Workspace, filepath.Dir(configPath))

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	// Cross-model evaluation warning. The chat and shield roles should
	// reference different models in the pool — same model on both sides
	// makes it easier for an attacker to escape both with one payload.
	chat, chatOK := cfg.ChatModel()
	shield, shieldOK := cfg.ShieldModel()
	if chatOK && shieldOK && chat.Provider == shield.Provider && chat.Model == shield.Model {
		fmt.Fprintf(os.Stderr,
			"Warning: chat and shield roles use the same model (%s/%s). "+
				"Cross-model evaluation is recommended for stronger security.\n",
			chat.Provider, chat.Model)
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
