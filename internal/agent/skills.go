package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill represents a custom user-defined skill following the Agent Skills
// Specification. Skills live at skills/<name>/SKILL.md with YAML frontmatter
// (name, description) and a markdown body with instructions.
type Skill struct {
	// Name is the skill identifier (lowercase, hyphens, max 64 chars).
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

// NewSkillManager loads custom skills from the workspace skills directory.
// Each skill is a subdirectory containing a SKILL.md file.
func NewSkillManager(workspacePath string) *SkillManager {
	sm := &SkillManager{loaded: make(map[string]bool)}
	skillsDir := filepath.Join(workspacePath, "skills")
	sm.skills = loadCustomSkills(skillsDir)
	return sm
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
			return sm.skills[i].Body, true
		}
	}
	return "", false
}

// LoadedSkillBodies returns the full body content of all loaded skills.
func (sm *SkillManager) LoadedSkillBodies() string {
	var bodies []string
	for _, s := range sm.skills {
		if sm.loaded[s.Name] && s.Body != "" {
			bodies = append(bodies, fmt.Sprintf("# Skill: %s\n\n%s", s.Name, s.Body))
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

// loadCustomSkills reads skills from subdirectories of the skills dir.
// Each subdirectory must contain a SKILL.md file with YAML frontmatter.
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
		skill, parseErr := parseSkill(string(data))
		if parseErr != nil {
			continue
		}
		skill.Dir = filepath.Join(dir, e.Name())
		skills = append(skills, skill)
	}
	return skills
}

func parseSkill(content string) (Skill, error) {
	if !strings.HasPrefix(content, "---") {
		return Skill{}, fmt.Errorf("no frontmatter")
	}
	parts := strings.SplitN(content[3:], "---", 2)
	if len(parts) < 2 {
		return Skill{}, fmt.Errorf("malformed frontmatter")
	}

	var skill Skill
	if err := yaml.Unmarshal([]byte(parts[0]), &skill); err != nil {
		return Skill{}, err
	}
	if skill.Name == "" {
		return Skill{}, fmt.Errorf("skill name is required")
	}
	skill.Body = strings.TrimSpace(parts[1])
	return skill, nil
}
