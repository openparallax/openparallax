package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/openparallax/openparallax/internal/agent"
	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:          "skill",
	Short:        "Manage skills (install, remove, list)",
	SilenceUsage: true,
}

var skillListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List installed skills",
	SilenceUsage: true,
	RunE:         runSkillList,
}

var skillInstallCmd = &cobra.Command{
	Use:   "install <name-or-url>",
	Short: "Install a skill from the official repo or a Git URL",
	Long: `Install a skill into ~/.openparallax/skills/.

Examples:
  openparallax skill install developer
  openparallax skill install https://github.com/user/my-skill.git`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runSkillInstall,
}

var skillRemoveCmd = &cobra.Command{
	Use:          "remove <name>",
	Short:        "Remove an installed skill",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runSkillRemove,
}

func init() {
	skillCmd.AddCommand(skillListCmd)
	skillCmd.AddCommand(skillInstallCmd)
	skillCmd.AddCommand(skillRemoveCmd)
	rootCmd.AddCommand(skillCmd)
}

func globalSkillsDir() (string, error) {
	opHome, err := openparallaxHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(opHome, "skills"), nil
}

func runSkillList(_ *cobra.Command, _ []string) error {
	dir, err := globalSkillsDir()
	if err != nil {
		return err
	}

	globalSkills := agent.LoadSkillsFromDir(dir)

	// Also scan workspace skills if we can find the config.
	var workspaceSkills []agent.Skill
	if cfgPath, resolveErr := resolveConfig(nil, ""); resolveErr == nil {
		if workspace := extractWorkspacePath(cfgPath); workspace != "" {
			workspaceSkills = agent.LoadSkillsFromDir(filepath.Join(workspace, "skills"))
		}
	}

	if len(globalSkills) == 0 && len(workspaceSkills) == 0 {
		fmt.Println("No skills installed.")
		fmt.Printf("\nGlobal skills directory: %s\n", dir)
		fmt.Println("Install with: openparallax skill install <name>")
		return nil
	}

	if len(globalSkills) > 0 {
		fmt.Printf("Global skills (%s):\n", dir)
		for _, s := range globalSkills {
			fmt.Printf("  %-20s %s\n", s.Name, s.Description)
		}
	}

	if len(workspaceSkills) > 0 {
		fmt.Println("\nWorkspace skills:")
		for _, s := range workspaceSkills {
			fmt.Printf("  %-20s %s\n", s.Name, s.Description)
		}
	}

	return nil
}

// extractWorkspacePath reads just the workspace path from config.yaml.
func extractWorkspacePath(cfgPath string) string {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "workspace:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "workspace:"))
		}
	}
	return ""
}

func runSkillInstall(_ *cobra.Command, args []string) error {
	source := args[0]

	dir, err := globalSkillsDir()
	if err != nil {
		return err
	}
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return fmt.Errorf("create skills directory: %w", mkErr)
	}

	// Determine if this is a Git URL or a skill name from the official repo.
	if isGitURL(source) {
		return installSkillFromGit(dir, source)
	}
	return installSkillFromOfficial(dir, source)
}

func isGitURL(s string) bool {
	return strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "git@") ||
		strings.HasSuffix(s, ".git")
}

func installSkillFromGit(dir, url string) error {
	// Derive skill name from URL (last path segment, strip .git).
	parts := strings.Split(strings.TrimSuffix(url, ".git"), "/")
	name := parts[len(parts)-1]
	destPath := filepath.Join(dir, name)

	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("skill %q already installed at %s", name, destPath)
	}

	fmt.Printf("  Cloning %s...\n", url)
	if err := gitCloneShallow(url, destPath); err != nil {
		return fmt.Errorf("clone failed: %w", err)
	}

	// Verify SKILL.md exists.
	if _, err := os.Stat(filepath.Join(destPath, "SKILL.md")); err != nil {
		_ = os.RemoveAll(destPath)
		return fmt.Errorf("cloned repo does not contain SKILL.md — not a valid skill")
	}

	fmt.Printf("  ✓ Installed skill %q\n", name)
	return nil
}

func installSkillFromOfficial(dir, name string) error {
	destPath := filepath.Join(dir, name)
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("skill %q already installed", name)
	}

	// Download from the official openparallax/skills repo.
	// Try fetching SKILL.md directly via GitHub raw content.
	url := fmt.Sprintf("https://raw.githubusercontent.com/openparallax/skills/main/%s/SKILL.md", name)

	if mkErr := os.MkdirAll(destPath, 0o755); mkErr != nil {
		return fmt.Errorf("create skill directory: %w", mkErr)
	}

	skillPath := filepath.Join(destPath, "SKILL.md")
	fmt.Printf("  Downloading skill %q...\n", name)
	if dlErr := fetchFile(url, skillPath, 30*time.Second, true); dlErr != nil {
		_ = os.RemoveAll(destPath)
		return fmt.Errorf("skill %q not found in official repository", name)
	}

	fmt.Printf("  ✓ Installed skill %q\n", name)
	return nil
}

func gitCloneShallow(url, dest string) error {
	cmd := exec.Command("git", "clone", "--depth", "1", url, dest)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runSkillRemove(_ *cobra.Command, args []string) error {
	name := args[0]

	dir, err := globalSkillsDir()
	if err != nil {
		return err
	}

	skillPath := filepath.Join(dir, name)
	if _, statErr := os.Stat(skillPath); statErr != nil {
		return fmt.Errorf("skill %q is not installed", name)
	}

	if rmErr := os.RemoveAll(skillPath); rmErr != nil {
		return fmt.Errorf("remove skill: %w", rmErr)
	}

	fmt.Printf("  ✓ Removed skill %q\n", name)
	return nil
}
