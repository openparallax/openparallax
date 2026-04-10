// Package tier0 implements the YAML policy engine for Shield's first evaluation tier.
// Policies define deny, verify, and allow rules that are matched against actions
// using glob patterns on file paths and action type lists.
package shield

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gobwas/glob"
	"github.com/openparallax/openparallax/platform"
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
	// MatchedPath is the first path that matched the rule's glob set, when
	// the rule has a path criterion. Empty for action-only rules.
	MatchedPath string
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

	if err := compileRules(raw.Deny); err != nil {
		return nil, fmt.Errorf("deny rules: %w", err)
	}
	if err := compileRules(raw.Verify); err != nil {
		return nil, fmt.Errorf("verify rules: %w", err)
	}
	if err := compileRules(raw.Allow); err != nil {
		return nil, fmt.Errorf("allow rules: %w", err)
	}

	return &PolicyEngine{
		deny:   raw.Deny,
		verify: raw.Verify,
		allow:  raw.Allow,
	}, nil
}

// Evaluate checks an action against policy rules.
// Order: deny -> verify -> allow -> NoMatch.
func (p *PolicyEngine) Evaluate(action *ActionRequest) PolicyResult {
	for _, rule := range p.deny {
		if matched, path := rule.matches(action); matched {
			return PolicyResult{Decision: Deny, Reason: rule.Name, MatchedPath: path}
		}
	}

	for _, rule := range p.verify {
		if matched, path := rule.matches(action); matched {
			return PolicyResult{Decision: Escalate, Reason: rule.Name, MatchedPath: path, EscalateTo: rule.TierOverride}
		}
	}

	for _, rule := range p.allow {
		if matched, path := rule.matches(action); matched {
			return PolicyResult{Decision: Allow, Reason: rule.Name, MatchedPath: path}
		}
	}

	return PolicyResult{Decision: NoMatch}
}

// DenyCount returns the number of deny rules.
func (p *PolicyEngine) DenyCount() int { return len(p.deny) }

// AllowCount returns the number of allow rules.
func (p *PolicyEngine) AllowCount() int { return len(p.allow) }

// matches checks if an action matches this rule's criteria. The second
// return value is the original (un-normalized) path that triggered the
// match, or "" for action-only rules.
func (r *policyRule) matches(action *ActionRequest) (bool, string) {
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
			return false, ""
		}
	}

	// Check path match — test ALL path fields (path, source, destination, etc.)
	// so that copy/move to a protected path is caught by the destination field.
	if len(r.compiled) > 0 {
		allPaths := extractPolicyPaths(action)
		if len(allPaths) == 0 {
			return false, ""
		}
		for _, path := range allPaths {
			normalized := filepath.ToSlash(platform.NormalizePath(path))
			// Bare filenames like "SOUL.md" have no directory component, so
			// glob patterns like "**/SOUL.md" won't match. Prepend "./" to
			// give the glob a path separator to work with.
			if !strings.Contains(normalized, "/") {
				normalized = "./" + normalized
			}
			for _, g := range r.compiled {
				if g.Match(normalized) {
					return true, path
				}
			}
		}
		return false, ""
	}

	return true, ""
}

// extractPolicyPaths returns all filesystem paths from an action's payload.
// Checks path, source, destination, dir, file, and target fields so that
// copy/move operations are caught by both source and destination.
func extractPolicyPaths(action *ActionRequest) []string {
	var paths []string
	for _, key := range []string{"path", "source", "destination", "dir", "file", "target"} {
		if v, ok := action.Payload[key].(string); ok && v != "" {
			paths = append(paths, v)
		}
	}
	return paths
}

// compileRules compiles glob patterns in each rule with tilde expansion
// and platform-normalized paths.
func compileRules(rules []policyRule) error {
	for i := range rules {
		rules[i].compiled = make([]glob.Glob, 0, len(rules[i].Paths))
		for _, pattern := range rules[i].Paths {
			expanded := platform.NormalizePath(pattern)
			g, err := glob.Compile(expanded, '/')
			if err != nil {
				return fmt.Errorf("rule %q: invalid glob %q: %w", rules[i].Name, pattern, err)
			}
			rules[i].compiled = append(rules[i].compiled, g)
		}
	}
	return nil
}
