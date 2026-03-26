package shield

import (
	"context"
	"fmt"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/plog"
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
	Log              *plog.Logger
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
			cfg.Log.Log("shield", "Tier 2 evaluator not available: %s (continuing with Tier 0 + Tier 1)", provErr)
		} else {
			promptPath := cfg.PromptPath
			if promptPath == "" {
				promptPath = "prompts/evaluator-v1.md"
			}
			evaluator, err = tier2.NewEvaluator(evalProvider, promptPath, cfg.CanaryToken)
			if err != nil {
				cfg.Log.Log("shield", "Tier 2 evaluator init failed: %s (continuing with Tier 0 + Tier 1)", err)
				evaluator = nil
			}
		}
	}

	log := cfg.Log
	if log == nil {
		log = plog.New(false)
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
