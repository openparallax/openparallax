package types

import "github.com/openparallax/openparallax/shield"

// VerdictDecision is an alias for the public shield type.
type VerdictDecision = shield.VerdictDecision

// Verdict is an alias for the public shield type.
type Verdict = shield.Verdict

// Verdict decision constants — aliases to the public shield package.
const (
	VerdictAllow    = shield.VerdictAllow
	VerdictBlock    = shield.VerdictBlock
	VerdictEscalate = shield.VerdictEscalate
)
