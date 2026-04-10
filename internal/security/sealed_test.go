package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultsAllTrue(t *testing.T) {
	d := Defaults()
	assert.True(t, d.ShieldEnabled)
	assert.True(t, d.ProtectionLayerEnabled)
	assert.True(t, d.DenylistEnabled)
	assert.True(t, d.SandboxRequired)
	assert.True(t, d.HashVerifierEnabled)
	assert.True(t, d.CanaryEnforced)
	assert.True(t, d.AuditChainEnabled)
	assert.True(t, d.AgentAuthRequired)
	assert.True(t, d.IFCEnabled)
	assert.True(t, d.SafeCommandAllowlistFrozen)
}

func TestSealAndGet(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	require.False(t, IsSealed())
	Seal(Defaults())
	require.True(t, IsSealed())

	cfg := Get()
	assert.True(t, cfg.ShieldEnabled)
}

func TestSealIsIdempotent(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	Seal(Defaults())

	// Second seal with different values is a no-op.
	Seal(Sealed{ShieldEnabled: false})

	cfg := Get()
	assert.True(t, cfg.ShieldEnabled, "second Seal should be ignored")
}

func TestGetBeforeSealPanics(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	assert.Panics(t, func() { Get() })
}
