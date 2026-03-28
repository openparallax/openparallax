package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/mattn/go-isatty"
	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/templates"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:          "init",
	Short:        "Initialize a new OpenParallax workspace",
	Long:         "Creates a workspace directory with memory files, configuration, and database.",
	SilenceUsage: true,
	RunE:         runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// defaultModels maps LLM providers to their default model names.
var defaultModels = map[string]string{
	"anthropic": "claude-sonnet-4-20250514",
	"openai":    "gpt-4o",
	"google":    "gemini-2.0-flash",
	"ollama":    "llama3.2",
}

// defaultAPIKeyEnvs maps LLM providers to their conventional API key env var names.
var defaultAPIKeyEnvs = map[string]string{
	"anthropic": "ANTHROPIC_API_KEY",
	"openai":    "OPENAI_API_KEY",
	"google":    "GOOGLE_API_KEY",
	"ollama":    "",
}

func runInit(cmd *cobra.Command, args []string) error {
	if !isatty.IsTerminal(os.Stdin.Fd()) && !isatty.IsCygwinTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("interactive terminal required: openparallax init must be run in a terminal, not piped")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	defaultWorkspace := filepath.Join(home, ".openparallax", "workspace")

	var (
		workspacePath      string
		llmProvider        string
		llmModel           string
		llmAPIKeyEnv       string
		llmBaseURL         string
		shieldProvider     string
		shieldModel        string
		shieldAPIKeyEnv    string
		shieldBaseURL      string
		embeddingProvider  string
		embeddingModel     string
		embeddingAPIKeyEnv string
		embeddingBaseURL   string
	)

	// Workspace path
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Workspace path").
				Description("Where should OpenParallax store its workspace files?").
				Value(&workspacePath).
				Placeholder(defaultWorkspace),
		),
	).Run()
	if err != nil {
		return err
	}
	if workspacePath == "" {
		workspacePath = defaultWorkspace
	}
	workspacePath = expandTilde(workspacePath, home)

	// Check if workspace already exists
	if _, statErr := os.Stat(filepath.Join(workspacePath, "SOUL.md")); statErr == nil {
		fmt.Fprintf(os.Stderr, "Warning: Workspace already exists at %s. Existing files will not be overwritten.\n", workspacePath)
	}

	// LLM provider
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("LLM Provider").
				Description("Which LLM provider should the agent use?").
				Options(
					huh.NewOption("Anthropic (Claude)", "anthropic"),
					huh.NewOption("OpenAI (GPT)", "openai"),
					huh.NewOption("Google (Gemini)", "google"),
					huh.NewOption("Ollama (Local)", "ollama"),
				).
				Value(&llmProvider),
		),
	).Run()
	if err != nil {
		return err
	}

	// Model name
	llmModel = defaultModels[llmProvider]
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Model name").
				Description("Which model should the agent use?").
				Value(&llmModel).
				Placeholder(llmModel),
		),
	).Run()
	if err != nil {
		return err
	}

	// API key env var
	if llmProvider != "ollama" {
		llmAPIKeyEnv = defaultAPIKeyEnvs[llmProvider]
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("API key environment variable").
					Description("Name of the environment variable containing the API key.").
					Value(&llmAPIKeyEnv).
					Placeholder(llmAPIKeyEnv),
			),
		).Run()
		if err != nil {
			return err
		}
	}

	// Custom base URL for OpenAI-compatible endpoints.
	if llmProvider == "openai" {
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Custom base URL").
					Description("Leave empty for default OpenAI. Set for LM Studio, DeepSeek, Mistral, or other compatible endpoints.").
					Value(&llmBaseURL).
					Placeholder("https://api.openai.com/v1"),
			),
		).Run()
		if err != nil {
			return err
		}
	}

	// Shield evaluator — default to a different provider than the agent for cross-model security.
	shieldProvider = defaultShieldProvider(llmProvider)
	shieldOptions := []huh.Option[string]{
		huh.NewOption("Anthropic (Claude)", "anthropic"),
		huh.NewOption("OpenAI (GPT)", "openai"),
		huh.NewOption("Google (Gemini)", "google"),
		huh.NewOption("Ollama (Local)", "ollama"),
		huh.NewOption("Skip (heuristic-only, not recommended)", ""),
	}
	for i, opt := range shieldOptions {
		if opt.Value == shieldProvider {
			shieldOptions[i] = opt.Selected(true)
		}
	}
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Shield Evaluator Provider").
				Description("The Shield evaluator should use a DIFFERENT model for stronger security.").
				Options(shieldOptions...).
				Value(&shieldProvider),
		),
	).Run()
	if err != nil {
		return err
	}

	if shieldProvider != "" {
		shieldModel = defaultModels[shieldProvider]
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Shield evaluator model").
					Value(&shieldModel).
					Placeholder(shieldModel),
			),
		).Run()
		if err != nil {
			return err
		}

		if shieldProvider != "ollama" {
			shieldAPIKeyEnv = defaultAPIKeyEnvs[shieldProvider]
			err = huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Shield evaluator API key environment variable").
						Value(&shieldAPIKeyEnv).
						Placeholder(shieldAPIKeyEnv),
				),
			).Run()
			if err != nil {
				return err
			}
		}

		if shieldProvider == "openai" {
			err = huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Shield evaluator custom base URL").
						Description("Leave empty for default OpenAI. Set for compatible endpoints.").
						Value(&shieldBaseURL).
						Placeholder("https://api.openai.com/v1"),
				),
			).Run()
			if err != nil {
				return err
			}
		}
	}

	// Embedding provider — prompt when Anthropic is the LLM provider (no native embeddings).
	embeddingOptions := []huh.Option[string]{
		huh.NewOption("OpenAI (text-embedding-3-small)", "openai"),
		huh.NewOption("Google (text-embedding-004)", "google"),
		huh.NewOption("Ollama (local)", "ollama"),
		huh.NewOption("Skip (keyword search only)", ""),
	}
	embeddingDesc := "Which provider should handle text embeddings for semantic memory search?"
	if llmProvider == "anthropic" {
		embeddingDesc = "Anthropic does not offer embeddings. Choose a separate provider for semantic memory search, or skip for keyword-only."
	}
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Embedding Provider").
				Description(embeddingDesc).
				Options(embeddingOptions...).
				Value(&embeddingProvider),
		),
	).Run()
	if err != nil {
		return err
	}

	if embeddingProvider != "" {
		defaultEmbeddingModels := map[string]string{
			"openai": "text-embedding-3-small",
			"google": "text-embedding-004",
			"ollama": "nomic-embed-text",
		}
		embeddingModel = defaultEmbeddingModels[embeddingProvider]

		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Embedding model").
					Value(&embeddingModel).
					Placeholder(embeddingModel),
			),
		).Run()
		if err != nil {
			return err
		}

		// API key — use LLM key if same provider, otherwise prompt.
		if embeddingProvider != "ollama" {
			defaultEmbKeyEnv := defaultAPIKeyEnvs[embeddingProvider]
			if embeddingProvider == llmProvider {
				embeddingAPIKeyEnv = llmAPIKeyEnv
			} else {
				embeddingAPIKeyEnv = defaultEmbKeyEnv
				err = huh.NewForm(
					huh.NewGroup(
						huh.NewInput().
							Title("Embedding API key environment variable").
							Description("Name of the environment variable containing the embedding API key.").
							Value(&embeddingAPIKeyEnv).
							Placeholder(defaultEmbKeyEnv),
					),
				).Run()
				if err != nil {
					return err
				}
			}
		}

		// Custom base URL for OpenAI-compatible embedding endpoints.
		if embeddingProvider == "openai" {
			err = huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Embedding custom base URL").
						Description("Leave empty for default OpenAI. Set for compatible endpoints (LM Studio, vLLM, routers).").
						Value(&embeddingBaseURL).
						Placeholder("https://api.openai.com/v1"),
				),
			).Run()
			if err != nil {
				return err
			}
		}
	}

	// Generate canary token
	canary, err := crypto.GenerateCanary()
	if err != nil {
		return fmt.Errorf("failed to generate canary token: %w", err)
	}

	// Create workspace directory
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Create .openparallax subdirectory
	dotDir := filepath.Join(workspacePath, ".openparallax")
	if err := os.MkdirAll(dotDir, 0o755); err != nil {
		return fmt.Errorf("failed to create .openparallax directory: %w", err)
	}

	// Copy workspace templates (do not overwrite existing files)
	if err := copyTemplates(workspacePath); err != nil {
		return fmt.Errorf("failed to copy workspace templates: %w", err)
	}

	// Write config.yaml
	configPath := filepath.Join(workspacePath, "config.yaml")
	if err := writeConfig(configPath, workspacePath, llmProvider, llmModel, llmAPIKeyEnv,
		llmBaseURL, shieldProvider, shieldModel, shieldAPIKeyEnv, shieldBaseURL,
		embeddingProvider, embeddingModel, embeddingAPIKeyEnv, embeddingBaseURL); err != nil {
		return fmt.Errorf("failed to write config.yaml: %w", err)
	}

	// Initialize SQLite database
	dbPath := filepath.Join(dotDir, "openparallax.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	_ = db.Close()

	// Store canary token
	canaryPath := filepath.Join(dotDir, "canary.token")
	if err := os.WriteFile(canaryPath, []byte(canary), 0o600); err != nil {
		return fmt.Errorf("failed to write canary token: %w", err)
	}

	fmt.Println()
	fmt.Println("Workspace initialized successfully!")
	fmt.Printf("  Workspace: %s\n", workspacePath)
	fmt.Printf("  Config:    %s\n", configPath)
	fmt.Printf("  Database:  %s\n", dbPath)
	fmt.Println()
	fmt.Println("Next steps:")
	if llmAPIKeyEnv != "" {
		fmt.Printf("  1. Set your API key: export %s=<your-key>\n", llmAPIKeyEnv)
	}
	fmt.Println("  2. Start the agent:  openparallax start")
	fmt.Println()

	return nil
}

// copyTemplates copies embedded workspace template files to the workspace directory.
// Existing files are not overwritten.
func copyTemplates(workspacePath string) error {
	return fs.WalkDir(templates.WorkspaceFS, "files", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel("files", path)
		if relPath == "." {
			return nil
		}

		destPath := filepath.Join(workspacePath, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}

		// Do not overwrite existing files.
		if _, statErr := os.Stat(destPath); statErr == nil {
			return nil
		}

		data, readErr := templates.WorkspaceFS.ReadFile(path)
		if readErr != nil {
			return readErr
		}

		return os.WriteFile(destPath, data, 0o644)
	})
}

// writeConfig generates the config.yaml file from wizard inputs.
func writeConfig(path, workspace, llmProvider, llmModel, llmAPIKeyEnv,
	llmBaseURL, shieldProvider, shieldModel, shieldAPIKeyEnv, shieldBaseURL,
	embeddingProvider, embeddingModel, embeddingAPIKeyEnv, embeddingBaseURL string) error {

	// Do not overwrite existing config.
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "Warning: %s already exists. Not overwriting.\n", path)
		return nil
	}

	var sb strings.Builder
	sb.WriteString("# OpenParallax Configuration\n")
	sb.WriteString("# Generated by openparallax init\n\n")

	fmt.Fprintf(&sb, "workspace: %s\n\n", workspace)

	sb.WriteString("llm:\n")
	fmt.Fprintf(&sb, "  provider: %s\n", llmProvider)
	fmt.Fprintf(&sb, "  model: %s\n", llmModel)
	if llmAPIKeyEnv != "" {
		fmt.Fprintf(&sb, "  api_key_env: %s\n", llmAPIKeyEnv)
	}
	if llmBaseURL != "" {
		fmt.Fprintf(&sb, "  base_url: %s\n", llmBaseURL)
	}
	sb.WriteString("\n")

	sb.WriteString("shield:\n")
	if shieldProvider != "" {
		sb.WriteString("  evaluator:\n")
		fmt.Fprintf(&sb, "    provider: %s\n", shieldProvider)
		fmt.Fprintf(&sb, "    model: %s\n", shieldModel)
		if shieldAPIKeyEnv != "" {
			fmt.Fprintf(&sb, "    api_key_env: %s\n", shieldAPIKeyEnv)
		}
		if shieldBaseURL != "" {
			fmt.Fprintf(&sb, "    base_url: %s\n", shieldBaseURL)
		}
	}
	sb.WriteString("  policy_file: policies/default.yaml\n")
	sb.WriteString("  onnx_threshold: 0.85\n")
	sb.WriteString("  heuristic_enabled: true\n\n")

	if embeddingProvider != "" {
		sb.WriteString("memory:\n")
		sb.WriteString("  embedding:\n")
		fmt.Fprintf(&sb, "    provider: %s\n", embeddingProvider)
		fmt.Fprintf(&sb, "    model: %s\n", embeddingModel)
		if embeddingAPIKeyEnv != "" {
			fmt.Fprintf(&sb, "    api_key_env: %s\n", embeddingAPIKeyEnv)
		}
		if embeddingBaseURL != "" {
			fmt.Fprintf(&sb, "    base_url: %s\n", embeddingBaseURL)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("chronicle:\n")
	sb.WriteString("  max_snapshots: 100\n")
	sb.WriteString("  max_age_days: 30\n\n")

	sb.WriteString("web:\n")
	sb.WriteString("  enabled: true\n")
	sb.WriteString("  port: 3100\n")
	sb.WriteString("  auth: true\n\n")

	sb.WriteString("general:\n")
	sb.WriteString("  fail_closed: true\n")
	sb.WriteString("  rate_limit: 30\n")
	sb.WriteString("  verdict_ttl_seconds: 60\n")
	sb.WriteString("  daily_budget: 100\n")

	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// defaultShieldProvider picks a different provider than the agent for cross-model
// evaluation. Falls back to "openai" if the agent already uses a different one.
func defaultShieldProvider(agentProvider string) string {
	if agentProvider != "openai" {
		return "openai"
	}
	return "anthropic"
}

// expandTilde replaces a leading ~ with the home directory.
func expandTilde(path, home string) string {
	if strings.HasPrefix(path, "~") {
		return filepath.Join(home, path[1:])
	}
	return path
}
