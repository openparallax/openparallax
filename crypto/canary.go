package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
)

// GenerateCanary creates a cryptographically random 64-character hex token.
// This token is injected into the evaluator prompt and verified in every
// LLM response to detect prompt injection.
func GenerateCanary() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// VerifyCanary checks if the expected canary token appears in the response.
// Uses constant-time comparison at each candidate position to prevent
// timing side-channel attacks.
func VerifyCanary(response string, expected string) bool {
	if len(expected) != 64 || len(response) < 64 {
		return false
	}
	expectedBytes := []byte(expected)
	responseBytes := []byte(response)

	found := 0
	for i := 0; i <= len(responseBytes)-64; i++ {
		found |= subtle.ConstantTimeCompare(responseBytes[i:i+64], expectedBytes)
	}
	return found == 1
}
