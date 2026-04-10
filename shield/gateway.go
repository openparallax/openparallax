package shield

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// formatPolicyReason renders a Tier 0 policy result for the LLM. The format
// is `<verb> [<rule>]: <action> on <path> matched a policy pattern` so the
// LLM can read which rule fired and which payload field tripped it. For
// action-only rules (no path criterion) the path clause is dropped.
func formatPolicyReason(verb string, actionType ActionType, r PolicyResult) string {
	if r.MatchedPath != "" {
		return fmt.Sprintf("%s [%s]: %s on %q matched a policy pattern", verb, r.Reason, actionType, r.MatchedPath)
	}
	return fmt.Sprintf("%s [%s]: %s matched a policy rule", verb, r.Reason, actionType)
}

// MetricsRecorder is the persistence sink for daily counters that must
// survive an engine restart. The shield package depends on the interface
// rather than internal/storage so it remains independently importable.
// A nil recorder is permitted; the gateway then degrades to in-memory
// counters and the daily budget resets every restart (the pre-fix behavior).
type MetricsRecorder interface {
	GetDailyMetric(metric string) int
	IncrementDailyMetric(metric string, delta int)
}

// shieldTier2UsedMetric is the metrics_daily key the gateway uses for the
// persisted Tier 2 evaluation counter. Kept as a constant so the engine,
// status reporter, and tests reference the same string.
const shieldTier2UsedMetric = "shield_tier2_used"

// GatewayConfig holds all tier implementations and settings.
type GatewayConfig struct {
	Policy      *PolicyEngine
	Classifier  *DualClassifier
	Evaluator   *Evaluator
	FailClosed  bool
	RateLimit   int
	VerdictTTL  int
	DailyBudget int
	Log         Logger
	// Metrics persists the Tier 2 daily budget across restarts. Optional.
	Metrics MetricsRecorder
}

// Gateway orchestrates the evaluation pipeline.
type Gateway struct {
	cfg         GatewayConfig
	rateLimiter *RateLimiter
	budgetCount int
	budgetDate  string
	mu          sync.Mutex
}

// NewGateway creates a Gateway. If cfg.Metrics is non-nil, the Tier 2 daily
// budget is seeded from the persisted counter so an engine restart mid-day
// does not hand the user a free budget refresh.
func NewGateway(cfg GatewayConfig) *Gateway {
	g := &Gateway{
		cfg:         cfg,
		rateLimiter: NewRateLimiter(cfg.RateLimit),
	}
	if cfg.Metrics != nil {
		g.budgetDate = time.Now().Format("2006-01-02")
		g.budgetCount = cfg.Metrics.GetDailyMetric(shieldTier2UsedMetric)
		if cfg.Log != nil {
			cfg.Log.Info("shield_budget_restored", "date", g.budgetDate,
				"used", g.budgetCount, "budget", cfg.DailyBudget)
		}
	}
	return g
}

// Evaluate routes an action through the appropriate tiers and returns a verdict.
func (g *Gateway) Evaluate(ctx context.Context, action *ActionRequest) *Verdict {
	// Rate limiting.
	if !g.rateLimiter.Allow() {
		g.cfg.Log.Info("shield_rate_limit", "action", action.Type)
		return g.block(action, 0, 1.0, "rate limit exceeded")
	}

	// Fast path: known-safe shell command prefix. Curated allowlist
	// of common dev workflow commands (git, npm, make, go, …) that
	// are safe regardless of arguments because they don't take
	// arbitrary path inputs. Single statements only — anything with
	// shell metacharacters falls through to normal evaluation.
	// Bypasses Tier 0/1/2 entirely; the user wins back the latency
	// and tokens of an LLM evaluation on every git status.
	if action.Type == ActionExecCommand && action.MinTier == 0 {
		if cmd, ok := action.Payload["command"].(string); ok && IsSafeCommand(cmd) {
			g.cfg.Log.Info("shield_safe_command_fast_path", "action", action.Type)
			return g.allow(action, 0, 1.0, "known-safe command prefix")
		}
	}

	// Tier 0: Policy engine.
	t0Result := g.cfg.Policy.Evaluate(action)
	switch t0Result.Decision {
	case Deny:
		g.cfg.Log.Info("shield_tier0_deny", "action", action.Type, "policy", t0Result.Reason)
		return g.block(action, 0, 1.0, formatPolicyReason("policy deny", action.Type, t0Result))
	case Allow:
		g.cfg.Log.Info("shield_tier0_allow", "action", action.Type, "policy", t0Result.Reason)
		// Only return immediately if no MinTier override requires higher evaluation.
		if action.MinTier <= 0 {
			return g.allow(action, 0, 1.0, formatPolicyReason("policy allow", action.Type, t0Result))
		}
		g.cfg.Log.Info("shield_mintier_override", "action", action.Type, "min_tier", action.MinTier)
	case Escalate:
		g.cfg.Log.Info("shield_tier0_escalate", "action", action.Type, "to_tier", t0Result.EscalateTo, "policy", t0Result.Reason)
	case NoMatch:
		g.cfg.Log.Debug("shield_tier0_nomatch", "action", action.Type)
	}

	// Determine minimum tier from multiple sources.
	minTier := 1

	// From protection layer (hardcoded).
	if action.MinTier > minTier {
		minTier = action.MinTier
	}

	// From Tier 0 escalation (policy).
	if t0Result.Decision == Escalate && t0Result.EscalateTo > minTier {
		minTier = t0Result.EscalateTo
	}

	// From action type defaults.
	actionMin := actionTypeMinTier(action.Type)
	if actionMin > minTier {
		minTier = actionMin
	}

	// Tier 1: Dual classifier. When the policy escalates to Tier 2+, we still
	// run a heuristic-only pre-check (deterministic high-precision rules) so
	// obvious attack patterns are blocked without burning a Tier 2 LLM call.
	// The ONNX classifier is skipped on Tier 2 escalations because it
	// over-fires on legitimate non-read actions.
	if minTier <= 1 {
		t1Result, err := g.cfg.Classifier.Classify(ctx, action)
		switch {
		case err != nil && g.cfg.FailClosed:
			g.cfg.Log.Warn("shield_tier1_error", "action", action.Type, "error", err)
			return g.block(action, 1, 0.5, "classifier error: "+err.Error())
		case err != nil:
			g.cfg.Log.Warn("shield_tier1_error_failopen", "action", action.Type, "error", err)
			// Continue to Tier 2 evaluation.
		case t1Result.Decision == VerdictBlock:
			g.cfg.Log.Info("shield_tier1_block", "action", action.Type, "reason", t1Result.Reason)
			return g.blockResult(action, 1, t1Result.Confidence, t1Result.Reason)
		case t1Result.Decision == VerdictEscalate:
			g.cfg.Log.Info("shield_tier1_escalate", "action", action.Type, "reason", t1Result.Reason)
			// Fall through to Tier 2.
		case minTier < 2:
			g.cfg.Log.Info("shield_tier1_allow", "action", action.Type, "confidence", t1Result.Confidence)
			return g.allow(action, 1, t1Result.Confidence, "classifier approved")
		}
	} else if hr := g.cfg.Classifier.HeuristicOnly(action); hr != nil && hr.Decision == VerdictBlock {
		g.cfg.Log.Info("shield_heuristic_precheck_block", "action", action.Type, "reason", hr.Reason)
		return g.blockResult(action, 1, hr.Confidence, hr.Reason)
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
		return g.allow(action, 1, 0.4, "budget exhausted (fail-open)")
	}

	t2Result, err := g.cfg.Evaluator.Evaluate(ctx, action)
	if err != nil {
		if g.cfg.FailClosed {
			g.cfg.Log.Warn("shield_tier2_error", "action", action.Type, "error", err)
			return g.block(action, 2, 0.5, "evaluator error: "+err.Error())
		}
		g.cfg.Log.Warn("shield_tier2_error_failopen", "action", action.Type, "error", err)
		return g.allow(action, 1, 0.3, "evaluator error (fail-open): "+err.Error())
	}

	g.cfg.Log.Info("shield_tier2_result", "action", action.Type, "decision", t2Result.Decision, "confidence", t2Result.Confidence)

	if t2Result.Decision == VerdictBlock {
		return g.blockResult(action, 2, t2Result.Confidence, t2Result.Reason)
	}

	if t2Result.Decision == VerdictEscalate {
		return g.escalate(action, 2, t2Result.Confidence, t2Result.Reason)
	}

	return g.allow(action, 2, t2Result.Confidence, t2Result.Reason)
}

func (g *Gateway) escalate(action *ActionRequest, tier int, conf float64, reason string) *Verdict {
	return &Verdict{
		Decision:    VerdictEscalate,
		Tier:        tier,
		Confidence:  conf,
		Reasoning:   reason,
		ActionHash:  action.Hash,
		EvaluatedAt: time.Now(),
		ExpiresAt:   time.Now().Add(time.Duration(g.cfg.VerdictTTL) * time.Second),
	}
}

func (g *Gateway) block(action *ActionRequest, tier int, conf float64, reason string) *Verdict {
	return &Verdict{
		Decision:    VerdictBlock,
		Tier:        tier,
		Confidence:  conf,
		Reasoning:   reason,
		ActionHash:  action.Hash,
		EvaluatedAt: time.Now(),
		ExpiresAt:   time.Now().Add(time.Duration(g.cfg.VerdictTTL) * time.Second),
	}
}

func (g *Gateway) blockResult(action *ActionRequest, tier int, conf float64, reason string) *Verdict {
	return g.block(action, tier, conf, reason)
}

func (g *Gateway) allow(action *ActionRequest, tier int, conf float64, reason string) *Verdict {
	return &Verdict{
		Decision:    VerdictAllow,
		Tier:        tier,
		Confidence:  conf,
		Reasoning:   reason,
		ActionHash:  action.Hash,
		EvaluatedAt: time.Now(),
		ExpiresAt:   time.Now().Add(time.Duration(g.cfg.VerdictTTL) * time.Second),
	}
}

// actionTypeMinTier returns the minimum evaluation tier based on action type.
func actionTypeMinTier(at ActionType) int {
	switch at {
	case ActionExecCommand:
		return 1
	case ActionSendEmail, ActionSendMessage, ActionHTTPRequest:
		return 1
	case ActionMemoryWrite:
		return 1
	default:
		return 0
	}
}

// ShieldStatus returns the current shield state: budget used, budget total, and whether tier2 is available.
type ShieldStatus struct {
	Active       bool `json:"active"`
	Tier2Used    int  `json:"tier2_used"`
	Tier2Budget  int  `json:"tier2_budget"`
	Tier2Enabled bool `json:"tier2_enabled"`
}

// Status returns the current operational state of the gateway.
func (g *Gateway) Status() ShieldStatus {
	g.mu.Lock()
	defer g.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	used := g.budgetCount
	if g.budgetDate != today {
		// Day has rolled over since the last in-memory check. With a
		// persistent recorder, today's usage may already be non-zero
		// (e.g. a previous process charged the budget earlier). Without
		// one, the day genuinely starts at zero.
		if g.cfg.Metrics != nil {
			used = g.cfg.Metrics.GetDailyMetric(shieldTier2UsedMetric)
		} else {
			used = 0
		}
	}
	return ShieldStatus{
		Active:       true,
		Tier2Used:    used,
		Tier2Budget:  g.cfg.DailyBudget,
		Tier2Enabled: g.cfg.Evaluator != nil,
	}
}

// UpdateBudget changes the daily Tier 2 evaluation budget.
func (g *Gateway) UpdateBudget(budget int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cfg.DailyBudget = budget
}

func (g *Gateway) checkBudget() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	if g.budgetDate != today {
		if g.cfg.Log != nil && g.budgetDate != "" {
			g.cfg.Log.Info("shield_budget_reset", "previous_date", g.budgetDate,
				"new_date", today, "previous_usage", g.budgetCount, "budget", g.cfg.DailyBudget)
		}
		g.budgetDate = today
		// Seed from persisted state on day rollover so a restart that
		// straddles midnight still reflects evaluations charged earlier
		// today by a previous process.
		if g.cfg.Metrics != nil {
			g.budgetCount = g.cfg.Metrics.GetDailyMetric(shieldTier2UsedMetric)
		} else {
			g.budgetCount = 0
		}
	}
	if g.budgetCount >= g.cfg.DailyBudget {
		return false
	}
	g.budgetCount++
	if g.cfg.Metrics != nil {
		g.cfg.Metrics.IncrementDailyMetric(shieldTier2UsedMetric, 1)
	}
	return true
}
