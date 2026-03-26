package parser

import (
	"strings"

	"github.com/openparallax/openparallax/internal/types"
)

// Validator performs deterministic validation and normalization of parsed intent.
type Validator struct {
	keywords    *KeywordDetector
	sensitivity *SensitivityLookup
	taxonomy    *GoalTaxonomy
}

// NewValidator creates a Validator.
func NewValidator() *Validator {
	return &Validator{
		keywords:    NewKeywordDetector(),
		sensitivity: NewSensitivityLookup(),
		taxonomy:    NewGoalTaxonomy(),
	}
}

// Validate takes a RawIntent from the LLM and produces a validated StructuredIntent.
// It normalizes the goal type, cross-checks destructive flags, clamps confidence,
// and overrides sensitivity based on file paths.
func (v *Validator) Validate(raw *RawIntent, originalInput string) (*types.StructuredIntent, error) {
	goal := v.taxonomy.Normalize(raw.Goal)

	action := types.ActionType(strings.ToLower(raw.Action))

	destructive := raw.Destructive || v.keywords.IsDestructive(originalInput)

	sensitivity := v.sensitivity.Evaluate(raw.Parameters)
	if destructive && sensitivity < types.SensitivityRestricted {
		sensitivity = types.SensitivityRestricted
	}

	confidence := raw.Confidence
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}

	if goal == types.GoalConversation && action == "" {
		action = "conversation"
	}

	return &types.StructuredIntent{
		Goal:          goal,
		PrimaryAction: action,
		Parameters:    raw.Parameters,
		Confidence:    confidence,
		Destructive:   destructive,
		Sensitivity:   sensitivity,
		RawInput:      originalInput,
	}, nil
}
