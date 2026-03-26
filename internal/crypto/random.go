package crypto

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/google/uuid"
)

// RandomHex generates n random bytes and returns them as a hex string.
func RandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// NewID generates a new UUID v4 string.
func NewID() string {
	return uuid.New().String()
}
