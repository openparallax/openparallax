package types

import "github.com/openparallax/openparallax/shield"

// SensitivityLevel is an alias for the public shield type.
type SensitivityLevel = shield.SensitivityLevel

// DataClassification is an alias for the public shield type.
type DataClassification = shield.DataClassification

// Sensitivity level constants — aliases to the public shield package.
const (
	SensitivityPublic       = shield.SensitivityPublic
	SensitivityInternal     = shield.SensitivityInternal
	SensitivityConfidential = shield.SensitivityConfidential
	SensitivityRestricted   = shield.SensitivityRestricted
	SensitivityCritical     = shield.SensitivityCritical
)
