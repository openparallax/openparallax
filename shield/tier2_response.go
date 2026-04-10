package shield

import (
	"encoding/json"
	"strings"
)

// EvalResult is the Tier 2 evaluation result.
type EvalResult struct {
	// Decision is ALLOW or BLOCK.
	Decision VerdictDecision
	// Confidence is the evaluator's confidence (0.0-1.0).
	Confidence float64
	// Reason explains the evaluation.
	Reason string
}

// ParseEvalResponse parses the evaluator's JSON response.
// Strips markdown code fences. Returns BLOCK on parse failure (fail-closed).
func ParseEvalResponse(response string) (*EvalResult, error) {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var parsed struct {
		Decision   string  `json:"decision"`
		Confidence float64 `json:"confidence"`
		Reasoning  string  `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return nil, err
	}

	decision := VerdictAllow
	switch strings.ToUpper(parsed.Decision) {
	case "BLOCK":
		decision = VerdictBlock
	case "ESCALATE":
		decision = VerdictEscalate
	}

	return &EvalResult{
		Decision:   decision,
		Confidence: parsed.Confidence,
		Reason:     parsed.Reasoning,
	}, nil
}
