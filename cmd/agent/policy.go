package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openparallax/openparallax/internal/config"
	"github.com/spf13/cobra"
)

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage Shield policy files",
	Long: `Manage the Shield policy files that govern how OpenParallax evaluates
proposed tool calls. Policies live in the workspace's security/shield/ directory and
control which actions are denied at Tier 0, escalated through the Shield
pipeline, or allowed without further evaluation.`,
}

var policyListCmd = &cobra.Command{
	Use:          "list [name]",
	Short:        "List available policy files in the workspace",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runPolicyList,
}

var policyShowCmd = &cobra.Command{
	Use:          "show [name]",
	Short:        "Print the active policy file contents",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runPolicyShow,
}

var policyEditCmd = &cobra.Command{
	Use:          "edit [name]",
	Short:        "Open the active policy file in your default text editor",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runPolicyEdit,
}

var policySetCmd = &cobra.Command{
	Use:          "set <policy> [name]",
	Short:        "Switch the workspace to a different policy file",
	Long:         `Set the active policy file. The policy must exist in the workspace's security/shield/ directory. Restart the engine for the change to take effect.`,
	Args:         cobra.RangeArgs(1, 2),
	SilenceUsage: true,
	RunE:         runPolicySet,
}

func init() {
	policyCmd.AddCommand(policyListCmd)
	policyCmd.AddCommand(policyShowCmd)
	policyCmd.AddCommand(policyEditCmd)
	policyCmd.AddCommand(policySetCmd)
	rootCmd.AddCommand(policyCmd)
}

// resolveWorkspaceFromConfig returns the workspace directory by loading the
// config file and reading its Workspace field, falling back to the directory
// containing the config file if Workspace is unset.
func resolveWorkspaceFromConfig(cfgPath string) (string, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	if cfg.Workspace != "" {
		return cfg.Workspace, nil
	}
	return filepath.Dir(cfgPath), nil
}

// listPoliciesInDir scans the Shield policy directory and returns the names
// of every .yaml file (without the extension), sorted alphabetically.
func listPoliciesInDir(policiesDir string) ([]string, error) {
	entries, err := os.ReadDir(policiesDir)
	if err != nil {
		return nil, fmt.Errorf("read shield policy directory: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		names = append(names, strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml"))
	}
	sort.Strings(names)
	return names, nil
}

func runPolicyList(_ *cobra.Command, args []string) error {
	cfgPath, err := resolveConfig(args, "")
	if err != nil {
		return err
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	workspace, err := resolveWorkspaceFromConfig(cfgPath)
	if err != nil {
		return err
	}
	policiesDir := filepath.Join(workspace, "security", "shield")

	names, err := listPoliciesInDir(policiesDir)
	if err != nil {
		return err
	}

	if len(names) == 0 {
		fmt.Printf("No policy files found in %s\n", policiesDir)
		return nil
	}

	activeName := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(cfg.Shield.PolicyFile), ".yaml"), ".yml")

	fmt.Printf("Policies in %s\n\n", policiesDir)
	for _, name := range names {
		marker := "  "
		if name == activeName {
			marker = "* "
		}
		fmt.Printf("%s%s\n", marker, name)
	}
	fmt.Printf("\n* = active policy\n")
	return nil
}

func runPolicyShow(_ *cobra.Command, args []string) error {
	cfgPath, err := resolveConfig(args, "")
	if err != nil {
		return err
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	workspace, err := resolveWorkspaceFromConfig(cfgPath)
	if err != nil {
		return err
	}

	policyPath := cfg.Shield.PolicyFile
	if !filepath.IsAbs(policyPath) {
		policyPath = filepath.Join(workspace, policyPath)
	}

	data, err := os.ReadFile(policyPath)
	if err != nil {
		return fmt.Errorf("read policy file: %w", err)
	}

	fmt.Printf("# %s\n\n", policyPath)
	fmt.Println(string(data))
	return nil
}

func runPolicyEdit(_ *cobra.Command, args []string) error {
	cfgPath, err := resolveConfig(args, "")
	if err != nil {
		return err
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	workspace, err := resolveWorkspaceFromConfig(cfgPath)
	if err != nil {
		return err
	}

	policyPath := cfg.Shield.PolicyFile
	if !filepath.IsAbs(policyPath) {
		policyPath = filepath.Join(workspace, policyPath)
	}

	if _, statErr := os.Stat(policyPath); statErr != nil {
		return fmt.Errorf("policy file not found: %s", policyPath)
	}

	editor := resolveEditor()
	fmt.Printf("Opening %s in %s...\n", policyPath, editor)

	cmd := exec.Command(editor, policyPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runPolicySet(_ *cobra.Command, args []string) error {
	policyName := args[0]
	var nameArgs []string
	if len(args) > 1 {
		nameArgs = args[1:]
	}

	cfgPath, err := resolveConfig(nameArgs, "")
	if err != nil {
		return err
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	workspace, err := resolveWorkspaceFromConfig(cfgPath)
	if err != nil {
		return err
	}
	policiesDir := filepath.Join(workspace, "security", "shield")

	// Resolve the policy name to a file relative to the workspace, accepting
	// either bare names ("strict"), names with extension ("strict.yaml"), or
	// full paths within the policies directory.
	candidate := strings.TrimSuffix(strings.TrimSuffix(policyName, ".yaml"), ".yml")
	policyFile := filepath.Join(policiesDir, candidate+".yaml")
	if _, statErr := os.Stat(policyFile); statErr != nil {
		// Try .yml extension as a fallback.
		altFile := filepath.Join(policiesDir, candidate+".yml")
		if _, altErr := os.Stat(altFile); altErr != nil {
			available, _ := listPoliciesInDir(policiesDir)
			if len(available) == 0 {
				return fmt.Errorf("policy %q not found in %s", policyName, policiesDir)
			}
			return fmt.Errorf("policy %q not found. Available: %s", policyName, strings.Join(available, ", "))
		}
		policyFile = altFile
	}

	// Store the path relative to the workspace for portability.
	relPath, relErr := filepath.Rel(workspace, policyFile)
	if relErr != nil {
		relPath = policyFile
	}

	cfg.Shield.PolicyFile = relPath

	if saveErr := config.Save(cfgPath, cfg); saveErr != nil {
		return fmt.Errorf("save config: %w", saveErr)
	}

	fmt.Printf("Policy set to %s\n", relPath)
	fmt.Printf("Restart the engine for the change to take effect: openparallax restart\n")
	return nil
}
