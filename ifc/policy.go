package ifc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Mode is the IFC enforcement mode.
type Mode string

const (
	// ModeEnforce blocks actions that violate the policy.
	ModeEnforce Mode = "enforce"
	// ModeAudit logs violations but does not block.
	ModeAudit Mode = "audit"
)

// Decision is the outcome of an IFC flow check.
type Decision string

const (
	// DecisionAllow permits the data flow.
	DecisionAllow Decision = "allow"
	// DecisionBlock prevents the data flow.
	DecisionBlock Decision = "block"
	// DecisionEscalate routes the action to Shield Tier 3 (human approval).
	DecisionEscalate Decision = "escalate"
)

// Policy is a compiled IFC policy ready for evaluation.
type Policy struct {
	Mode    Mode
	Sources []SourceRule
	Sinks   map[string][]ActionType // category name → action types
	Rules   map[SensitivityLevel]map[string]Decision
	// sinkLookup maps each action type to its category for O(1) Decide().
	sinkLookup map[ActionType]string
}

// SourceRule classifies data based on where it came from.
type SourceRule struct {
	Name        string           `yaml:"name"`
	Sensitivity SensitivityLevel `yaml:"-"`
	RawSens     string           `yaml:"sensitivity"`
	Match       SourceMatch      `yaml:"match"`
}

// SourceMatch defines the path-matching criteria for a source rule.
type SourceMatch struct {
	BasenameIn       []string `yaml:"basename_in"`
	BasenameNotIn    []string `yaml:"basename_not_in"`
	BasenameSuffixIn []string `yaml:"basename_suffix_in"`
	BasenameContains []string `yaml:"basename_contains"`
	PathContains     []string `yaml:"path_contains"`
	PathIn           []string `yaml:"path_in"`
	BasenameRegex    string   `yaml:"basename_regex"`
}

// rawPolicy is the YAML deserialization target.
type rawPolicy struct {
	Mode    string                       `yaml:"mode"`
	Sources []SourceRule                 `yaml:"sources"`
	Sinks   map[string][]string          `yaml:"sinks"`
	Rules   map[string]map[string]string `yaml:"rules"`
}

// LoadPolicy parses and validates an IFC policy from a YAML file.
func LoadPolicy(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ifc policy %s: %w", path, err)
	}
	return ParsePolicy(data)
}

// ParsePolicy parses and validates an IFC policy from YAML bytes.
func ParsePolicy(data []byte) (*Policy, error) {
	var raw rawPolicy
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse ifc policy: %w", err)
	}

	// Validate and parse mode.
	mode := Mode(raw.Mode)
	if mode != ModeEnforce && mode != ModeAudit {
		return nil, fmt.Errorf("ifc policy: invalid mode %q (must be enforce or audit)", raw.Mode)
	}

	// Parse source sensitivity levels.
	for i := range raw.Sources {
		sl, err := parseSensitivity(raw.Sources[i].RawSens)
		if err != nil {
			return nil, fmt.Errorf("ifc policy source %q: %w", raw.Sources[i].Name, err)
		}
		raw.Sources[i].Sensitivity = sl
	}

	// Parse sinks.
	sinks := make(map[string][]ActionType)
	sinkLookup := make(map[ActionType]string)
	for category, actions := range raw.Sinks {
		for _, a := range actions {
			at := ActionType(a)
			if existing, ok := sinkLookup[at]; ok {
				return nil, fmt.Errorf("ifc policy: action %q appears in sink categories %q and %q", a, existing, category)
			}
			sinks[category] = append(sinks[category], at)
			sinkLookup[at] = category
		}
	}

	// Parse rules.
	rules := make(map[SensitivityLevel]map[string]Decision)
	for sensName, sinkDecisions := range raw.Rules {
		sl, err := parseSensitivity(sensName)
		if err != nil {
			return nil, fmt.Errorf("ifc policy rules: %w", err)
		}
		row := make(map[string]Decision)
		for sinkCat, decision := range sinkDecisions {
			d := Decision(decision)
			if d != DecisionAllow && d != DecisionBlock && d != DecisionEscalate {
				return nil, fmt.Errorf("ifc policy rules[%s][%s]: invalid decision %q", sensName, sinkCat, decision)
			}
			if _, ok := sinks[sinkCat]; !ok {
				return nil, fmt.Errorf("ifc policy rules[%s]: unknown sink category %q", sensName, sinkCat)
			}
			row[sinkCat] = d
		}
		// Every sink category must have a decision.
		for cat := range sinks {
			if _, ok := row[cat]; !ok {
				return nil, fmt.Errorf("ifc policy rules[%s]: missing decision for sink category %q", sensName, cat)
			}
		}
		rules[sl] = row
	}

	return &Policy{
		Mode:       mode,
		Sources:    raw.Sources,
		Sinks:      sinks,
		Rules:      rules,
		sinkLookup: sinkLookup,
	}, nil
}

// Classify returns the sensitivity classification for a path. First matching
// source rule wins. Returns nil (public) if no rule matches or the path is empty.
func (p *Policy) Classify(path string) *DataClassification {
	if path == "" {
		return nil
	}
	normalized := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(normalized)

	for _, rule := range p.Sources {
		if matchSource(rule.Match, normalized, base) {
			if rule.Sensitivity == SensitivityPublic {
				return nil
			}
			return &DataClassification{
				Sensitivity: rule.Sensitivity,
				SourcePath:  path,
			}
		}
	}
	return nil
}

// Decide determines whether data with the given classification can flow to
// the given action type. Returns DecisionAllow if the classification is nil.
// For unknown action types (MCP tools, future built-ins), returns DecisionAllow
// — the denylist principle ensures new tools are allowed by default.
func (p *Policy) Decide(classification *DataClassification, action ActionType) Decision {
	if classification == nil {
		return DecisionAllow
	}
	category, ok := p.sinkLookup[action]
	if !ok {
		// Unknown action type — not in any sink category. Default-allow
		// so MCP tools and future built-ins work without policy updates.
		return DecisionAllow
	}
	row, ok := p.Rules[classification.Sensitivity]
	if !ok {
		return DecisionAllow
	}
	d, ok := row[category]
	if !ok {
		return DecisionAllow
	}
	return d
}

func matchSource(m SourceMatch, normalized, base string) bool {
	// Empty match = catch-all (the "default" rule).
	if len(m.BasenameIn) == 0 && len(m.BasenameNotIn) == 0 &&
		len(m.BasenameSuffixIn) == 0 && len(m.BasenameContains) == 0 &&
		len(m.PathContains) == 0 && len(m.PathIn) == 0 && m.BasenameRegex == "" {
		return true
	}

	matched := false

	// basename_in: exact basename match.
	if len(m.BasenameIn) > 0 {
		found := false
		for _, b := range m.BasenameIn {
			if base == strings.ToLower(b) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
		matched = true
	}

	// basename_not_in: exclusion list (overrides basename_in).
	if len(m.BasenameNotIn) > 0 {
		for _, b := range m.BasenameNotIn {
			if base == strings.ToLower(b) {
				return false
			}
		}
	}

	// basename_suffix_in: suffix match on the basename.
	if len(m.BasenameSuffixIn) > 0 {
		found := false
		for _, s := range m.BasenameSuffixIn {
			if strings.HasSuffix(base, strings.ToLower(s)) {
				found = true
				break
			}
		}
		if !found && !matched {
			return false
		}
		if found {
			matched = true
		}
	}

	// basename_contains: substring match on the basename.
	if len(m.BasenameContains) > 0 {
		found := false
		for _, s := range m.BasenameContains {
			if strings.Contains(base, strings.ToLower(s)) {
				found = true
				break
			}
		}
		if !found && !matched {
			return false
		}
		if found {
			matched = true
		}
	}

	// path_contains: substring match on the full normalized path.
	if len(m.PathContains) > 0 {
		found := false
		for _, s := range m.PathContains {
			if strings.Contains(normalized, strings.ToLower(s)) {
				found = true
				break
			}
		}
		if !found && !matched {
			return false
		}
		if found {
			matched = true
		}
	}

	// path_in: exact path match.
	if len(m.PathIn) > 0 {
		found := false
		for _, p := range m.PathIn {
			if normalized == strings.ToLower(p) {
				found = true
				break
			}
		}
		if !found && !matched {
			return false
		}
		if found {
			matched = true
		}
	}

	return matched
}

func parseSensitivity(s string) (SensitivityLevel, error) {
	switch strings.ToLower(s) {
	case "public":
		return SensitivityPublic, nil
	case "internal":
		return SensitivityInternal, nil
	case "confidential":
		return SensitivityConfidential, nil
	case "restricted":
		return SensitivityRestricted, nil
	case "critical":
		return SensitivityCritical, nil
	default:
		return 0, fmt.Errorf("unknown sensitivity level %q", s)
	}
}
