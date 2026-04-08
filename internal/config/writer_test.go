package config

import (
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleConfig(workspace string) *types.AgentConfig {
	return &types.AgentConfig{
		Workspace: workspace,
		Models: []types.ModelEntry{
			{Name: "chat", Provider: "anthropic", Model: "claude-sonnet-4-6", APIKeyEnv: "ANTHROPIC_API_KEY"},
			{Name: "shield", Provider: "openai", Model: "gpt-5.4-mini", APIKeyEnv: "OPENAI_API_KEY"},
		},
		Roles: types.RolesConfig{Chat: "chat", Shield: "shield", Embedding: "shield"},
		Identity: types.IdentityConfig{
			Name:   "TestAgent",
			Avatar: "🤖",
		},
		Shield: types.ShieldConfig{
			PolicyFile:        "policies/default.yaml",
			OnnxThreshold:     0.85,
			HeuristicEnabled:  true,
			ClassifierEnabled: true,
			ClassifierMode:    "local",
		},
		Chronicle: types.ChronicleConfig{MaxSnapshots: 100, MaxAgeDays: 30},
		Web:       types.WebConfig{Enabled: true, Port: 3100, Auth: true},
		General:   types.GeneralConfig{FailClosed: true, RateLimit: 30, VerdictTTLSeconds: 60, DailyBudget: 100},
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := sampleConfig(dir)

	require.NoError(t, Save(path, cfg))

	loaded, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, cfg.Identity.Name, loaded.Identity.Name)
	assert.Equal(t, cfg.Identity.Avatar, loaded.Identity.Avatar)
	require.Len(t, loaded.Models, 2)
	assert.Equal(t, "anthropic", loaded.Models[0].Provider)
	assert.Equal(t, "claude-sonnet-4-6", loaded.Models[0].Model)
	assert.Equal(t, "chat", loaded.Roles.Chat)
	assert.Equal(t, "shield", loaded.Roles.Shield)

	chat, ok := loaded.ChatModel()
	require.True(t, ok)
	assert.Equal(t, "anthropic", chat.Provider)
}

func TestSaveValidatesBeforePromoting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Seed a valid config first.
	require.NoError(t, Save(path, sampleConfig(dir)))

	// Now attempt to save a config with no models — must fail and leave
	// the previous file intact.
	bad := sampleConfig(dir)
	bad.Models = nil
	err := Save(path, bad)
	require.Error(t, err)

	// Original file still loadable.
	loaded, err := Load(path)
	require.NoError(t, err)
	require.Len(t, loaded.Models, 2)
}

func TestSaveCreatesBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := sampleConfig(dir)

	require.NoError(t, Save(path, cfg))
	require.NoError(t, Save(path, cfg))

	backupDir := filepath.Join(dir, ".openparallax", "backups")
	entries, err := filepathGlob(backupDir, "config-*.yaml")
	require.NoError(t, err)
	assert.NotEmpty(t, entries, "expected at least one backup after second save")
}

func TestSettableKeysAllRoundTrip(t *testing.T) {
	for name, key := range SettableKeys {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			cfg := sampleConfig(dir)
			require.NoError(t, Save(path, cfg))

			loaded, err := Load(path)
			require.NoError(t, err)

			value := "test-value"
			switch name {
			case "roles.chat", "roles.shield":
				value = "chat"
			case "roles.embedding", "roles.sub_agent":
				value = "shield"
			case "chat.provider", "shield.provider", "embedding.provider":
				value = "ollama"
			}

			require.NoError(t, key.Setter(loaded, value))
			require.NoError(t, Save(path, loaded))

			_, err = Load(path)
			require.NoError(t, err, "key %s broke round-trip", name)
		})
	}
}

func filepathGlob(dir, pattern string) ([]string, error) {
	return filepath.Glob(filepath.Join(dir, pattern))
}

// fakeAuditEmitter records each EmitConfigChanged call. Implements
// config.AuditEmitter for tests.
type fakeAuditEmitter struct {
	calls []struct {
		source  string
		details string
	}
}

func (f *fakeAuditEmitter) EmitConfigChanged(source, details string) error {
	f.calls = append(f.calls, struct {
		source  string
		details string
	}{source, details})
	return nil
}

func TestSaveEmitsConfigChangedAuditEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := sampleConfig(dir)
	require.NoError(t, Save(path, cfg))

	emitter := &fakeAuditEmitter{}
	cfg.Identity.Name = "Renamed"
	require.NoError(t, Save(path, cfg, WithAudit(emitter, "slash-config", []string{"identity.name"})))

	require.Len(t, emitter.calls, 1)
	got := emitter.calls[0]
	assert.Equal(t, "slash-config", got.source)
	assert.Contains(t, got.details, "source=slash-config")
	assert.Contains(t, got.details, "keys=identity.name")
	assert.Contains(t, got.details, "prev_hash=")
	assert.Contains(t, got.details, "new_hash=")

	// previous and new hashes must differ — same payload twice would
	// indicate the diff layer is broken.
	assert.NotContains(t, got.details, "prev_hash=(none)")
}

func TestSaveWithoutAuditDoesNotEmit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := sampleConfig(dir)

	require.NoError(t, Save(path, cfg))
	// No emitter passed → no panic, no calls.
}

func TestSaveAuditFirstWriteReportsNoPrevHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := sampleConfig(dir)

	emitter := &fakeAuditEmitter{}
	require.NoError(t, Save(path, cfg, WithAudit(emitter, "cli-init", []string{"*"})))

	require.Len(t, emitter.calls, 1)
	assert.Contains(t, emitter.calls[0].details, "prev_hash=(none)")
}
