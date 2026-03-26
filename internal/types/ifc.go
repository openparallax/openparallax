package types

// SensitivityLevel is the data sensitivity classification.
type SensitivityLevel int

const (
	// SensitivityPublic is data with no access restrictions.
	SensitivityPublic SensitivityLevel = 0
	// SensitivityInternal is data restricted to internal use.
	SensitivityInternal SensitivityLevel = 1
	// SensitivityConfidential is data with limited distribution.
	SensitivityConfidential SensitivityLevel = 2
	// SensitivityRestricted is data requiring elevated access controls.
	SensitivityRestricted SensitivityLevel = 3
	// SensitivityCritical is data with the highest protection requirements.
	SensitivityCritical SensitivityLevel = 4
)

// DataClassification is the IFC tag attached to data flowing through the pipeline.
type DataClassification struct {
	// Sensitivity is the data sensitivity level.
	Sensitivity SensitivityLevel `json:"sensitivity"`

	// SourcePath is where the data came from.
	SourcePath string `json:"source_path,omitempty"`

	// ContentType classifies the data content.
	// Values: "credential", "pii", "financial", "medical", "legal", "code", "general"
	ContentType string `json:"content_type,omitempty"`
}
