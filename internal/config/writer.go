package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/openparallax/openparallax/internal/types"
	"gopkg.in/yaml.v3"
)

// backupRetention is the maximum number of timestamped backups to keep.
const backupRetention = 10

// Save serializes cfg using yaml.Marshal, writes the file atomically via
// <path>.tmp + rename, then re-loads through Load() to verify the
// round-trip succeeds. Before overwriting, the previous file is copied
// to <workspace>/.openparallax/backups/config-<timestamp>.yaml. The
// backup directory is rotated to the most recent backupRetention files.
//
// On any failure (marshal, write, validate, round-trip), the on-disk
// file is left untouched (or restored from backup) and the error is
// returned to the caller.
func Save(path string, cfg *types.AgentConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Best-effort backup of the previous file. Failure to back up is
	// not fatal — the round-trip below catches schema breakage anyway.
	if _, statErr := os.Stat(path); statErr == nil {
		if backupErr := backupConfig(path, cfg.Workspace); backupErr != nil {
			// Log via stderr; do not block the save.
			fmt.Fprintf(os.Stderr, "warning: config backup failed: %s\n", backupErr)
		}
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}

	// Validate the marshaled output through the loader before promoting.
	if _, loadErr := Load(tmp); loadErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("config round-trip failed: %w", loadErr)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename temp config: %w", err)
	}

	return nil
}

// backupConfig copies the existing config file into the workspace's
// .openparallax/backups directory with a timestamped filename, then
// rotates the directory to keep at most backupRetention files.
func backupConfig(path, workspace string) error {
	if workspace == "" {
		workspace = filepath.Dir(path)
	}
	backupDir := filepath.Join(workspace, ".openparallax", "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	stamp := time.Now().Format("20060102-150405")
	dest := filepath.Join(backupDir, fmt.Sprintf("config-%s.yaml", stamp))
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return err
	}

	return rotateBackups(backupDir)
}

// rotateBackups deletes the oldest backups beyond backupRetention.
func rotateBackups(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) < 7 || name[:7] != "config-" {
			continue
		}
		files = append(files, name)
	}

	sort.Strings(files)
	if len(files) <= backupRetention {
		return nil
	}

	for _, name := range files[:len(files)-backupRetention] {
		_ = os.Remove(filepath.Join(dir, name))
	}
	return nil
}
