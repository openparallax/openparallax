package shield

import (
	"path/filepath"
	"strings"

	"github.com/openparallax/openparallax/internal/types"
)

// ClassifySource returns a DataClassification for data read from a path.
func ClassifySource(path string) *types.DataClassification {
	normalized := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(normalized)

	// Credentials and keys.
	if isCredentialPath(normalized, base) {
		return &types.DataClassification{
			Sensitivity: types.SensitivityCritical,
			SourcePath:  path,
			ContentType: "credential",
		}
	}

	// SSH, GPG, cloud config.
	if isSecurityPath(normalized) {
		return &types.DataClassification{
			Sensitivity: types.SensitivityRestricted,
			SourcePath:  path,
			ContentType: "credential",
		}
	}

	// Agent internal config.
	if isAgentConfigPath(normalized, base) {
		return &types.DataClassification{
			Sensitivity: types.SensitivityConfidential,
			SourcePath:  path,
			ContentType: "general",
		}
	}

	// Default: public.
	return &types.DataClassification{
		Sensitivity: types.SensitivityPublic,
		SourcePath:  path,
		ContentType: "general",
	}
}

// IsFlowAllowed checks if data with the given classification can flow to the
// destination action type. Returns false if the flow violates IFC policy.
func IsFlowAllowed(classification *types.DataClassification, destAction types.ActionType) bool {
	if classification == nil {
		return true
	}

	switch classification.Sensitivity {
	case types.SensitivityPublic:
		return true

	case types.SensitivityInternal:
		// Can flow to workspace, memory, local tools. Not to external services.
		return !isExternalAction(destAction)

	case types.SensitivityConfidential:
		// Can flow to workspace only. Not to HTTP, email, shell, external.
		return isWorkspaceOnlyAction(destAction)

	case types.SensitivityRestricted, types.SensitivityCritical:
		// Cannot flow anywhere beyond display. Read-only.
		return false
	}

	return true
}

func isExternalAction(at types.ActionType) bool {
	switch at {
	case types.ActionHTTPRequest, types.ActionSendEmail, types.ActionSendMessage:
		return true
	default:
		return false
	}
}

func isWorkspaceOnlyAction(at types.ActionType) bool {
	switch at {
	case types.ActionWriteFile, types.ActionReadFile, types.ActionListDir,
		types.ActionSearchFiles, types.ActionCreateDir,
		types.ActionCopyFile, types.ActionMoveFile,
		types.ActionMemoryWrite, types.ActionMemorySearch:
		return true
	default:
		return false
	}
}

func isCredentialPath(normalized, base string) bool {
	credFiles := []string{
		".env", "credentials", "credentials.json", "token.json",
		"secret", "secrets.yaml", "secrets.json",
	}
	for _, f := range credFiles {
		if base == f {
			return true
		}
	}
	if strings.Contains(normalized, "api_key") || strings.Contains(normalized, "apikey") {
		return true
	}
	return false
}

func isSecurityPath(normalized string) bool {
	secDirs := []string{".ssh/", ".aws/", ".gnupg/", ".kube/", ".docker/"}
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
