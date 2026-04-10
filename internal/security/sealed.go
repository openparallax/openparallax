// Package security provides the immutable, compiled-in security configuration
// that governs which defenses are non-negotiable. Values are frozen at engine
// startup via Seal() and validated by a SHA-256 digest on every subsequent
// Get(). Tampering panics fail-closed.
//
// The type system enforces the boundary: there is no constructor that reads
// from YAML, no setter, no SettableKey entry, and no environment variable
// that controls any field. The only source is Defaults(), which returns
// compiled-in constants.
package security

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync/atomic"
)

// Sealed is the immutable, init-time-frozen security configuration. Loaded
// once at engine startup from compiled-in defaults. Never read from YAML,
// never read from environment variables, never settable via /config set.
// Tampering with these values at runtime is detected and panics fail-closed.
type Sealed struct {
	// ShieldEnabled ensures the 4-tier Shield pipeline is active.
	ShieldEnabled bool `json:"shield_enabled"`
	// ProtectionLayerEnabled ensures the hardcoded pre-Shield protection gate runs.
	ProtectionLayerEnabled bool `json:"protection_layer_enabled"`
	// DenylistEnabled ensures the cross-platform default denylist is enforced.
	DenylistEnabled bool `json:"denylist_enabled"`
	// SandboxRequired ensures the kernel sandbox is engaged (best-effort).
	SandboxRequired bool `json:"sandbox_required"`
	// HashVerifierEnabled ensures TOCTOU prevention via action hash verification.
	HashVerifierEnabled bool `json:"hash_verifier_enabled"`
	// CanaryEnforced ensures Tier 2 evaluator canary token verification runs.
	CanaryEnforced bool `json:"canary_enforced"`
	// AuditChainEnabled ensures the append-only audit log with SHA-256 chain is active.
	AuditChainEnabled bool `json:"audit_chain_enabled"`
	// AgentAuthRequired ensures the agent process authenticates to the engine.
	AgentAuthRequired bool `json:"agent_auth_required"`
	// IFCEnabled ensures the Information Flow Control subsystem is active.
	// The policy (sources, sinks, rules) is tunable; the subsystem itself is not.
	IFCEnabled bool `json:"ifc_enabled"`
	// SafeCommandAllowlistFrozen ensures the safe-command fast path uses the
	// compiled-in, curated allowlist and is not user-extensible.
	SafeCommandAllowlistFrozen bool `json:"safe_command_allowlist_frozen"`
}

// Defaults returns the immutable, compiled-in security configuration.
// There is no constructor that takes a config source. Every field is true.
func Defaults() Sealed {
	return Sealed{
		ShieldEnabled:              true,
		ProtectionLayerEnabled:     true,
		DenylistEnabled:            true,
		SandboxRequired:            true,
		HashVerifierEnabled:        true,
		CanaryEnforced:             true,
		AuditChainEnabled:          true,
		AgentAuthRequired:          true,
		IFCEnabled:                 true,
		SafeCommandAllowlistFrozen: true,
	}
}

var (
	sealed atomic.Pointer[Sealed]
	digest atomic.Pointer[string]
)

// Seal locks in the runtime security configuration. Called once at engine
// startup. Subsequent calls are no-ops. After Seal(), Get() validates the
// stored value against a SHA-256 digest on every read; mismatches panic.
func Seal(cfg Sealed) {
	if sealed.Load() != nil {
		return
	}
	raw, _ := json.Marshal(cfg)
	h := sha256.Sum256(raw)
	s := hex.EncodeToString(h[:])
	sealed.Store(&cfg)
	digest.Store(&s)
}

// Get returns the sealed security configuration. Panics if Seal() has not
// been called or if the stored value's digest no longer matches — meaning
// something tampered with the in-memory representation.
func Get() Sealed {
	cfg := sealed.Load()
	if cfg == nil {
		panic("security: Get() called before Seal()")
	}
	raw, _ := json.Marshal(*cfg)
	h := sha256.Sum256(raw)
	if hex.EncodeToString(h[:]) != *digest.Load() {
		panic("security: tamper detected — sealed config digest mismatch")
	}
	return *cfg
}

// IsSealed reports whether Seal() has been called. Used by tests and
// startup code to gate Get() calls without risking a panic.
func IsSealed() bool {
	return sealed.Load() != nil
}

// ResetForTest clears the sealed state so tests can call Seal() again.
// Must only be called from test code.
func ResetForTest() {
	sealed.Store(nil)
	digest.Store(nil)
}
