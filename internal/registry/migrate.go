package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// sentinelFile is written after migration to prevent re-running.
const sentinelFile = ".migrated"

// Migrate scans ~/.openparallax/ for existing agent workspaces and generates
// the agents.json registry. Each directory containing a config.yaml is treated
// as an agent workspace. A sentinel file prevents re-migration.
func Migrate(registryPath string) error {
	dir := filepath.Dir(registryPath)

	// Check sentinel.
	if _, err := os.Stat(filepath.Join(dir, sentinelFile)); err == nil {
		return nil // Already migrated.
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("scan openparallax dir: %w", err)
	}

	reg, err := Load(registryPath)
	if err != nil {
		return err
	}

	found := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "workspace" {
			// Legacy default — skip.
			continue
		}

		cfgPath := filepath.Join(dir, name, "config.yaml")
		if _, statErr := os.Stat(cfgPath); statErr != nil {
			continue
		}

		agentName, webPort := readMigrationConfig(cfgPath, name)
		if webPort == 0 {
			webPort = reg.AllocatePort()
		}

		rec := AgentRecord{
			Name:       agentName,
			Slug:       name,
			Workspace:  filepath.Join(dir, name),
			ConfigPath: cfgPath,
			WebPort:    webPort,
			GRPCPort:   webPort + GRPCPortOffset,
			CreatedAt:  time.Now(),
		}

		// Skip if already registered (idempotent).
		if _, exists := reg.Lookup(name); exists {
			continue
		}
		if addErr := reg.Add(rec); addErr != nil {
			// Port collision — allocate a new one.
			rec.WebPort = reg.AllocatePort()
			rec.GRPCPort = rec.WebPort + GRPCPortOffset
			_ = reg.Add(rec)
		}
		found++
	}

	if found > 0 {
		// Write sentinel.
		_ = os.WriteFile(filepath.Join(dir, sentinelFile), []byte("migrated\n"), 0o644)
	}

	return nil
}

// readMigrationConfig extracts the agent name and web port from a config.yaml.
func readMigrationConfig(cfgPath, fallbackName string) (string, int) {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fallbackName, 0
	}

	var cfg struct {
		Identity struct {
			Name string `yaml:"name"`
		} `yaml:"identity"`
		Web struct {
			Port int `yaml:"port"`
		} `yaml:"web"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fallbackName, 0
	}

	name := cfg.Identity.Name
	if name == "" {
		// Capitalize first letter of the fallback name.
		if len(fallbackName) > 0 {
			name = strings.ToUpper(fallbackName[:1]) + fallbackName[1:]
		} else {
			name = fallbackName
		}
	}
	return name, cfg.Web.Port
}
