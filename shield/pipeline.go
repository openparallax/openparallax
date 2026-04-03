package shield

import (
	"context"
	"fmt"

	"github.com/openparallax/openparallax/llm"
)

// Config holds pipeline configuration.
type Config struct {
	PolicyFile       string
	OnnxThreshold    float64
	HeuristicEnabled bool
	ClassifierAddr   string
	Evaluator        *EvaluatorConfig
	CanaryToken      string
	PromptPath       string
	FailClosed       bool
	RateLimit        int
	VerdictTTL       int
	DailyBudget      int
	Log              Logger
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

	// Try local ONNX classifier first (in-process, fastest).
	// Fall back to HTTP sidecar if configured. Otherwise heuristic-only.
	var onnxClient OnnxClient
	threshold := cfg.OnnxThreshold
	if threshold == 0 {
		threshold = 0.85
	}
	localClient := NewLocalOnnxClient(threshold)
	switch {
	case localClient.IsAvailable():
		onnxClient = localClient
		if cfg.Log != nil {
			cfg.Log.Info("onnx_classifier_loaded", "source", "local", "threshold", threshold)
		}
	case cfg.ClassifierAddr != "":
		onnxClient = NewHTTPOnnxClient(cfg.ClassifierAddr)
		if cfg.Log != nil {
			cfg.Log.Info("onnx_classifier_loaded", "source", "http", "addr", cfg.ClassifierAddr)
		}
	default:
		if cfg.Log != nil {
			cfg.Log.Warn("onnx_classifier_unavailable",
				"message", "Shield running in heuristic-only mode. Run 'openparallax get-classifier' for enhanced protection.")
		}
	}
	dualClassifier := NewDualClassifier(onnxClient, threshold, cfg.HeuristicEnabled)

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
			promptPath := cfg.PromptPath
			if promptPath == "" {
				promptPath = "prompts/evaluator-v1.md"
			}
			evaluator, err = NewEvaluator(evalProvider, promptPath, cfg.CanaryToken)
			if err != nil {
				if cfg.Log != nil {
					cfg.Log.Warn("tier2_init_failed", "error", err)
				}
				evaluator = nil
			}
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
