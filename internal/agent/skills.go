package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill represents a loaded skill with frontmatter metadata and body content.
type Skill struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	WhenToUse   string   `yaml:"when_to_use"`
	Actions     []string `yaml:"actions"`
	Keywords    []string `yaml:"keywords"`
	Emoji       string   `yaml:"emoji"`
	Body        string   `yaml:"-"`
}

// MatchesTool returns true if the skill's actions list includes the given tool name.
func (s *Skill) MatchesTool(toolName string) bool {
	for _, a := range s.Actions {
		if a == toolName {
			return true
		}
	}
	return false
}

// SkillManager loads and manages skills for system prompt injection.
type SkillManager struct {
	skills []Skill
	active map[string]bool
}

// NewSkillManager loads skills from workspace and bundled directories.
func NewSkillManager(workspacePath string) *SkillManager {
	sm := &SkillManager{active: make(map[string]bool)}

	// Load bundled skills first (lower priority).
	bundled := loadSkillsFromDir("skills")
	// Load workspace skills (higher priority, overrides bundled).
	workspace := loadSkillsFromDir(filepath.Join(workspacePath, "skills"))

	// Merge: workspace overrides bundled by name.
	seen := make(map[string]bool)
	for _, s := range workspace {
		sm.skills = append(sm.skills, s)
		seen[s.Name] = true
	}
	for _, s := range bundled {
		if !seen[s.Name] {
			sm.skills = append(sm.skills, s)
		}
	}

	return sm
}

// LightSummary returns a compact skills overview for the system prompt.
func (sm *SkillManager) LightSummary() string {
	if len(sm.skills) == 0 {
		return ""
	}

	var lines []string
	for _, s := range sm.skills {
		prefix := ""
		if s.Emoji != "" {
			prefix = s.Emoji + " "
		}
		lines = append(lines, fmt.Sprintf("- %s**%s**: %s", prefix, s.Name, s.Description))
	}

	return fmt.Sprintf(`# Available Skills

You have specialized knowledge for these domains:
%s
When a conversation enters one of these domains, you'll receive detailed guidance.`, strings.Join(lines, "\n"))
}

// MatchSkills returns skills relevant to the given tool names that aren't already active.
func (sm *SkillManager) MatchSkills(toolNames []string) []Skill {
	var matched []Skill
	for i := range sm.skills {
		if sm.active[sm.skills[i].Name] {
			continue
		}
		for _, toolName := range toolNames {
			if sm.skills[i].MatchesTool(toolName) {
				sm.active[sm.skills[i].Name] = true
				matched = append(matched, sm.skills[i])
				break
			}
		}
	}
	return matched
}

// ActiveSkillBodies returns the full body content of all activated skills.
func (sm *SkillManager) ActiveSkillBodies() string {
	var bodies []string
	for _, s := range sm.skills {
		if sm.active[s.Name] && s.Body != "" {
			bodies = append(bodies, fmt.Sprintf("# Skill: %s\n\n%s", s.Name, s.Body))
		}
	}
	return strings.Join(bodies, "\n\n---\n\n")
}

// ResetSession clears the active skills for a new session.
func (sm *SkillManager) ResetSession() {
	sm.active = make(map[string]bool)
}

// Skills returns all loaded skills.
func (sm *SkillManager) Skills() []Skill {
	return sm.skills
}

func loadSkillsFromDir(dir string) []Skill {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var skills []Skill
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		skill, err := parseSkill(string(data))
		if err != nil {
			continue
		}
		skills = append(skills, skill)
	}
	return skills
}

func parseSkill(content string) (Skill, error) {
	// Split YAML frontmatter from body.
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
	skill.Body = strings.TrimSpace(parts[1])
	return skill, nil
}
