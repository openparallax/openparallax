package tier1

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/openparallax/openparallax/internal/platform"
	"github.com/openparallax/openparallax/internal/types"
)

// HeuristicEngine evaluates actions against regex-based detection rules.
type HeuristicEngine struct {
	rules []compiledRule
}

type compiledRule struct {
	rule    platform.HeuristicRule
	pattern *regexp.Regexp
}

// NewHeuristicEngine creates a HeuristicEngine with platform-aware rules.
// All regex patterns are compiled at init time. Invalid patterns are skipped.
func NewHeuristicEngine() *HeuristicEngine {
	allRules := platform.ShellInjectionRules()
	allRules = append(allRules, CrossPlatformDetectionRules()...)

	compiled := make([]compiledRule, 0, len(allRules))
	for _, r := range allRules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			continue
		}
		compiled = append(compiled, compiledRule{rule: r, pattern: re})
	}

	return &HeuristicEngine{rules: compiled}
}

// RuleCount returns the total number of compiled rules.
func (h *HeuristicEngine) RuleCount() int {
	return len(h.rules)
}

// Evaluate checks an action against all heuristic rules.
// Only scans security-relevant fields (command, path, url, source, destination)
// to avoid false positives on file content being written.
func (h *HeuristicEngine) Evaluate(action *types.ActionRequest) *ClassifierResult {
	texts := []string{string(action.Type)}
	securityFields := []string{"command", "path", "source", "destination", "url", "pattern"}
	for _, key := range securityFields {
		if v, ok := action.Payload[key]; ok {
			texts = append(texts, fmt.Sprintf("%v", v))
		}
	}
	combined := strings.Join(texts, " ")

	var highestSeverity string
	var matchedRule string

	for _, cr := range h.rules {
		if cr.pattern.MatchString(combined) {
			if isHigherSeverity(cr.rule.Severity, highestSeverity) {
				highestSeverity = cr.rule.Severity
				matchedRule = cr.rule.Name
			}
		}
	}

	if highestSeverity == "" {
		return &ClassifierResult{
			Decision:   types.VerdictAllow,
			Confidence: 0.7,
			Reason:     "no heuristic rules matched",
			Source:     "heuristic",
		}
	}

	return &ClassifierResult{
		Decision:   types.VerdictBlock,
		Confidence: severityToConfidence(highestSeverity),
		Reason:     fmt.Sprintf("heuristic: %s (%s)", matchedRule, highestSeverity),
		Source:     "heuristic",
	}
}

func isHigherSeverity(a, b string) bool {
	order := map[string]int{"critical": 4, "high": 3, "medium": 2, "low": 1, "": 0}
	return order[a] > order[b]
}

func severityToConfidence(s string) float64 {
	switch s {
	case "critical":
		return 0.95
	case "high":
		return 0.85
	case "medium":
		return 0.7
	default:
		return 0.5
	}
}
