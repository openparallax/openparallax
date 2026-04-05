package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAllActionTypesCount(t *testing.T) {
	assert.Len(t, AllActionTypes, 69, "AllActionTypes should have 69 defined action types")
}

func TestAllActionTypesUnique(t *testing.T) {
	seen := make(map[ActionType]bool)
	for _, at := range AllActionTypes {
		assert.False(t, seen[at], "duplicate action type: %s", at)
		seen[at] = true
	}
}

func TestAllGoalTypesCount(t *testing.T) {
	assert.Len(t, AllGoalTypes, 13, "AllGoalTypes should have 13 defined goal types")
}

func TestVerdictIsExpiredBeforeTTL(t *testing.T) {
	v := Verdict{
		ExpiresAt: time.Now().Add(10 * time.Second),
	}
	assert.False(t, v.IsExpired(), "verdict should not be expired before TTL")
}

func TestVerdictIsExpiredAfterTTL(t *testing.T) {
	v := Verdict{
		ExpiresAt: time.Now().Add(-10 * time.Second),
	}
	assert.True(t, v.IsExpired(), "verdict should be expired after TTL")
}

func TestSentinelErrorsAreDistinct(t *testing.T) {
	errors := []error{
		ErrPipelineNotReady, ErrParserFailed, ErrSelfEvalFailed, ErrShieldUnavailable,
		ErrActionBlocked, ErrActionTimeout, ErrOTRBlocked, ErrHashMismatch,
		ErrApprovalTimeout, ErrApprovalDenied,
		ErrSessionNotFound, ErrSessionModeChange,
		ErrSnapshotNotFound, ErrTransactionActive, ErrNoActiveTransaction, ErrIntegrityViolation,
		ErrConfigNotFound, ErrConfigInvalid,
		ErrMemoryFileNotFound, ErrPathTraversal,
	}

	seen := make(map[string]bool)
	for _, err := range errors {
		msg := err.Error()
		assert.False(t, seen[msg], "duplicate error message: %s", msg)
		seen[msg] = true
	}
}

func TestSessionModeValues(t *testing.T) {
	assert.Equal(t, SessionMode("normal"), SessionNormal)
	assert.Equal(t, SessionMode("otr"), SessionOTR)
}

func TestAgentConfigYAMLRoundTrip(t *testing.T) {
	cfg := AgentConfig{
		Workspace: "/tmp/workspace",
		Models: []ModelEntry{
			{Name: "chat", Provider: "anthropic", Model: "claude-sonnet-4-20250514", APIKeyEnv: "ANTHROPIC_API_KEY"},
		},
		Roles: RolesConfig{Chat: "chat"},
		Shield: ShieldConfig{
			PolicyFile:       "policies/default.yaml",
			OnnxThreshold:    0.85,
			HeuristicEnabled: true,
		},
		General: GeneralConfig{
			FailClosed:        true,
			RateLimit:         30,
			VerdictTTLSeconds: 60,
			DailyBudget:       100,
		},
	}

	data, err := yaml.Marshal(&cfg)
	require.NoError(t, err)

	var decoded AgentConfig
	err = yaml.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, cfg.Models[0].Provider, decoded.Models[0].Provider)
	assert.Equal(t, cfg.Models[0].Model, decoded.Models[0].Model)
	assert.Equal(t, cfg.Roles.Chat, decoded.Roles.Chat)
	assert.Equal(t, cfg.Shield.OnnxThreshold, decoded.Shield.OnnxThreshold)
	assert.Equal(t, cfg.General.FailClosed, decoded.General.FailClosed)
}

func TestDefaultIdentityValues(t *testing.T) {
	assert.Equal(t, "Atlas", DefaultIdentity.Name)
	assert.NotEmpty(t, DefaultIdentity.Role)
}

func TestAllMemoryFilesCount(t *testing.T) {
	assert.Len(t, AllMemoryFiles, 8, "AllMemoryFiles should have 8 entries")
}

func TestAllMessagingPlatformsCount(t *testing.T) {
	assert.Len(t, AllMessagingPlatforms, 9, "AllMessagingPlatforms should have 9 entries")
}
