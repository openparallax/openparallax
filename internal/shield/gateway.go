package shield

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/shield/tier0"
	"github.com/openparallax/openparallax/internal/shield/tier1"
	"github.com/openparallax/openparallax/internal/shield/tier2"
	"github.com/openparallax/openparallax/internal/types"
)

// GatewayConfig holds all tier implementations and settings.
type GatewayConfig struct {
	Policy      *tier0.PolicyEngine
	Classifier  *tier1.DualClassifier
	Evaluator   *tier2.Evaluator
	FailClosed  bool
	RateLimit   int
	VerdictTTL  int
	DailyBudget int
	Log         *logging.Logger
}

// Gateway orchestrates the evaluation pipeline.
type Gateway struct {
	cfg         GatewayConfig
	rateLimiter *RateLimiter
	budgetCount int
	budgetDate  string
	mu          sync.Mutex
}

// NewGateway creates a Gateway.
func NewGateway(cfg GatewayConfig) *Gateway {
	return &Gateway{
		cfg:         cfg,
		rateLimiter: NewRateLimiter(cfg.RateLimit),
	}
}

// Evaluate routes an action through the appropriate tiers and returns a verdict.
func (g *Gateway) Evaluate(ctx context.Context, action *types.ActionRequest) *types.Verdict {
	// Rate limiting.
	if !g.rateLimiter.Allow() {
		g.cfg.Log.Info("shield_rate_limit", "action", action.Type)
		return g.block(action, 0, 1.0, "rate limit exceeded")
	}

	// Self-protection: block access to Shield's own files.
	if g.isSelfProtected(action) {
		g.cfg.Log.Info("shield_self_protect", "action", action.Type, "path", extractPath(action))
		return g.block(action, 0, 1.0, "access to security-critical files is blocked")
	}

	// Tier 0: Policy engine.
	t0Result := g.cfg.Policy.Evaluate(action)
	switch t0Result.Decision {
	case tier0.Deny:
		g.cfg.Log.Info("shield_tier0_deny", "action", action.Type, "policy", t0Result.Reason)
		return g.block(action, 0, 1.0, fmt.Sprintf("policy deny: %s", t0Result.Reason))
	case tier0.Allow:
		g.cfg.Log.Info("shield_tier0_allow", "action", action.Type, "policy", t0Result.Reason)
		return g.allow(action, 0, 1.0, fmt.Sprintf("policy allow: %s", t0Result.Reason))
	case tier0.Escalate:
		g.cfg.Log.Info("shield_tier0_escalate", "action", action.Type, "to_tier", t0Result.EscalateTo, "policy", t0Result.Reason)
	case tier0.NoMatch:
		g.cfg.Log.Debug("shield_tier0_nomatch", "action", action.Type)
	}

	// Determine minimum tier from escalation.
	minTier := 1
	if t0Result.Decision == tier0.Escalate && t0Result.EscalateTo > minTier {
		minTier = t0Result.EscalateTo
	}

	// Action type minimum tier.
	actionMin := actionTypeMinTier(action.Type)
	if actionMin > minTier {
		minTier = actionMin
	}

	// Tier 1: Dual classifier.
	if minTier <= 1 {
		t1Result, err := g.cfg.Classifier.Classify(ctx, action)
		switch {
		case err != nil && g.cfg.FailClosed:
			g.cfg.Log.Warn("shield_tier1_error", "action", action.Type, "error", err)
			return g.block(action, 1, 0.5, "classifier error: "+err.Error())
		case err == nil && t1Result.Decision == types.VerdictBlock:
			g.cfg.Log.Info("shield_tier1_block", "action", action.Type, "reason", t1Result.Reason)
			return g.blockResult(action, 1, t1Result.Confidence, t1Result.Reason)
		case err == nil && minTier < 2:
			g.cfg.Log.Info("shield_tier1_allow", "action", action.Type, "confidence", t1Result.Confidence)
			return g.allow(action, 1, t1Result.Confidence, "classifier approved")
		}
	}

	// Tier 2: LLM evaluator.
	if g.cfg.Evaluator == nil {
		if g.cfg.FailClosed {
			g.cfg.Log.Info("shield_tier2_unavailable", "action", action.Type)
			return g.block(action, 2, 0.5, "Tier 2 evaluation required but not available")
		}
		return g.allow(action, 1, 0.5, "Tier 2 not available, allowing with reduced confidence")
	}

	if !g.checkBudget() {
		g.cfg.Log.Info("shield_budget_exhausted", "action", action.Type)
		if g.cfg.FailClosed {
			return g.block(action, 2, 0.5, "daily evaluation budget exhausted")
		}
	}

	t2Result, err := g.cfg.Evaluator.Evaluate(ctx, action)
	if err != nil {
		if g.cfg.FailClosed {
			g.cfg.Log.Warn("shield_tier2_error", "action", action.Type, "error", err)
			return g.block(action, 2, 0.5, "evaluator error: "+err.Error())
		}
	}

	g.cfg.Log.Info("shield_tier2_result", "action", action.Type, "decision", t2Result.Decision, "confidence", t2Result.Confidence)

	if t2Result.Decision == types.VerdictBlock {
		return g.blockResult(action, 2, t2Result.Confidence, t2Result.Reason)
	}

	return g.allow(action, 2, t2Result.Confidence, t2Result.Reason)
}

func (g *Gateway) block(action *types.ActionRequest, tier int, conf float64, reason string) *types.Verdict {
	return &types.Verdict{
		Decision:    types.VerdictBlock,
		Tier:        tier,
		Confidence:  conf,
		Reasoning:   reason,
		ActionHash:  action.Hash,
		EvaluatedAt: time.Now(),
		ExpiresAt:   time.Now().Add(time.Duration(g.cfg.VerdictTTL) * time.Second),
	}
}

func (g *Gateway) blockResult(action *types.ActionRequest, tier int, conf float64, reason string) *types.Verdict {
	return g.block(action, tier, conf, reason)
}

func (g *Gateway) allow(action *types.ActionRequest, tier int, conf float64, reason string) *types.Verdict {
	return &types.Verdict{
		Decision:    types.VerdictAllow,
		Tier:        tier,
		Confidence:  conf,
		Reasoning:   reason,
		ActionHash:  action.Hash,
		EvaluatedAt: time.Now(),
		ExpiresAt:   time.Now().Add(time.Duration(g.cfg.VerdictTTL) * time.Second),
	}
}

// actionTypeMinTier returns the minimum evaluation tier based on action type.
func actionTypeMinTier(at types.ActionType) int {
	switch at {
	case types.ActionExecCommand:
		return 1
	case types.ActionSendEmail, types.ActionSendMessage, types.ActionHTTPRequest:
		return 1
	case types.ActionMemoryWrite:
		return 1
	default:
		return 0
	}
}

// isSelfProtected blocks access to Shield's own config, canary, audit, and prompt files.
// Matches exact filenames or the .openparallax/ directory prefix to avoid false positives
// on filenames that happen to contain substrings like "audit".
func (g *Gateway) isSelfProtected(action *types.ActionRequest) bool {
	rawPath := extractPath(action)
	if rawPath == "" {
		return false
	}

	normalized := strings.ToLower(filepath.ToSlash(rawPath))
	filename := filepath.Base(normalized)

	// Exact filename matches for security-critical files.
	protectedFiles := []string{
		"canary.token",
		"evaluator-v1.md",
		"audit.jsonl",
		"openparallax.db",
		"config.yaml",
	}
	for _, f := range protectedFiles {
		if filename == f {
			return true
		}
	}

	// Block any path inside the .openparallax/ internal directory.
	if strings.Contains(normalized, ".openparallax/") {
		return true
	}

	return false
}

func (g *Gateway) checkBudget() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	if g.budgetDate != today {
		g.budgetDate = today
		g.budgetCount = 0
	}
	if g.budgetCount >= g.cfg.DailyBudget {
		return false
	}
	g.budgetCount++
	return true
}

// extractPath pulls the file path from an action's payload.
func extractPath(action *types.ActionRequest) string {
	if p, ok := action.Payload["path"].(string); ok && p != "" {
		return p
	}
	if p, ok := action.Payload["command"].(string); ok && p != "" {
		return p
	}
	return ""
}
