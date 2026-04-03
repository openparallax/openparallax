package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key, err := DeriveKey(testCanary(), "test-encryption")
	require.NoError(t, err)

	plaintext := []byte("oauth-access-token-value-12345")
	ciphertext, err := Encrypt(key, plaintext)
	require.NoError(t, err)

	decrypted, err := Decrypt(key, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestDecryptWrongKeyFails(t *testing.T) {
	key1, err := DeriveKey(testCanary(), "key-one")
	require.NoError(t, err)
	key2, err := DeriveKey(testCanary(), "key-two")
	require.NoError(t, err)

	ciphertext, err := Encrypt(key1, []byte("secret"))
	require.NoError(t, err)

	_, err = Decrypt(key2, ciphertext)
	assert.ErrorIs(t, err, ErrDecryptionFailed)
}

func TestDecryptTruncatedCiphertext(t *testing.T) {
	key, err := DeriveKey(testCanary(), "test")
	require.NoError(t, err)

	_, err = Decrypt(key, []byte("too-short"))
	assert.ErrorIs(t, err, ErrDecryptionFailed)
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	key, err := DeriveKey(testCanary(), "test")
	require.NoError(t, err)

	ciphertext, err := Encrypt(key, []byte("original"))
	require.NoError(t, err)

	// Tamper with the last byte.
	ciphertext[len(ciphertext)-1] ^= 0xFF

	_, err = Decrypt(key, ciphertext)
	assert.ErrorIs(t, err, ErrDecryptionFailed)
}

func TestDeriveKeyDeterministic(t *testing.T) {
	k1, err := DeriveKey(testCanary(), "same-info")
	require.NoError(t, err)
	k2, err := DeriveKey(testCanary(), "same-info")
	require.NoError(t, err)
	assert.Equal(t, k1, k2)
}

func TestDeriveKeyDifferentInfoProducesDifferentKeys(t *testing.T) {
	k1, err := DeriveKey(testCanary(), "info-a")
	require.NoError(t, err)
	k2, err := DeriveKey(testCanary(), "info-b")
	require.NoError(t, err)
	assert.False(t, bytes.Equal(k1, k2))
}

func TestEncryptNonceUniqueness(t *testing.T) {
	key, err := DeriveKey(testCanary(), "test")
	require.NoError(t, err)

	plaintext := []byte("same input")
	ct1, err := Encrypt(key, plaintext)
	require.NoError(t, err)
	ct2, err := Encrypt(key, plaintext)
	require.NoError(t, err)

	// Same plaintext should produce different ciphertexts (different nonces).
	assert.False(t, bytes.Equal(ct1, ct2))
}

func TestDeriveKeyTooShortCanary(t *testing.T) {
	_, err := DeriveKey("abcd", "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestDeriveKeyInvalidHex(t *testing.T) {
	_, err := DeriveKey("not-hex-data!", "test")
	assert.Error(t, err)
}

// testCanary returns a valid 64-char hex canary for testing.
func testCanary() string {
	return "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
}
