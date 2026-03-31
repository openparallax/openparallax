package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// ErrDecryptionFailed indicates that decryption failed due to wrong key or tampered ciphertext.
var ErrDecryptionFailed = errors.New("decryption failed")

// DeriveKey derives a 256-bit AES key from a canary token hex string using HKDF-SHA256.
// The info parameter provides domain separation (e.g. "openparallax-oauth-encryption").
func DeriveKey(canaryHex, info string) ([]byte, error) {
	raw, err := hex.DecodeString(canaryHex)
	if err != nil {
		return nil, fmt.Errorf("decode canary hex: %w", err)
	}
	if len(raw) < 16 {
		return nil, fmt.Errorf("canary too short: need at least 16 bytes, got %d", len(raw))
	}

	reader := hkdf.New(sha256.New, raw, nil, []byte(info))
	key := make([]byte, 32) // AES-256
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, fmt.Errorf("HKDF expand: %w", err)
	}
	return key, nil
}

// Encrypt encrypts plaintext using AES-256-GCM. Returns nonce || ciphertext.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// nonce is prepended to the ciphertext
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts ciphertext produced by Encrypt. Expects nonce || ciphertext.
func Decrypt(key, ciphertextWithNonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrDecryptionFailed
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertextWithNonce) < nonceSize+1 {
		return nil, ErrDecryptionFailed
	}

	nonce := ciphertextWithNonce[:nonceSize]
	ciphertext := ciphertextWithNonce[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}
	return plaintext, nil
}
