package parser

import (
	"strings"

	"github.com/openparallax/openparallax/internal/platform"
	"github.com/openparallax/openparallax/internal/types"
)

// SensitivityLookup evaluates data sensitivity based on file paths and content markers.
type SensitivityLookup struct {
	sensitivePaths []string
}

// NewSensitivityLookup creates a SensitivityLookup with platform-aware paths.
func NewSensitivityLookup() *SensitivityLookup {
	return &SensitivityLookup{
		sensitivePaths: platform.SensitivePaths(),
	}
}

// Evaluate returns the sensitivity level based on parameters.
// Checks file paths against known sensitive locations and credential patterns.
func (s *SensitivityLookup) Evaluate(params map[string]string) types.SensitivityLevel {
	path := params["path"]
	if path == "" {
		path = params["file"]
	}
	if path == "" {
		return types.SensitivityPublic
	}

	normalized := platform.NormalizePath(path)

	for _, sp := range s.sensitivePaths {
		spNorm := platform.NormalizePath(sp)
		if strings.HasPrefix(normalized, spNorm) || normalized == spNorm {
			return types.SensitivityCritical
		}
	}

	lower := strings.ToLower(normalized)
	credentialPatterns := []string{
		"id_rsa", "id_ed25519", "id_ecdsa", ".pem", ".key", ".p12", ".pfx",
		".env", "credentials", "secrets", "token", "password", ".rdp", ".ppk",
		"ntuser.dat", ".vnc",
	}
	for _, pat := range credentialPatterns {
		if strings.Contains(lower, pat) {
			return types.SensitivityCritical
		}
	}

	financialPatterns := []string{"financial", "tax", "bank", "invoice", "salary", "ssn", "passport"}
	for _, pat := range financialPatterns {
		if strings.Contains(lower, pat) {
			return types.SensitivityRestricted
		}
	}

	return types.SensitivityPublic
}
