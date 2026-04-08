package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/openparallax/openparallax/internal/types"
	"gopkg.in/yaml.v3"
)

// backupRetention is the maximum number of timestamped backups to keep.
// The audit chain (when a SaveOption with auditing is supplied) records
// the previous-hash and new-hash of every save, so even if a backup file
// rotates out of the window the diff is still cryptographically attested
// in the audit log.
const backupRetention = 100

// AuditEmitter is the minimal contract config.Save needs to record a
// ConfigChanged entry. Implemented by *audit.Logger but intentionally
// abstracted so the config package does not import audit (and the
// dependency edge can stay unidirectional).
type AuditEmitter interface {
	EmitConfigChanged(source, details string) error
}

// saveOptions is the internal accumulator for SaveOption.
type saveOptions struct {
	audit       AuditEmitter
	source      string
	changedKeys []string
}

// SaveOption configures a Save call. Optional — Save with no options is
// equivalent to the previous behaviour (atomic write + round-trip + backup,
// no audit emission).
type SaveOption func(*saveOptions)

// WithAudit attaches an audit emitter and metadata. On a successful save
// Save will call emitter.EmitConfigChanged exactly once with the source,
// the changed key list, the previous file hash, and the new file hash.
func WithAudit(emitter AuditEmitter, source string, changedKeys []string) SaveOption {
	return func(o *saveOptions) {
		o.audit = emitter
		o.source = source
		o.changedKeys = changedKeys
	}
}

// Save serializes cfg using yaml.Marshal, writes the file atomically via
// <path>.tmp + rename, then re-loads through Load() to verify the
// round-trip succeeds. Before overwriting, the previous file is copied
// to <workspace>/.openparallax/backups/config-<timestamp>.yaml. The
// backup directory is rotated to the most recent backupRetention files.
//
// On any failure (marshal, write, validate, round-trip), the on-disk
// file is left untouched (or restored from backup) and the error is
// returned to the caller.
//
// When called with a WithAudit option, a successful save emits one
// ConfigChanged audit entry containing the source, the list of keys the
// caller is mutating, and the SHA-256 of the previous and new file
// contents. The hash diff makes silent rotation of the backups directory
// detectable after the fact.
func Save(path string, cfg *types.AgentConfig, opts ...SaveOption) error {
	options := &saveOptions{}
	for _, opt := range opts {
		opt(options)
	}

	previousHash := hashFileIfExists(path)

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Best-effort backup of the previous file. Failure to back up is
	// not fatal — the round-trip below catches schema breakage anyway,
	// and the audit chain still records the hash diff.
	if _, statErr := os.Stat(path); statErr == nil {
		if backupErr := backupConfig(path, cfg.Workspace); backupErr != nil {
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

	if options.audit != nil {
		newHash := hashBytes(data)
		details := formatConfigChangedDetails(options.source, options.changedKeys, previousHash, newHash)
		if emitErr := options.audit.EmitConfigChanged(options.source, details); emitErr != nil {
			// Audit emission failure is logged but not propagated — the
			// config write already succeeded and rolling it back would
			// create a worse outcome (live cfg in memory diverges from
			// disk). The caller can read the warning via stderr/log.
			fmt.Fprintf(os.Stderr, "warning: config audit emit failed: %s\n", emitErr)
		}
	}

	return nil
}

func hashFileIfExists(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return hashBytes(data)
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func formatConfigChangedDetails(source string, keys []string, prev, next string) string {
	var b strings.Builder
	if source == "" {
		source = "unknown"
	}
	fmt.Fprintf(&b, "source=%s", source)
	if len(keys) > 0 {
		fmt.Fprintf(&b, " keys=%s", strings.Join(keys, ","))
	}
	if prev == "" {
		prev = "(none)"
	}
	fmt.Fprintf(&b, " prev_hash=%s new_hash=%s", prev, next)
	return b.String()
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
