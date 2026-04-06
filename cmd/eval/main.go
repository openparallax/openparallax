// Package main implements the OpenParallax adversarial eval harness.
// It tests the Shield security pipeline against attack cases using three
// configurations: baseline (A), guardrails-only (B), and full Parallax (C).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func main() {
	root := &cobra.Command{
		Use:   "openparallax-eval",
		Short: "Adversarial eval harness for the OpenParallax Shield pipeline",
		Long: `Tests the Shield security pipeline against adversarial attack cases.

Three configurations:
  A (baseline)   — Shield disabled, no safety prompt
  B (guardrails) — Shield disabled, comprehensive safety prompt
  C (parallax)   — Shield enabled, normal operation`,
		RunE: runEval,
	}

	root.Flags().String("suite", "", "path to YAML test suite (required)")
	root.Flags().String("config", "", "eval configuration: A, B, or C (required)")
	root.Flags().String("output", "results.json", "output path for structured results")
	root.Flags().String("workspace", "", "path to an initialized workspace with config.yaml")
	root.Flags().Int("concurrency", 1, "parallel test cases (reserved for future use)")

	if err := root.MarkFlagRequired("suite"); err != nil {
		fmt.Fprintf(os.Stderr, "flag setup error: %v\n", err)
		os.Exit(1)
	}
	if err := root.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "flag setup error: %v\n", err)
		os.Exit(1)
	}

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runEval(cmd *cobra.Command, _ []string) error {
	suitePath, _ := cmd.Flags().GetString("suite")
	configName, _ := cmd.Flags().GetString("config")
	outputPath, _ := cmd.Flags().GetString("output")
	workspacePath, _ := cmd.Flags().GetString("workspace")

	if configName != "A" && configName != "B" && configName != "C" {
		return fmt.Errorf("--config must be A, B, or C (got %q)", configName)
	}

	// Resolve workspace and config.yaml.
	if workspacePath == "" {
		workspacePath = "."
	}
	workspacePath, err := filepath.Abs(workspacePath)
	if err != nil {
		return fmt.Errorf("resolve workspace path: %w", err)
	}
	configPath := filepath.Join(workspacePath, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config.yaml not found in workspace %s", workspacePath)
	}

	// Parse test suite.
	cases, err := loadSuite(suitePath)
	if err != nil {
		return fmt.Errorf("load suite: %w", err)
	}
	fmt.Printf("Loaded %d test cases from %s\n", len(cases), suitePath)

	// Create the harness engine.
	engine, err := createEngine(configName, workspacePath, configPath)
	if err != nil {
		return fmt.Errorf("create engine (config %s): %w", configName, err)
	}
	fmt.Printf("Engine ready (config %s)\n\n", configName)

	// Run the suite.
	results := RunSuite(engine, cases, configName)

	// Compute summary and write output.
	summary := ComputeSummary(results)
	sr := &SuiteResults{
		Config:    configName,
		Timestamp: time.Now(),
		CaseCount: len(cases),
		Results:   results,
		Summary:   summary,
	}

	if err := WriteSuiteResults(outputPath, sr); err != nil {
		return fmt.Errorf("write results: %w", err)
	}

	printSummary(sr)
	fmt.Printf("\nResults written to %s\n", outputPath)
	return nil
}

func loadSuite(path string) ([]TestCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var suite struct {
		Cases []TestCase `yaml:"cases"`
	}
	if err := yaml.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	if len(suite.Cases) == 0 {
		return nil, fmt.Errorf("no test cases found in %s", path)
	}

	return suite.Cases, nil
}

func createEngine(configName, workspacePath, configPath string) (*HarnessEngine, error) {
	switch configName {
	case "A":
		return NewBaselineEngine(workspacePath, configPath)
	case "B":
		return NewGuardrailEngine(workspacePath, configPath)
	case "C":
		return NewParallaxEngine(workspacePath, configPath)
	default:
		return nil, fmt.Errorf("unknown config: %s", configName)
	}
}

func printSummary(sr *SuiteResults) {
	s := sr.Summary
	fmt.Println("\n========== EVAL SUMMARY ==========")
	fmt.Printf("Config:           %s\n", sr.Config)
	fmt.Printf("Cases:            %d\n", sr.CaseCount)
	fmt.Printf("Pass Rate:        %.1f%%\n", s.PassRate*100)
	fmt.Printf("Overall ASR:      %.1f%%\n", s.OverallASR*100)
	fmt.Printf("False Positive:   %.1f%%\n", s.FalsePositiveRate*100)

	if len(s.ASRByCategory) > 0 {
		fmt.Println("\nASR by Category:")
		for cat, asr := range s.ASRByCategory {
			fmt.Printf("  %-20s %.1f%%\n", cat, asr*100)
		}
	}

	if len(s.TierDistribution) > 0 {
		fmt.Println("\nTier Distribution:")
		for tier, count := range s.TierDistribution {
			fmt.Printf("  Tier %d: %d\n", tier, count)
		}
	}

	if s.AvgShieldLatencyMs > 0 {
		fmt.Printf("\nShield Latency (ms):\n")
		fmt.Printf("  Avg: %.1f  P50: %.1f  P95: %.1f  P99: %.1f\n",
			s.AvgShieldLatencyMs, s.P50ShieldMs, s.P95ShieldMs, s.P99ShieldMs)
	}

	fmt.Println("==================================")
}
