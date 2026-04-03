package shield

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/openparallax/openparallax/platform"
)

// MetadataEnricher adds data classification to ActionRequests before Shield
// evaluation. It runs keyword detection for destructive patterns and
// sensitivity evaluation for file paths.
type MetadataEnricher struct {
	keywords       []*regexp.Regexp
	sensitivePaths []string
}

// NewMetadataEnricher creates a MetadataEnricher with platform-aware paths.
func NewMetadataEnricher() *MetadataEnricher {
	patterns := []string{
		`(?i)\b(delete|remove|destroy|erase|wipe|purge)\b.*\b(all|every|entire)\b`,
		`(?i)\brm\s+-rf?\b`,
		`(?i)\bformat\s+(c:|d:|the\s+drive)\b`,
		`(?i)\bdrop\s+(table|database|schema)\b`,
		`(?i)\btruncate\s+table\b`,
		`(?i)\b(overwrite|replace)\s+(all|every)\b`,
		`(?i)\bfactory\s+reset\b`,
		`(?i)\bgit\s+push\s+(-f|--force)\b`,
		`(?i)\bgit\s+reset\s+--hard\b`,
		`(?i)\bchmod\s+777\b`,
		`(?i)\bsudo\s+rm\b`,
		`(?i)\bmkfs\b`,
		`(?i)\bdd\s+if=.*of=\b`,
		`(?i)\b(shutdown|reboot|halt)\s+(-h|now)\b`,
		`(?i)\bkill\s+-9\s+(-1|1)\b`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		compiled = append(compiled, regexp.MustCompile(p))
	}

	return &MetadataEnricher{
		keywords:       compiled,
		sensitivePaths: platform.SensitivePaths(),
	}
}

// Enrich sets DataClassification on the action based on keyword and path analysis.
func (m *MetadataEnricher) Enrich(action *ActionRequest) {
	sensitivity := SensitivityPublic

	// Check path sensitivity.
	path := extractActionPath(action)
	if path != "" {
		sensitivity = m.evaluatePathSensitivity(path)
	}

	// Check keyword destructiveness in payload.
	payloadText := formatPayloadText(action.Payload)
	if m.isDestructive(payloadText) && sensitivity < SensitivityRestricted {
		sensitivity = SensitivityRestricted
	}

	if sensitivity > SensitivityPublic {
		action.DataClassification = &DataClassification{
			Sensitivity: sensitivity,
			SourcePath:  path,
		}
	}
}

func (m *MetadataEnricher) isDestructive(text string) bool {
	for _, p := range m.keywords {
		if p.MatchString(text) {
			return true
		}
	}
	return false
}

func (m *MetadataEnricher) evaluatePathSensitivity(path string) SensitivityLevel {
	normalized := platform.NormalizePath(path)

	for _, sp := range m.sensitivePaths {
		spNorm := platform.NormalizePath(sp)
		if strings.HasPrefix(normalized, spNorm) || normalized == spNorm {
			return SensitivityCritical
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
			return SensitivityCritical
		}
	}

	financialPatterns := []string{"financial", "tax", "bank", "invoice", "salary", "ssn", "passport"}
	for _, pat := range financialPatterns {
		if strings.Contains(lower, pat) {
			return SensitivityRestricted
		}
	}

	return SensitivityPublic
}

func extractActionPath(action *ActionRequest) string {
	if p, ok := action.Payload["path"].(string); ok && p != "" {
		return p
	}
	if p, ok := action.Payload["source"].(string); ok && p != "" {
		return p
	}
	return ""
}

func formatPayloadText(payload map[string]any) string {
	var parts []string
	for _, v := range payload {
		parts = append(parts, fmt.Sprintf("%v", v))
	}
	return strings.Join(parts, " ")
}
