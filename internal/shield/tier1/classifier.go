package tier1

import (
	"context"
	"sync"

	"github.com/openparallax/openparallax/internal/types"
)

// ClassifierResult is the output of Tier 1 classification.
type ClassifierResult struct {
	// Decision is ALLOW or BLOCK.
	Decision types.VerdictDecision
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
}

// NewDualClassifier creates a DualClassifier. Pass nil for onnx if the
// sidecar is not available — heuristic-only is a valid operating mode.
func NewDualClassifier(onnx OnnxClient, threshold float64, heuristicEnabled bool) *DualClassifier {
	return &DualClassifier{
		onnx:             onnx,
		onnxThreshold:    threshold,
		heuristicEnabled: heuristicEnabled,
		rules:            NewHeuristicEngine(),
	}
}

// Classify runs both classifiers in parallel and returns the most severe result.
func (d *DualClassifier) Classify(ctx context.Context, action *types.ActionRequest) (*ClassifierResult, error) {
	var onnxResult, heuristicResult *ClassifierResult
	var wg sync.WaitGroup

	if d.onnx != nil && d.onnx.IsAvailable() {
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

// combine merges two classifier results, choosing the most severe.
func combine(onnx, heuristic *ClassifierResult) *ClassifierResult {
	if onnx == nil && heuristic == nil {
		return &ClassifierResult{
			Decision:   types.VerdictAllow,
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

	// BLOCK beats ALLOW.
	if onnx.Decision == types.VerdictBlock || heuristic.Decision == types.VerdictBlock {
		if onnx.Decision == types.VerdictBlock && heuristic.Decision == types.VerdictBlock {
			if onnx.Confidence > heuristic.Confidence {
				return onnx
			}
			return heuristic
		}
		if onnx.Decision == types.VerdictBlock {
			return onnx
		}
		return heuristic
	}

	if onnx.Confidence > heuristic.Confidence {
		return onnx
	}
	return heuristic
}
