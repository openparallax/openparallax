package parser

import (
	"fmt"
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
func (v *Validator) Validate(raw *RawIntent, originalInput string) (*types.StructuredIntent, error) {
	goal := v.taxonomy.Normalize(raw.Goal)
	action := types.ActionType(strings.ToLower(raw.Action))
	destructive := raw.Destructive || v.keywords.IsDestructive(originalInput)

	params := flattenParams(raw.Parameters)
	sensitivity := v.sensitivity.Evaluate(params)
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
		Parameters:    params,
		Confidence:    confidence,
		Destructive:   destructive,
		Sensitivity:   sensitivity,
		RawInput:      originalInput,
	}, nil
}

// flattenParams converts map[string]any to map[string]string by stringifying values.
func flattenParams(raw map[string]any) map[string]string {
	result := make(map[string]string, len(raw))
	for k, v := range raw {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}
