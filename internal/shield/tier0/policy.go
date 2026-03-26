// Package tier0 implements the YAML policy engine for Shield's first evaluation tier.
// Policies define deny, verify, and allow rules that are matched against actions
// using glob patterns on file paths and action type lists.
package tier0

import (
	"os"

	"github.com/gobwas/glob"
	"github.com/openparallax/openparallax/internal/platform"
	"github.com/openparallax/openparallax/internal/types"
	"gopkg.in/yaml.v3"
)

// Decision is the result of a Tier 0 policy evaluation.
type Decision int

const (
	// NoMatch indicates no policy rule matched the action.
	NoMatch Decision = iota
	// Allow indicates a policy allow rule matched.
	Allow
	// Deny indicates a policy deny rule matched.
	Deny
	// Escalate indicates a policy verify rule matched, requiring higher-tier evaluation.
	Escalate
)

// PolicyResult is the output of policy evaluation.
type PolicyResult struct {
	// Decision is the evaluation outcome.
	Decision Decision
	// Reason is the name of the matched rule.
	Reason string
	// EscalateTo is the minimum evaluation tier (only set when Decision == Escalate).
	EscalateTo int
}

// PolicyEngine evaluates actions against YAML policy rules.
type PolicyEngine struct {
	deny   []policyRule
	verify []policyRule
	allow  []policyRule
}

// policyRule is a single rule in a policy file.
type policyRule struct {
	Name         string   `yaml:"name"`
	ActionTypes  []string `yaml:"action_types,omitempty"`
	Paths        []string `yaml:"paths,omitempty"`
	TierOverride int      `yaml:"tier_override,omitempty"`
	compiled     []glob.Glob
}

// NewPolicyEngine loads a YAML policy file and creates the engine.
func NewPolicyEngine(policyPath string) (*PolicyEngine, error) {
	data, err := os.ReadFile(policyPath)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Deny   []policyRule `yaml:"deny"`
		Verify []policyRule `yaml:"verify"`
		Allow  []policyRule `yaml:"allow"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	compileRules(raw.Deny)
	compileRules(raw.Verify)
	compileRules(raw.Allow)

	return &PolicyEngine{
		deny:   raw.Deny,
		verify: raw.Verify,
		allow:  raw.Allow,
	}, nil
}

// Evaluate checks an action against policy rules.
// Order: deny -> verify -> allow -> NoMatch.
func (p *PolicyEngine) Evaluate(action *types.ActionRequest) PolicyResult {
	for _, rule := range p.deny {
		if rule.matches(action) {
			return PolicyResult{Decision: Deny, Reason: rule.Name}
		}
	}

	for _, rule := range p.verify {
		if rule.matches(action) {
			return PolicyResult{Decision: Escalate, Reason: rule.Name, EscalateTo: rule.TierOverride}
		}
	}

	for _, rule := range p.allow {
		if rule.matches(action) {
			return PolicyResult{Decision: Allow, Reason: rule.Name}
		}
	}

	return PolicyResult{Decision: NoMatch}
}

// DenyCount returns the number of deny rules.
func (p *PolicyEngine) DenyCount() int { return len(p.deny) }

// AllowCount returns the number of allow rules.
func (p *PolicyEngine) AllowCount() int { return len(p.allow) }

// matches checks if an action matches this rule's criteria.
func (r *policyRule) matches(action *types.ActionRequest) bool {
	// Check action type match.
	if len(r.ActionTypes) > 0 {
		matched := false
		for _, at := range r.ActionTypes {
			if at == string(action.Type) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check path match.
	if len(r.compiled) > 0 {
		path := extractPath(action)
		if path == "" {
			return false
		}
		normalized := platform.NormalizePath(path)
		matched := false
		for _, g := range r.compiled {
			if g.Match(normalized) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// extractPath pulls the file path from an action's payload.
func extractPath(action *types.ActionRequest) string {
	if p, ok := action.Payload["path"].(string); ok && p != "" {
		return p
	}
	if p, ok := action.Payload["source"].(string); ok && p != "" {
		return p
	}
	if p, ok := action.Payload["command"].(string); ok && p != "" {
		return p
	}
	return ""
}

// compileRules compiles glob patterns in each rule with tilde expansion
// and platform-normalized paths.
func compileRules(rules []policyRule) {
	for i := range rules {
		rules[i].compiled = make([]glob.Glob, 0, len(rules[i].Paths))
		for _, pattern := range rules[i].Paths {
			expanded := platform.NormalizePath(pattern)
			g, err := glob.Compile(expanded, '/')
			if err == nil {
				rules[i].compiled = append(rules[i].compiled, g)
			}
		}
	}
}
