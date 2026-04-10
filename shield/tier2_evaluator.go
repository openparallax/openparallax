package shield

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/llm"
)

// evaluatorTimeout is the maximum time for a Tier 2 LLM evaluation call.
// If the LLM provider is unresponsive, the evaluation fails closed (BLOCK).
const evaluatorTimeout = 30 * time.Second

// evaluatorMaxTokens is the LLM token cap for Tier 2 evaluation calls.
const evaluatorMaxTokens = 500

// Evaluator uses an independent LLM to evaluate actions for security risks.
type Evaluator struct {
	llm         llm.Provider
	prompt      string
	canaryToken string
	promptHash  string
}

// NewEvaluator creates a Tier 2 evaluator. The evaluator prompt is compiled
// into the binary and cannot be tampered with at runtime. The canary token
// is injected at construction time.
func NewEvaluator(provider llm.Provider, canaryToken string) *Evaluator {
	prompt, promptHash := LoadPrompt(canaryToken)
	return &Evaluator{
		llm:         provider,
		prompt:      prompt,
		canaryToken: canaryToken,
		promptHash:  promptHash,
	}
}

// PromptHash returns the SHA-256 hash of the original evaluator prompt file
// (before canary injection).
func (e *Evaluator) PromptHash() string {
	return e.promptHash
}

// Evaluate sends the action to the LLM evaluator and parses the response.
// Verifies the canary token appears in the response (detects injection).
func (e *Evaluator) Evaluate(ctx context.Context, action *ActionRequest) (*EvalResult, error) {
	userPrompt := fmt.Sprintf(`Evaluate this action for security:

Action type: %s
Payload: %s

Respond with ONLY a JSON object:
{
  "decision": "ALLOW", "BLOCK", or "ESCALATE",
  "confidence": 0.0-1.0,
  "reasoning": "brief explanation",
  "canary": "%s"
}

Use ESCALATE when the action is genuinely ambiguous and a human operator
should make the final call — e.g., the intent is unclear without more
context, the action has irreversible side effects but no obvious malice,
or the security implications depend on facts you cannot verify.`, action.Type, formatPayload(action.Payload), e.canaryToken)

	evalCtx, cancel := context.WithTimeout(ctx, evaluatorTimeout)
	defer cancel()

	response, err := e.llm.Complete(evalCtx, userPrompt, llm.WithSystem(e.prompt), llm.WithMaxTokens(evaluatorMaxTokens))
	if err != nil {
		return nil, fmt.Errorf("evaluator LLM call failed: %w", err)
	}

	// Canary missing = possible injection of the evaluator itself.
	if !crypto.VerifyCanary(response, e.canaryToken) {
		return &EvalResult{
			Decision:   VerdictBlock,
			Confidence: 1.0,
			Reason:     "canary token missing from evaluator response — possible evaluator injection",
		}, nil
	}

	result, err := ParseEvalResponse(response)
	if err != nil {
		return &EvalResult{
			Decision:   VerdictBlock,
			Confidence: 0.5,
			Reason:     "failed to parse evaluator response: " + err.Error(),
		}, nil
	}

	return result, nil
}

func formatPayload(payload map[string]any) string {
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data)
}
