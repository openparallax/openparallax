package ifc

import (
	"path/filepath"
	"strings"
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
	case SensitivityInternal:
		return !isExternalAction(destAction)
	case SensitivityConfidential:
		return isWorkspaceOnlyAction(destAction)
	case SensitivityRestricted, SensitivityCritical:
		return false
	}

	return true
}

func isExternalAction(at ActionType) bool {
	switch at {
	case ActionHTTPRequest, ActionSendEmail, ActionSendMessage:
		return true
	default:
		return false
	}
}

func isWorkspaceOnlyAction(at ActionType) bool {
	switch at {
	case ActionWriteFile, ActionReadFile, ActionListDir,
		ActionSearchFiles, ActionCreateDir,
		ActionCopyFile, ActionMoveFile,
		ActionMemoryWrite, ActionMemorySearch:
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
	return strings.Contains(normalized, "api_key") || strings.Contains(normalized, "apikey")
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
