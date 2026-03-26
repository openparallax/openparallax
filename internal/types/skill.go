package types

// SkillDefinition is parsed from a SKILL.md file's YAML frontmatter.
type SkillDefinition struct {
	// Name is the skill identifier.
	Name string `yaml:"name" json:"name"`

	// Description is a brief summary of what the skill does.
	Description string `yaml:"description" json:"description"`

	// WhenToUse describes the conditions under which this skill is relevant.
	WhenToUse string `yaml:"when_to_use" json:"when_to_use"`

	// Actions lists the action types this skill provides.
	Actions []ActionType `yaml:"actions" json:"actions"`

	// Keywords are terms that trigger this skill in matching.
	Keywords []string `yaml:"keywords" json:"keywords"`

	// Emoji is the display icon.
	Emoji string `yaml:"emoji" json:"emoji"`

	// Body is the full markdown content of the SKILL.md (below frontmatter).
	Body string `json:"-"`

	// SourcePath is where this skill was loaded from.
	SourcePath string `json:"source_path"`
}
