package ifc

import (
	"path/filepath"
	"strings"

	"github.com/openparallax/openparallax/platform"
)

// ClassifySource returns a DataClassification for data read from a path.
func ClassifySource(path string) *DataClassification {
	normalized := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(normalized)

	if isCredentialPath(normalized, base) {
		return &DataClassification{
			Sensitivity: SensitivityCritical,
			SourcePath:  path,
			ContentType: "credential",
		}
	}

	if isSecurityPath(normalized) {
		return &DataClassification{
			Sensitivity: SensitivityRestricted,
			SourcePath:  path,
			ContentType: "credential",
		}
	}

	if isAgentConfigPath(normalized, base) {
		return &DataClassification{
			Sensitivity: SensitivityConfidential,
			SourcePath:  path,
			ContentType: "general",
		}
	}

	return &DataClassification{
		Sensitivity: SensitivityPublic,
		SourcePath:  path,
		ContentType: "general",
	}
}

// IsFlowAllowed checks if data with the given classification can flow to the
// destination action type. Returns false if the flow violates IFC policy.
func IsFlowAllowed(classification *DataClassification, destAction ActionType) bool {
	if classification == nil {
		return true
	}

	switch classification.Sensitivity {
	case SensitivityPublic:
		return true
	case SensitivityInternal, SensitivityConfidential:
		return !isExternalAction(destAction)
	case SensitivityRestricted, SensitivityCritical:
		return false
	}

	return true
}

// isExternalAction returns true for tool calls that send data outside the
// agent process — HTTP requests, outbound mail, channel messages. These are
// the only sinks that get blocked for Internal/Confidential data; everything
// else (file I/O, memory, MCP tools, future built-ins) is allowed by default.
// The denylist shape — rather than an allowlist of "known safe sinks" — keeps
// new tools from being silently blocked the moment they're added.
func isExternalAction(at ActionType) bool {
	switch at {
	case ActionHTTPRequest, ActionSendEmail, ActionSendMessage:
		return true
	default:
		return false
	}
}

func isCredentialPath(normalized, base string) bool {
	credFiles := []string{
		".env", ".env.local", ".env.production",
		"credentials", "credentials.json", "token.json",
		"secret", "secrets.yaml", "secrets.json",
		".netrc", ".pgpass", ".my.cnf",
		"service-account.json", "keyfile.json",
	}
	for _, f := range credFiles {
		if base == f {
			return true
		}
	}
	for _, s := range platform.RestrictedBasenameSuffixes() {
		if strings.HasSuffix(base, s) {
			return true
		}
	}
	return strings.Contains(normalized, "api_key") ||
		strings.Contains(normalized, "apikey") ||
		strings.Contains(normalized, "private_key") ||
		strings.Contains(normalized, "client_secret")
}

func isSecurityPath(normalized string) bool {
	secDirs := []string{".ssh/", ".aws/", ".gnupg/", ".kube/", ".docker/", ".password-store/", ".azure/"}
	for _, d := range secDirs {
		if strings.Contains(normalized, d) {
			return true
		}
	}
	return false
}

func isAgentConfigPath(normalized, base string) bool {
	agentFiles := []string{"config.yaml", "soul.md", "identity.md"}
	for _, f := range agentFiles {
		if base == f {
			return true
		}
	}
	return strings.Contains(normalized, ".openparallax/")
}
