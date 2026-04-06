package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openparallax/openparallax/internal/agent"
	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/engine/executors"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/llm"
	"github.com/openparallax/openparallax/shield"
)

// HarnessEngine is a headless engine that processes messages for eval.
// It wires together an LLM provider, optional Shield pipeline, and a
// recording executor. It does NOT use internal/engine.Engine.
type HarnessEngine struct {
	provider        llm.Provider
	shield          *shield.Pipeline
	recorder        *RecordingExecutor
	workspace       string
	configPath      string
	guardrailPrompt string
	modelOverride   string
	tier3AutoDecide func(toolName string, simulatedHuman string) bool
	schemas         []executors.ToolSchema
}

// NewBaselineEngine creates an engine with Shield disabled and no safety prompt.
// Config A measures raw LLM behavior with no protection.
func NewBaselineEngine(workspacePath, configPath, modelOverride, baseURLOverride, apiKeyEnvOverride string) (*HarnessEngine, error) {
	provider, schemas, err := buildCommon(workspacePath, configPath, modelOverride, baseURLOverride, apiKeyEnvOverride)
	if err != nil {
		return nil, err
	}
	return &HarnessEngine{
		provider:        provider,
		recorder:        NewRecordingExecutor(nil),
		workspace:       workspacePath,
		tier3AutoDecide: defaultTier3,
		schemas:         schemas,
	}, nil
}

// NewGuardrailEngine creates an engine with Shield disabled but a comprehensive
// safety system prompt injected. Config B measures prompt-level defense.
func NewGuardrailEngine(workspacePath, configPath, modelOverride, baseURLOverride, apiKeyEnvOverride string) (*HarnessEngine, error) {
	provider, schemas, err := buildCommon(workspacePath, configPath, modelOverride, baseURLOverride, apiKeyEnvOverride)
	if err != nil {
		return nil, err
	}

	guardrailPath := filepath.Join(executableDir(), "prompts", "guardrails.md")
	prompt, readErr := os.ReadFile(guardrailPath)
	if readErr != nil {
		// Fall back to looking relative to the working directory.
		prompt, readErr = os.ReadFile("cmd/eval/prompts/guardrails.md")
		if readErr != nil {
			return nil, fmt.Errorf("guardrail prompt not found: %w", readErr)
		}
	}

	return &HarnessEngine{
		provider:        provider,
		recorder:        NewRecordingExecutor(nil),
		workspace:       workspacePath,
		guardrailPrompt: string(prompt),
		tier3AutoDecide: defaultTier3,
		schemas:         schemas,
	}, nil
}

// NewParallaxEngine creates an engine with Shield enabled and normal operation.
// Config C measures the full Parallax defense.
func NewParallaxEngine(workspacePath, configPath, modelOverride, baseURLOverride, apiKeyEnvOverride string) (*HarnessEngine, error) {
	provider, schemas, err := buildCommon(workspacePath, configPath, modelOverride, baseURLOverride, apiKeyEnvOverride)
	if err != nil {
		return nil, err
	}

	cfg, loadErr := config.Load(configPath)
	if loadErr != nil {
		return nil, fmt.Errorf("load config for shield: %w", loadErr)
	}

	policyFile := cfg.Shield.PolicyFile
	if policyFile == "" {
		policyFile = filepath.Join(workspacePath, "policies", "default.yaml")
	}
	if !filepath.IsAbs(policyFile) {
		policyFile = filepath.Join(workspacePath, policyFile)
	}

	pipeline, pipeErr := shield.NewPipeline(shield.Config{
		PolicyFile:       policyFile,
		OnnxThreshold:    cfg.Shield.OnnxThreshold,
		HeuristicEnabled: cfg.Shield.HeuristicEnabled,
		ClassifierAddr:   cfg.Shield.ClassifierAddr,
		Evaluator: &shield.EvaluatorConfig{
			Provider:  cfg.Shield.Evaluator.Provider,
			Model:     cfg.Shield.Evaluator.Model,
			APIKeyEnv: cfg.Shield.Evaluator.APIKeyEnv,
			BaseURL:   cfg.Shield.Evaluator.BaseURL,
		},
		FailClosed:  cfg.General.FailClosed,
		RateLimit:   cfg.General.RateLimit,
		VerdictTTL:  cfg.General.VerdictTTLSeconds,
		DailyBudget: cfg.General.DailyBudget,
	})
	if pipeErr != nil {
		return nil, fmt.Errorf("shield pipeline init: %w", pipeErr)
	}

	return &HarnessEngine{
		provider:        provider,
		shield:          pipeline,
		recorder:        NewRecordingExecutor(nil),
		workspace:       workspacePath,
		tier3AutoDecide: defaultTier3,
		schemas:         schemas,
	}, nil
}

// buildShieldPipeline creates a Shield pipeline from the workspace config.
// Used by inject mode (no LLM needed) and by NewParallaxEngine.
func buildShieldPipeline(workspacePath, configPath string) (*shield.Pipeline, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	policyFile := cfg.Shield.PolicyFile
	if policyFile == "" {
		policyFile = filepath.Join(workspacePath, "policies", "default.yaml")
	}
	if !filepath.IsAbs(policyFile) {
		policyFile = filepath.Join(workspacePath, policyFile)
	}

	// Read the workspace canary token for Tier 2 evaluator.
	canaryToken := ""
	canaryPath := filepath.Join(workspacePath, ".openparallax", "canary.token")
	if data, readErr := os.ReadFile(canaryPath); readErr == nil {
		canaryToken = strings.TrimSpace(string(data))
	}

	promptPath := filepath.Join(workspacePath, "prompts", "evaluator-v1.md")
	if _, statErr := os.Stat(promptPath); statErr != nil {
		// Fall back to the templates location.
		promptPath = filepath.Join(workspacePath, ".openparallax", "evaluator-v1.md")
	}

	return shield.NewPipeline(shield.Config{
		PolicyFile:       policyFile,
		OnnxThreshold:    cfg.Shield.OnnxThreshold,
		HeuristicEnabled: cfg.Shield.HeuristicEnabled,
		ClassifierAddr:   cfg.Shield.ClassifierAddr,
		Evaluator: &shield.EvaluatorConfig{
			Provider:  cfg.Shield.Evaluator.Provider,
			Model:     cfg.Shield.Evaluator.Model,
			APIKeyEnv: cfg.Shield.Evaluator.APIKeyEnv,
			BaseURL:   cfg.Shield.Evaluator.BaseURL,
		},
		CanaryToken: canaryToken,
		PromptPath:  promptPath,
		FailClosed:  cfg.General.FailClosed,
		RateLimit:   10000, // No rate limiting during eval.
		VerdictTTL:  cfg.General.VerdictTTLSeconds,
		DailyBudget: 10000, // No budget limit during eval.
	})
}

// buildCommon loads the workspace config, creates the LLM provider, and
// extracts tool schemas from a temporary real executor registry.
func buildCommon(workspacePath, configPath, modelOverride, baseURLOverride, apiKeyEnvOverride string) (llm.Provider, []executors.ToolSchema, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	llmCfg := cfg.LLM
	if modelOverride != "" {
		llmCfg.Model = modelOverride
	}
	if baseURLOverride != "" {
		llmCfg.BaseURL = baseURLOverride
	}
	if apiKeyEnvOverride != "" {
		llmCfg.APIKeyEnv = apiKeyEnvOverride
	}

	provider, err := llm.NewProvider(llmCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("create llm provider: %w", err)
	}

	// Build a real registry to extract tool schemas, then discard it.
	registry := executors.NewRegistry(workspacePath, cfg, nil, nil)
	schemas := registry.AllToolSchemas()

	return provider, schemas, nil
}

// buildSystemPrompt constructs the system prompt for a test case.
func (h *HarnessEngine) buildSystemPrompt(userMessage string) string {
	assembler := agent.NewContextAssembler(h.workspace, nil)
	prompt, err := assembler.Assemble(types.SessionNormal, userMessage)
	if err != nil {
		prompt = "You are a helpful assistant."
	}

	if h.guardrailPrompt != "" {
		prompt = h.guardrailPrompt + "\n\n---\n\n" + prompt
	}

	return prompt
}

// toolDefinitions converts the stored schemas to LLM tool definitions.
func (h *HarnessEngine) toolDefinitions() []llm.ToolDefinition {
	return agent.GenerateToolDefinitions(h.schemas)
}

// defaultTier3 simulates human approval based on the test case's simulated_human field.
func defaultTier3(_ string, simulatedHuman string) bool {
	return simulatedHuman == "approve"
}

// executableDir returns the directory of the running executable.
func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}
