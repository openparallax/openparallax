package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// skillNameRe enforces the Agent Skills Specification name format:
// lowercase alphanumeric + hyphens, 1-64 characters, must start with
// a letter or digit (not a hyphen). Matches the spec at
// https://agentskills.io/specification.
var skillNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// maxSkillDescriptionLen is the spec-defined maximum description length.
const maxSkillDescriptionLen = 1024

// Skill represents a custom user-defined skill following the Agent Skills
// Specification. Skills live at skills/<name>/SKILL.md with YAML frontmatter
// (name, description) and a markdown body with instructions.
type Skill struct {
	// Name is the skill identifier (lowercase, hyphens, 1-64 chars).
	Name string `yaml:"name"`
	// Description explains what the skill does and when to use it.
	// Used for discovery — the LLM sees this to decide whether to load the skill.
	Description string `yaml:"description"`
	// Body is the full markdown instruction content (loaded on demand).
	Body string `yaml:"-"`
	// Dir is the skill directory path (for loading resources).
	Dir string `yaml:"-"`
}

// SkillManager loads and manages custom user-defined skills.
// Built-in tool guidance comes from tool schemas, not skills.
type SkillManager struct {
	skills []Skill
	loaded map[string]bool
}

// NewSkillManager loads skills from the global shared directory and the
// workspace-local skills directory. Workspace skills override global skills
// with the same name. Skills listed in disabled are excluded from discovery.
func NewSkillManager(workspacePath string, disabled []string) *SkillManager {
	sm := &SkillManager{loaded: make(map[string]bool)}

	disabledSet := make(map[string]bool, len(disabled))
	for _, name := range disabled {
		disabledSet[name] = true
	}

	// Global skills at ~/.openparallax/skills/ (shared across all agents).
	globalSkills := loadGlobalSkills()
	byName := make(map[string]Skill, len(globalSkills))
	for _, s := range globalSkills {
		if !disabledSet[s.Name] {
			byName[s.Name] = s
		}
	}

	// Workspace skills override global on name collision.
	workspaceDir := filepath.Join(workspacePath, "skills")
	for _, s := range loadCustomSkills(workspaceDir) {
		if !disabledSet[s.Name] {
			byName[s.Name] = s
		}
	}

	for _, s := range byName {
		sm.skills = append(sm.skills, s)
	}
	return sm
}

// loadGlobalSkills loads skills from ~/.openparallax/skills/.
func loadGlobalSkills() []Skill {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return loadCustomSkills(filepath.Join(home, ".openparallax", "skills"))
}

// DiscoverySummary returns a compact index of available custom skills for
// the system prompt. Only name and description — the LLM uses this to
// decide which skills to load via load_skills.
func (sm *SkillManager) DiscoverySummary() string {
	if len(sm.skills) == 0 {
		return ""
	}

	var lines []string
	for _, s := range sm.skills {
		lines = append(lines, fmt.Sprintf("- %s: %s", s.Name, s.Description))
	}

	return fmt.Sprintf(`# Custom Skills

You have access to user-defined guidance for these domains:
%s

To get detailed instructions for a domain, call load_skills with the skill name.`, strings.Join(lines, "\n"))
}

// LoadSkill loads a skill's full body by name. Returns the body text.
// Called when the LLM requests a skill via load_skills.
func (sm *SkillManager) LoadSkill(name string) (string, bool) {
	for i := range sm.skills {
		if sm.skills[i].Name == name {
			sm.loaded[name] = true
			return stripMarkdown(sm.skills[i].Body), true
		}
	}
	return "", false
}

// LoadedSkillBodies returns the full body content of all loaded skills.
func (sm *SkillManager) LoadedSkillBodies() string {
	var bodies []string
	for _, s := range sm.skills {
		if sm.loaded[s.Name] && s.Body != "" {
			bodies = append(bodies, fmt.Sprintf("Skill: %s\n\n%s", s.Name, stripMarkdown(s.Body)))
		}
	}
	return strings.Join(bodies, "\n\n---\n\n")
}

// ResetSession clears the loaded skills for a new session.
func (sm *SkillManager) ResetSession() {
	sm.loaded = make(map[string]bool)
}

// Skills returns all discovered skills.
func (sm *SkillManager) Skills() []Skill {
	return sm.skills
}

// HasSkills returns true if any custom skills are available.
func (sm *SkillManager) HasSkills() bool {
	return len(sm.skills) > 0
}

// LoadSkillsFromDir reads skills from subdirectories of the given directory.
// Each subdirectory must contain a SKILL.md file with YAML frontmatter.
func LoadSkillsFromDir(dir string) []Skill {
	return loadCustomSkills(dir)
}

func loadCustomSkills(dir string) []Skill {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var skills []Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillPath := filepath.Join(dir, e.Name(), "SKILL.md")
		data, readErr := os.ReadFile(skillPath)
		if readErr != nil {
			continue
		}
		skill, parseErr := parseSkill(string(data), e.Name())
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping skill %q: %s\n", e.Name(), parseErr)
			continue
		}
		skill.Dir = filepath.Join(dir, e.Name())
		skills = append(skills, skill)
	}
	return skills
}

// parseSkill parses a SKILL.md file and validates it against the Agent
// Skills Specification. The dirName parameter is the parent directory
// name, used to verify the name field matches the folder.
func parseSkill(content, dirName string) (Skill, error) {
	if !strings.HasPrefix(content, "---") {
		return Skill{}, fmt.Errorf("missing YAML frontmatter (file must start with ---)")
	}
	parts := strings.SplitN(content[3:], "---", 2)
	if len(parts) < 2 {
		return Skill{}, fmt.Errorf("malformed frontmatter (missing closing ---)")
	}

	var skill Skill
	if err := yaml.Unmarshal([]byte(parts[0]), &skill); err != nil {
		return Skill{}, fmt.Errorf("invalid YAML frontmatter: %w", err)
	}

	// Name: required, must match spec format.
	if skill.Name == "" {
		return Skill{}, fmt.Errorf("'name' field is required in frontmatter")
	}
	if !skillNameRe.MatchString(skill.Name) {
		return Skill{}, fmt.Errorf("'name' must be 1-64 lowercase alphanumeric + hyphens (got %q)", skill.Name)
	}
	if skill.Name != dirName {
		return Skill{}, fmt.Errorf("'name' (%q) must match the directory name (%q)", skill.Name, dirName)
	}

	// Description: required for discovery. Without it the LLM cannot
	// decide when to load the skill.
	if skill.Description == "" {
		return Skill{}, fmt.Errorf("'description' field is required — the LLM uses it to decide when to load the skill")
	}
	if len(skill.Description) > maxSkillDescriptionLen {
		return Skill{}, fmt.Errorf("'description' exceeds %d characters (%d)", maxSkillDescriptionLen, len(skill.Description))
	}

	skill.Body = strings.TrimSpace(parts[1])
	return skill, nil
}
