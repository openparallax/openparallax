package shield

import (
	"context"
	"sync"
)

// ClassifierResult is the output of Tier 1 classification.
type ClassifierResult struct {
	// Decision is ALLOW or BLOCK.
	Decision VerdictDecision
	// Confidence is the classification confidence (0.0-1.0).
	Confidence float64
	// Reason explains the classification.
	Reason string
	// Source identifies which classifier produced this result.
	Source string
}

// DualClassifier runs ONNX and heuristic classifiers in parallel.
// The most severe result wins (BLOCK beats ALLOW).
type DualClassifier struct {
	onnx             OnnxClient
	onnxThreshold    float64
	heuristicEnabled bool
	rules            *HeuristicEngine
	skipONNX         map[ActionType]bool
}

// NewDualClassifier creates a DualClassifier. Pass nil for onnx if the
// sidecar is not available — heuristic-only is a valid operating mode.
func NewDualClassifier(onnx OnnxClient, threshold float64, heuristicEnabled bool) *DualClassifier {
	return &DualClassifier{
		onnx:             onnx,
		onnxThreshold:    threshold,
		heuristicEnabled: heuristicEnabled,
		rules:            NewHeuristicEngine(),
		skipONNX:         map[ActionType]bool{},
	}
}

// SetSkipTypes configures action types where the ONNX classifier is bypassed
// because the model has been observed to over-fire on benign payloads.
// Heuristics still run for these types. Replaces any prior skip set.
func (d *DualClassifier) SetSkipTypes(types []string) {
	d.skipONNX = make(map[ActionType]bool, len(types))
	for _, t := range types {
		d.skipONNX[ActionType(t)] = true
	}
}

// HeuristicOnly runs only the AlwaysBlock subset of heuristic rules, bypassing
// the ONNX classifier and the broader heuristic ruleset. Used for Tier 2
// escalations where ONNX over-fires on legitimate non-read actions but a
// narrow set of high-precision rules (e.g., agent-internal enumeration) must
// still block deterministically. Returns nil if heuristics are disabled.
func (d *DualClassifier) HeuristicOnly(action *ActionRequest) *ClassifierResult {
	if !d.heuristicEnabled {
		return nil
	}
	return d.rules.EvaluateAlwaysBlock(action)
}

// Classify runs both classifiers in parallel and returns the most severe result.
func (d *DualClassifier) Classify(ctx context.Context, action *ActionRequest) (*ClassifierResult, error) {
	var onnxResult, heuristicResult *ClassifierResult
	var wg sync.WaitGroup

	if d.onnx != nil && d.onnx.IsAvailable() && !d.skipONNX[action.Type] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := d.onnx.Classify(ctx, action)
			if err == nil {
				onnxResult = result
			}
		}()
	}

	if d.heuristicEnabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			heuristicResult = d.rules.Evaluate(action)
		}()
	}

	wg.Wait()

	return combine(onnxResult, heuristicResult), nil
}

// decisionSeverity maps a verdict decision to a numeric severity for ranking.
func decisionSeverity(d VerdictDecision) int {
	switch d {
	case VerdictBlock:
		return 2
	case VerdictEscalate:
		return 1
	default:
		return 0
	}
}

// combine merges two classifier results, choosing the most severe.
func combine(onnx, heuristic *ClassifierResult) *ClassifierResult {
	if onnx == nil && heuristic == nil {
		return &ClassifierResult{
			Decision:   VerdictAllow,
			Confidence: 0.5,
			Reason:     "no classifier available",
			Source:     "none",
		}
	}

	if onnx == nil {
		return heuristic
	}
	if heuristic == nil {
		return onnx
	}

	// Severity ranking: BLOCK > ESCALATE > ALLOW.
	oSev := decisionSeverity(onnx.Decision)
	hSev := decisionSeverity(heuristic.Decision)

	switch {
	case oSev > hSev:
		return onnx
	case hSev > oSev:
		return heuristic
	default:
		// Same severity — return the higher-confidence result.
		if onnx.Confidence > heuristic.Confidence {
			return onnx
		}
		return heuristic
	}
}
