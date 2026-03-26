// Package parser extracts structured intent from natural language input using
// a two-stage approach: LLM extraction followed by deterministic validation.
package parser

import (
	"context"
	"fmt"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/types"
)

// Parser extracts structured intent from natural language input.
type Parser struct {
	extractor *Extractor
	validator *Validator
}

// New creates a Parser with the given LLM provider.
func New(provider llm.Provider) *Parser {
	return &Parser{
		extractor: NewExtractor(provider),
		validator: NewValidator(),
	}
}

// Parse takes raw user input and returns a StructuredIntent.
// Stage 1: LLM extracts goal, action, parameters from natural language.
// Stage 2: Deterministic validator enforces schema, normalizes, and cross-checks.
func (p *Parser) Parse(ctx context.Context, input string) (*types.StructuredIntent, error) {
	raw, err := p.extractor.Extract(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", types.ErrParserFailed, err)
	}

	intent, err := p.validator.Validate(raw, input)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", types.ErrParserFailed, err)
	}

	return intent, nil
}
