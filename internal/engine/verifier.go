package engine

import (
	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/types"
)

// Verifier checks that the action hash hasn't been tampered with between
// evaluation and execution.
type Verifier struct{}

// NewVerifier creates a Verifier.
func NewVerifier() *Verifier { return &Verifier{} }

// Verify recomputes the action hash and compares it to the stored hash.
// Returns an error if they don't match (TOCTOU attack detected).
func (v *Verifier) Verify(action *types.ActionRequest) error {
	computed, err := crypto.HashAction(string(action.Type), action.Payload)
	if err != nil {
		return types.ErrHashMismatch
	}
	if computed != action.Hash {
		return types.ErrHashMismatch
	}
	return nil
}
