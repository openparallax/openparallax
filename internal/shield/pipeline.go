package shield

import (
	"context"
	"fmt"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/shield/tier0"
	"github.com/openparallax/openparallax/internal/shield/tier1"
	"github.com/openparallax/openparallax/internal/shield/tier2"
	"github.com/openparallax/openparallax/internal/types"
)

// Config holds pipeline configuration.
type Config struct {
	PolicyFile       string
	OnnxThreshold    float64
	HeuristicEnabled bool
	ClassifierAddr   string
	Evaluator        *types.EvaluatorConfig
	CanaryToken      string
	PromptPath       string
	FailClosed       bool
	RateLimit        int
	VerdictTTL       int
	DailyBudget      int
	Log              *logging.Logger
}

// Pipeline is the evaluation pipeline. Created once, used for all evaluations.
type Pipeline struct {
	gateway *Gateway
}

// NewPipeline creates the evaluation pipeline from configuration.
// This is the entry point for both embedded (in Engine) and standalone (Shield binary) use.
func NewPipeline(cfg Config) (*Pipeline, error) {
	policyEngine, err := tier0.NewPolicyEngine(cfg.PolicyFile)
	if err != nil {
		return nil, fmt.Errorf("policy engine init failed: %w", err)
	}

	var onnxClient tier1.OnnxClient
	if cfg.ClassifierAddr != "" {
		onnxClient = tier1.NewHTTPOnnxClient(cfg.ClassifierAddr)
	}
	dualClassifier := tier1.NewDualClassifier(onnxClient, cfg.OnnxThreshold, cfg.HeuristicEnabled)

	var evaluator *tier2.Evaluator
	if cfg.Evaluator != nil && cfg.Evaluator.Provider != "" {
		evalProvider, provErr := llm.NewProvider(types.LLMConfig{
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
			evaluator, err = tier2.NewEvaluator(evalProvider, promptPath, cfg.CanaryToken)
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
		log = logging.Nop()
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
func (p *Pipeline) Evaluate(ctx context.Context, action *types.ActionRequest) *types.Verdict {
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
