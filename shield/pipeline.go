package shield

import (
	"context"
	"fmt"

	"github.com/openparallax/openparallax/llm"
)

// Config holds pipeline configuration.
type Config struct {
	PolicyFile          string
	OnnxThreshold       float64
	HeuristicEnabled    bool
	ClassifierEnabled   bool
	ClassifierMode      string // "" (heuristic-only), "local", or "sidecar"
	ClassifierAddr      string
	ClassifierSkipTypes []string // action types where ONNX is bypassed
	Evaluator           *EvaluatorConfig
	CanaryToken         string
	FailClosed          bool
	RateLimit           int
	VerdictTTL          int
	DailyBudget         int
	Log                 Logger
	// Metrics persists the Tier 2 daily budget across engine restarts.
	// Optional; nil keeps the legacy in-memory-only behavior.
	Metrics MetricsRecorder
}

// Pipeline is the evaluation pipeline. Created once, used for all evaluations.
type Pipeline struct {
	gateway *Gateway
}

// NewPipeline creates the evaluation pipeline from configuration.
// This is the entry point for both embedded (in Engine) and standalone (Shield binary) use.
func NewPipeline(cfg Config) (*Pipeline, error) {
	policyEngine, err := NewPolicyEngine(cfg.PolicyFile)
	if err != nil {
		return nil, fmt.Errorf("policy engine init failed: %w", err)
	}

	// The ONNX classifier is opt-in. By default Shield runs heuristic-only at
	// Tier 1, which is a valid and proven operating mode (run-010 attack data).
	// To enable ONNX, set shield.classifier_enabled and shield.classifier_mode
	// in workspace config (after running `openparallax get-classifier`).
	var onnxClient OnnxClient
	threshold := cfg.OnnxThreshold
	if threshold == 0 {
		threshold = 0.85
	}
	if cfg.ClassifierEnabled && cfg.ClassifierMode == "sidecar" && cfg.ClassifierAddr != "" {
		onnxClient = NewHTTPOnnxClient(cfg.ClassifierAddr)
		if cfg.Log != nil {
			cfg.Log.Info("onnx_classifier_loaded", "source", "sidecar", "addr", cfg.ClassifierAddr)
		}
	}
	dualClassifier := NewDualClassifier(onnxClient, threshold, cfg.HeuristicEnabled)
	dualClassifier.SetSkipTypes(cfg.ClassifierSkipTypes)

	var evaluator *Evaluator
	if cfg.Evaluator != nil && cfg.Evaluator.Provider != "" {
		evalProvider, provErr := llm.NewProvider(llm.Config{
			Provider:  cfg.Evaluator.Provider,
			Model:     cfg.Evaluator.Model,
			APIKeyEnv: cfg.Evaluator.APIKeyEnv,
			BaseURL:   cfg.Evaluator.BaseURL,
		})
		if provErr != nil {
			if cfg.Log != nil {
				cfg.Log.Warn("tier2_not_available", "error", provErr)
			}
		} else {
			evaluator = NewEvaluator(evalProvider, cfg.CanaryToken)
		}
	}

	log := cfg.Log
	if log == nil {
		log = nopLogger{}
	}

	gateway := NewGateway(GatewayConfig{
		Policy:      policyEngine,
		Classifier:  dualClassifier,
		Evaluator:   evaluator,
		FailClosed:  cfg.FailClosed,
		RateLimit:   cfg.RateLimit,
		VerdictTTL:  cfg.VerdictTTL,
		DailyBudget: cfg.DailyBudget,
		Log:         log,
		Metrics:     cfg.Metrics,
	})

	return &Pipeline{gateway: gateway}, nil
}

// Evaluate runs an ActionRequest through the evaluation pipeline.
func (p *Pipeline) Evaluate(ctx context.Context, action *ActionRequest) *Verdict {
	return p.gateway.Evaluate(ctx, action)
}

// Status returns the current operational state of the Shield.
func (p *Pipeline) Status() ShieldStatus {
	return p.gateway.Status()
}

// UpdateBudget changes the daily Tier 2 evaluation budget.
func (p *Pipeline) UpdateBudget(budget int) {
	p.gateway.UpdateBudget(budget)
}
