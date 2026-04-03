package crypto

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSHA256HexKnownValue(t *testing.T) {
	// SHA-256 of empty string is a well-known constant.
	hash := SHA256Hex([]byte(""))
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", hash)
}

func TestSHA256HexDeterministic(t *testing.T) {
	a := SHA256Hex([]byte("hello world"))
	b := SHA256Hex([]byte("hello world"))
	assert.Equal(t, a, b)
}

func TestCanonicalizeKeyOrdering(t *testing.T) {
	a := map[string]any{"z": 1, "a": 2, "m": 3}
	b := map[string]any{"a": 2, "m": 3, "z": 1}

	ca, err := Canonicalize(a)
	require.NoError(t, err)
	cb, err := Canonicalize(b)
	require.NoError(t, err)

	assert.Equal(t, string(ca), string(cb), "same data with different key order should canonicalize identically")
}

func TestCanonicalizeNestedMaps(t *testing.T) {
	a := map[string]any{"outer": map[string]any{"z": 1, "a": 2}}
	b := map[string]any{"outer": map[string]any{"a": 2, "z": 1}}

	ca, err := Canonicalize(a)
	require.NoError(t, err)
	cb, err := Canonicalize(b)
	require.NoError(t, err)

	assert.Equal(t, string(ca), string(cb))
}

func TestHashActionDeterministic(t *testing.T) {
	payload := map[string]any{"path": "/tmp/test.txt"}
	a, err := HashAction("read_file", payload)
	require.NoError(t, err)

	b, err := HashAction("read_file", payload)
	require.NoError(t, err)

	assert.Equal(t, a, b)
	assert.Len(t, a, 64) // SHA-256 hex is 64 characters.
}

func TestHashActionDifferentPayload(t *testing.T) {
	a, err := HashAction("read_file", map[string]any{"path": "/tmp/a.txt"})
	require.NoError(t, err)

	b, err := HashAction("read_file", map[string]any{"path": "/tmp/b.txt"})
	require.NoError(t, err)

	assert.NotEqual(t, a, b, "different payloads should produce different hashes")
}

func TestHashActionDifferentType(t *testing.T) {
	payload := map[string]any{"path": "/tmp/test.txt"}
	a, err := HashAction("read_file", payload)
	require.NoError(t, err)

	b, err := HashAction("write_file", payload)
	require.NoError(t, err)

	assert.NotEqual(t, a, b, "different action types should produce different hashes")
}

func TestGenerateCanaryFormat(t *testing.T) {
	canary, err := GenerateCanary()
	require.NoError(t, err)

	assert.Len(t, canary, 64, "canary should be 64 hex characters")
	// Verify it's valid hex.
	for _, c := range canary {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"canary should only contain lowercase hex characters, got %c", c)
	}
}

func TestGenerateCanaryUnique(t *testing.T) {
	a, err := GenerateCanary()
	require.NoError(t, err)
	b, err := GenerateCanary()
	require.NoError(t, err)

	assert.NotEqual(t, a, b, "two canary tokens should be different")
}

func TestVerifyCanaryPresent(t *testing.T) {
	canary := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	response := "Some text before " + canary + " and some after."

	assert.True(t, VerifyCanary(response, canary))
}

func TestVerifyCanaryAbsent(t *testing.T) {
	canary := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	response := "This response does not contain the token."

	assert.False(t, VerifyCanary(response, canary))
}

func TestVerifyCanaryPartialMatch(t *testing.T) {
	canary := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	// Only first 32 chars — should not match.
	response := "prefix " + canary[:32] + " suffix"

	assert.False(t, VerifyCanary(response, canary))
}

func TestVerifyCanaryWrongLength(t *testing.T) {
	assert.False(t, VerifyCanary("short response", "short"))
}

func TestVerifyCanaryEmptyResponse(t *testing.T) {
	canary := strings.Repeat("a", 64)
	assert.False(t, VerifyCanary("", canary))
}

func TestNewIDFormat(t *testing.T) {
	id := NewID()
	assert.Len(t, id, 36, "UUID should be 36 characters (with dashes)")
	assert.Contains(t, id, "-")
}

func TestNewIDUnique(t *testing.T) {
	a := NewID()
	b := NewID()
	assert.NotEqual(t, a, b)
}

func TestRandomHex(t *testing.T) {
	hex, err := RandomHex(16)
	require.NoError(t, err)
	assert.Len(t, hex, 32, "16 bytes should produce 32 hex characters")
}
