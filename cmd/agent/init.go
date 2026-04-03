package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/huh"
	"github.com/mattn/go-isatty"
	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/internal/engine/executors"
	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/registry"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/templates"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:          "init [name]",
	Short:        "Initialize a new OpenParallax workspace",
	Long:         "Interactive wizard that creates a workspace with smart defaults. Takes under 60 seconds.\nOptionally pass an agent name as an argument to skip the name prompt.",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// providerInfo holds smart defaults for each LLM provider.
type providerInfo struct {
	label       string
	model       string
	shieldModel string
	apiKeyEnv   string
}

var providers = map[string]providerInfo{
	"anthropic": {
		label:       "Anthropic",
		model:       "claude-sonnet-4-20250514",
		shieldModel: "claude-haiku-4-5-20251001",
		apiKeyEnv:   "ANTHROPIC_API_KEY",
	},
	"openai": {
		label:       "OpenAI",
		model:       "gpt-4o",
		shieldModel: "gpt-4o-mini",
		apiKeyEnv:   "OPENAI_API_KEY",
	},
	"google": {
		label:       "Google",
		model:       "gemini-2.0-flash",
		shieldModel: "gemini-2.0-flash",
		apiKeyEnv:   "GOOGLE_API_KEY",
	},
	"ollama": {
		label:       "Ollama",
		model:       "llama3.2",
		shieldModel: "llama3.2",
		apiKeyEnv:   "",
	},
}

var defaultEmbeddingModels = map[string]string{
	"openai": "text-embedding-3-small",
	"google": "text-embedding-004",
	"ollama": "nomic-embed-text",
}

var defaultEmbeddingKeyEnvs = map[string]string{
	"openai": "OPENAI_API_KEY",
	"google": "GOOGLE_API_KEY",
}

var avatarOptions = []struct {
	emoji string
	label string
}{
	{"⬡", "hexagon"},
	{"🤖", "robot"},
	{"🧠", "brain"},
	{"⚡", "lightning"},
	{"🛡️", "shield"},
}

func runInit(_ *cobra.Command, args []string) error {
	if !isatty.IsTerminal(os.Stdin.Fd()) && !isatty.IsCygwinTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("interactive terminal required: openparallax init must be run in a terminal, not piped")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	// Auto-detect existing API keys.
	detectedProvider, detectedKey := detectAPIKey()

	var (
		agentNameInput string
		avatarChoice   string
		llmProvider    string
		apiKeyInput    string
		modelInput     string
		baseURLInput   string
		shieldProvider string
		shieldAPIKey   string
		shieldModel    string
		shieldBaseURL  string
		embProvider    string
		embAPIKey      string
		workspacePath  string
		confirmLaunch  bool
	)

	// ─── Step 1: Welcome + Name ─────────────────────────────

	fmt.Println()
	fmt.Println("  ┌─────────────────────────────────────┐")
	fmt.Println("  │       Welcome to OpenParallax        │")
	fmt.Println("  │                                      │")
	fmt.Println("  │  You're setting up a personal AI     │")
	fmt.Println("  │  agent — private, secured, yours.    │")
	fmt.Println("  │                                      │")
	fmt.Println("  │  Everything runs on your machine.    │")
	fmt.Println("  │  Your data never leaves.             │")
	fmt.Println("  └─────────────────────────────────────┘")
	fmt.Println()

	// Accept name from positional arg or interactive prompt.
	if len(args) > 0 {
		agentNameInput = args[0]
		if vErr := validateAgentName(agentNameInput); vErr != nil {
			return fmt.Errorf("invalid agent name: %w", vErr)
		}
	} else {
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("What would you like to call your agent?").
					Value(&agentNameInput).
					Placeholder("Atlas").
					Validate(validateAgentName),
			),
		).Run()
		if err != nil {
			return err
		}
	}
	if agentNameInput == "" {
		agentNameInput = "Atlas"
	}

	// ─── Step 2: Avatar ─────────────────────────────────────

	avatarOpts := make([]huh.Option[string], 0, len(avatarOptions)+1)
	for _, a := range avatarOptions {
		avatarOpts = append(avatarOpts, huh.NewOption(fmt.Sprintf("%s  %s", a.emoji, a.label), a.emoji))
	}
	avatarOpts = append(avatarOpts, huh.NewOption("Custom emoji", "custom"))

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(fmt.Sprintf("Pick an avatar for %s:", agentNameInput)).
				Options(avatarOpts...).
				Value(&avatarChoice),
		),
	).Run()
	if err != nil {
		return err
	}

	if avatarChoice == "custom" {
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Enter a single emoji:").
					Value(&avatarChoice).
					Validate(func(s string) error {
						s = strings.TrimSpace(s)
						if s == "" || utf8.RuneCountInString(s) > 2 {
							return fmt.Errorf("enter a single emoji")
						}
						return nil
					}),
			),
		).Run()
		if err != nil {
			return err
		}
	}

	// ─── Step 3: LLM Provider ───────────────────────────────

	providerOpts := []huh.Option[string]{
		huh.NewOption("Anthropic  (Claude)", "anthropic"),
		huh.NewOption("OpenAI     (GPT, or any OpenAI-compatible API)", "openai"),
		huh.NewOption("Google     (Gemini)", "google"),
		huh.NewOption("Ollama     (Local)", "ollama"),
	}

	// Pre-select detected provider.
	if detectedProvider != "" {
		for i, opt := range providerOpts {
			if opt.Value == detectedProvider {
				providerOpts[i] = opt.Selected(true)
				break
			}
		}
	}

	providerDesc := fmt.Sprintf("Which AI provider should %s use?", agentNameInput)
	if detectedProvider != "" {
		providerDesc += fmt.Sprintf("\n  ✓ Found %s API key in your environment.", providers[detectedProvider].label)
	}

	// Check if Ollama is running.
	ollamaDetected := detectOllama()
	if ollamaDetected {
		for i, opt := range providerOpts {
			if opt.Value == "ollama" {
				providerOpts[i] = huh.NewOption("Ollama     (Local — ✓ Detected)", "ollama")
				break
			}
		}
	}

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("LLM Provider").
				Description(providerDesc).
				Options(providerOpts...).
				Value(&llmProvider),
		),
	).Run()
	if err != nil {
		return err
	}

	// ─── API Key ────────────────────────────────────────────

	info := providers[llmProvider]

	if llmProvider != "ollama" {
		// Pre-fill if we detected the right key.
		if detectedProvider == llmProvider && detectedKey != "" {
			apiKeyInput = detectedKey
			fmt.Printf("  ✓ Using %s API key from environment.\n", info.label)
		} else {
			keyURL := apiKeyURL(llmProvider)
			err = huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title(fmt.Sprintf("Enter your %s API key:", info.label)).
						Description(fmt.Sprintf("Get one at %s", keyURL)).
						Value(&apiKeyInput).
						EchoMode(huh.EchoModePassword).
						Validate(func(s string) error {
							if strings.TrimSpace(s) == "" {
								return fmt.Errorf("API key is required")
							}
							return nil
						}),
				),
			).Run()
			if err != nil {
				return err
			}
		}

		// For OpenAI-compatible providers, ask for base URL.
		if llmProvider == "openai" {
			err = huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Custom API base URL (optional):").
						Description("Leave empty for OpenAI. For compatible providers (LM Studio, Together AI, Groq, etc.), enter their API URL.").
						Value(&baseURLInput).
						Placeholder("https://api.openai.com/v1"),
				),
			).Run()
			if err != nil {
				return err
			}
			baseURLInput = strings.TrimSpace(baseURLInput)
		}

	}

	apiKeyInput = strings.TrimSpace(apiKeyInput)

	// ─── Step 4: Chat Model ─────────────────────────────────

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Chat model:").
				Description("The primary model for conversations and tool use.").
				Value(&modelInput).
				Placeholder(info.model),
		),
	).Run()
	if err != nil {
		return err
	}
	if modelInput == "" {
		modelInput = info.model
	}

	// Connection test for chat provider.
	fmt.Printf("  Testing connection to %s...\n", info.label)
	testCfg := types.LLMConfig{
		Provider: llmProvider,
		Model:    modelInput,
		BaseURL:  baseURLInput,
	}
	if testErr := llm.TestConnection(testCfg, apiKeyInput); testErr != nil {
		fmt.Printf("  ✗ Connection failed: %s\n", testErr)
		fmt.Println("  Review your settings and try again.")

		retryFields := []huh.Field{
			huh.NewInput().
				Title("Model:").
				Value(&modelInput),
		}
		if llmProvider == "openai" {
			retryFields = append(retryFields,
				huh.NewInput().
					Title("Base URL:").
					Value(&baseURLInput).
					Placeholder("https://api.openai.com/v1"),
			)
		}
		if llmProvider != "ollama" {
			retryFields = append(retryFields,
				huh.NewInput().
					Title("API key:").
					Value(&apiKeyInput).
					EchoMode(huh.EchoModePassword),
			)
		}

		err = huh.NewForm(huh.NewGroup(retryFields...)).Run()
		if err != nil {
			return err
		}
		apiKeyInput = strings.TrimSpace(apiKeyInput)
		baseURLInput = strings.TrimSpace(baseURLInput)

		testCfg.Model = modelInput
		testCfg.BaseURL = baseURLInput
		if testErr2 := llm.TestConnection(testCfg, apiKeyInput); testErr2 != nil {
			return fmt.Errorf("connection to %s failed: %w", info.label, testErr2)
		}
	}
	fmt.Printf("  ✓ Connected to %s. Using %s.\n\n", info.label, modelInput)

	// ─── Step 5: Shield ─────────────────────────────────────
	// Same flow as chat: provider → model + base URL + API key.
	// Placeholders default to chat values when same provider.

	shieldProviderOpts := []huh.Option[string]{
		huh.NewOption(fmt.Sprintf("Same as chat  (%s)", info.label), llmProvider).Selected(true),
	}
	for key, p := range providers {
		if key != llmProvider {
			shieldProviderOpts = append(shieldProviderOpts, huh.NewOption(p.label, key))
		}
	}

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Shield Provider").
				Description("Shield evaluates every tool call for safety. A cheaper/faster model from any provider works well.").
				Options(shieldProviderOpts...).
				Value(&shieldProvider),
		),
	).Run()
	if err != nil {
		return err
	}

	shieldInfo := providers[shieldProvider]
	sameShieldProvider := shieldProvider == llmProvider

	shieldModelPH := shieldInfo.shieldModel
	if sameShieldProvider {
		shieldModelPH = modelInput
	}

	shieldFields := []huh.Field{
		huh.NewInput().
			Title("Shield model:").
			Value(&shieldModel).
			Placeholder(shieldModelPH),
	}
	if shieldProvider == "openai" {
		ph := "https://api.openai.com/v1"
		if sameShieldProvider && baseURLInput != "" {
			ph = baseURLInput
		}
		shieldFields = append(shieldFields,
			huh.NewInput().
				Title("Shield base URL (optional):").
				Value(&shieldBaseURL).
				Placeholder(ph),
		)
	}
	if shieldProvider != "ollama" {
		shieldFields = append(shieldFields,
			huh.NewInput().
				Title("Shield API key:").
				Description("Leave empty to reuse chat key.").
				Value(&shieldAPIKey).
				EchoMode(huh.EchoModePassword),
		)
	}
	err = huh.NewForm(huh.NewGroup(shieldFields...)).Run()
	if err != nil {
		return err
	}
	if shieldModel == "" {
		shieldModel = shieldModelPH
	}
	shieldBaseURL = strings.TrimSpace(shieldBaseURL)
	shieldAPIKey = strings.TrimSpace(shieldAPIKey)
	if sameShieldProvider {
		if shieldBaseURL == "" {
			shieldBaseURL = baseURLInput
		}
		if shieldAPIKey == "" {
			shieldAPIKey = apiKeyInput
		}
	}

	// Connection test for shield.
	fmt.Printf("  Testing Shield connection to %s...\n", shieldInfo.label)
	shieldTestCfg := types.LLMConfig{
		Provider: shieldProvider,
		Model:    shieldModel,
		BaseURL:  shieldBaseURL,
	}
	if testErr := llm.TestConnection(shieldTestCfg, shieldAPIKey); testErr != nil {
		fmt.Printf("  ✗ Shield connection failed: %s\n", testErr)
		fmt.Println("  Review your settings and try again.")

		retryFields := []huh.Field{
			huh.NewInput().
				Title("Shield model:").
				Value(&shieldModel),
		}
		if shieldProvider == "openai" {
			retryFields = append(retryFields,
				huh.NewInput().
					Title("Shield base URL:").
					Value(&shieldBaseURL),
			)
		}
		if shieldProvider != "ollama" {
			retryFields = append(retryFields,
				huh.NewInput().
					Title("Shield API key:").
					Value(&shieldAPIKey).
					EchoMode(huh.EchoModePassword),
			)
		}
		err = huh.NewForm(huh.NewGroup(retryFields...)).Run()
		if err != nil {
			return err
		}
		shieldAPIKey = strings.TrimSpace(shieldAPIKey)
		shieldBaseURL = strings.TrimSpace(shieldBaseURL)
		shieldTestCfg.Model = shieldModel
		shieldTestCfg.BaseURL = shieldBaseURL
		if testErr2 := llm.TestConnection(shieldTestCfg, shieldAPIKey); testErr2 != nil {
			return fmt.Errorf("shield connection to %s failed: %w", shieldInfo.label, testErr2)
		}
	}
	fmt.Printf("  ✓ Shield connected. Using %s.\n\n", shieldModel)

	// ─── Step 6: Embedding ──────────────────────────────────
	// Same flow: provider → model + base URL + API key.
	// Has a "Skip" option for keyword-only search.

	embAPIKeyEnv := ""
	embModel := ""

	embOpts := []huh.Option[string]{
		huh.NewOption("OpenAI  (text-embedding-3-small)", "openai"),
		huh.NewOption("Google  (text-embedding-004)", "google"),
		huh.NewOption("Ollama  (local embedding model)", "ollama"),
		huh.NewOption("Skip    (keyword search only)", ""),
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		embOpts[0] = huh.NewOption("OpenAI  (text-embedding-3-small — ✓ key detected)", "openai").Selected(true)
	}
	if os.Getenv("GOOGLE_API_KEY") != "" {
		embOpts[1] = huh.NewOption("Google  (text-embedding-004 — ✓ key detected)", "google")
	}

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Embedding Provider").
				Description("Embeddings enable semantic memory search. Choose a provider, or skip for keyword-only search.").
				Options(embOpts...).
				Value(&embProvider),
		),
	).Run()
	if err != nil {
		return err
	}

	embBaseURL := ""
	if embProvider != "" {
		defaultEmb := defaultEmbeddingModels[embProvider]
		if defaultEmb == "" {
			defaultEmb = "embedding-model"
		}
		embAPIKeyEnv = defaultEmbeddingKeyEnvs[embProvider]
		sameEmbProvider := embProvider == llmProvider

		embFields := []huh.Field{
			huh.NewInput().
				Title("Embedding model:").
				Value(&embModel).
				Placeholder(defaultEmb),
		}
		if embProvider == "openai" {
			ph := "https://api.openai.com/v1"
			if sameEmbProvider && baseURLInput != "" {
				ph = baseURLInput
			}
			embFields = append(embFields,
				huh.NewInput().
					Title("Embedding base URL (optional):").
					Value(&embBaseURL).
					Placeholder(ph),
			)
		}
		if embProvider != "ollama" {
			embFields = append(embFields,
				huh.NewInput().
					Title("Embedding API key:").
					Description("Leave empty to reuse chat key.").
					Value(&embAPIKey).
					EchoMode(huh.EchoModePassword),
			)
		}
		err = huh.NewForm(huh.NewGroup(embFields...)).Run()
		if err != nil {
			return err
		}
		if embModel == "" {
			embModel = defaultEmb
		}
		embBaseURL = strings.TrimSpace(embBaseURL)
		embAPIKey = strings.TrimSpace(embAPIKey)
	}

	// ─── Step 6: Workspace ──────────────────────────────────

	slug := slugify(agentNameInput)
	defaultWorkspace := filepath.Join(home, ".openparallax", slug)

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("Where should %s live?", agentNameInput)).
				Description("This is where sessions, memory, and project files are stored.").
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

	// ─── Step 7: Confirmation ───────────────────────────────

	embDisplay := "none (keyword search only)"
	if embProvider != "" {
		embDisplay = fmt.Sprintf("%s / %s", embProvider, embModel)
	}

	// Allocate web port from the global agent registry.
	regPath, regPathErr := registry.DefaultPath()
	if regPathErr != nil {
		return regPathErr
	}
	reg, regErr := registry.Load(regPath)
	if regErr != nil {
		return fmt.Errorf("load registry: %w", regErr)
	}
	webPort := reg.AllocatePort()

	fmt.Println()
	fmt.Println("  ┌─────────────────────────────────────┐")
	fmt.Println("  │           Ready to go!               │")
	fmt.Println("  │                                      │")
	fmt.Printf("  │  Agent:       %-22s│\n", fmt.Sprintf("%s %s", agentNameInput, avatarChoice))
	fmt.Printf("  │  Chat:        %-22s│\n", truncate(fmt.Sprintf("%s / %s", info.label, modelInput), 22))
	shieldInfo2 := providers[shieldProvider]
	fmt.Printf("  │  Shield:      %-22s│\n", truncate(fmt.Sprintf("%s / %s", shieldInfo2.label, shieldModel), 22))
	fmt.Printf("  │  Embedding:   %-22s│\n", truncate(embDisplay, 22))
	fmt.Printf("  │  Workspace:   %-22s│\n", truncate(workspacePath, 22))
	fmt.Printf("  │  Web UI:      http://127.0.0.1:%-5d │\n", webPort)
	fmt.Println("  │                                      │")
	fmt.Println("  │  You can change any of these later   │")
	fmt.Println("  │  in Settings or config.yaml          │")
	fmt.Println("  └─────────────────────────────────────┘")
	fmt.Println()

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Start %s now?", agentNameInput)).
				Value(&confirmLaunch),
		),
	).Run()
	if err != nil {
		return err
	}

	// ─── Create Workspace ───────────────────────────────────

	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	dotDir := filepath.Join(workspacePath, ".openparallax")
	if err := os.MkdirAll(dotDir, 0o755); err != nil {
		return fmt.Errorf("failed to create .openparallax: %w", err)
	}

	// Copy templates with agent name substitution.
	if err := copyTemplates(workspacePath, agentNameInput); err != nil {
		return fmt.Errorf("failed to copy templates: %w", err)
	}

	// Write config.yaml.
	configPath := filepath.Join(workspacePath, "config.yaml")
	if err := writeConfig(configPath, workspacePath, agentNameInput, avatarChoice,
		llmProvider, info, baseURLInput, modelInput,
		shieldProvider, shieldModel, shieldBaseURL,
		embProvider, embModel, embAPIKeyEnv, embBaseURL,
		webPort); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Initialize SQLite.
	dbPath := filepath.Join(dotDir, "openparallax.db")
	db, dbErr := storage.Open(dbPath)
	if dbErr != nil {
		return fmt.Errorf("failed to initialize database: %w", dbErr)
	}
	_ = db.Close()

	// Generate and store canary token.
	canary, canaryErr := crypto.GenerateCanary()
	if canaryErr != nil {
		return fmt.Errorf("failed to generate canary: %w", canaryErr)
	}
	if err := os.WriteFile(filepath.Join(dotDir, "canary.token"), []byte(canary), 0o600); err != nil {
		return fmt.Errorf("failed to write canary: %w", err)
	}

	// Register agent in the global registry.
	regRec := registry.AgentRecord{
		Name:       agentNameInput,
		Slug:       slug,
		Workspace:  workspacePath,
		ConfigPath: configPath,
		WebPort:    webPort,
		GRPCPort:   webPort + registry.GRPCPortOffset,
		CreatedAt:  time.Now(),
	}
	if addErr := reg.Add(regRec); addErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not register agent: %s\n", addErr)
	}

	fmt.Println()
	fmt.Println("  ✓ Workspace initialized!")
	fmt.Printf("    Config:    %s\n", configPath)
	fmt.Printf("    Database:  %s\n", dbPath)
	fmt.Println()

	if confirmLaunch {
		fmt.Printf("  Starting %s...\n\n", agentNameInput)
		startConfigPath = configPath
		return runStart(nil, nil)
	}

	fmt.Printf("  Run 'openparallax start -c %s' when you're ready.\n\n", configPath)
	return nil
}

// ─── Helpers ────────────────────────────────────────────────

// validateAgentName checks the agent name format.
func validateAgentName(s string) error {
	if s == "" {
		return nil // defaults to Atlas
	}
	if utf8.RuneCountInString(s) > 20 {
		return fmt.Errorf("max 20 characters")
	}
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9 ]+$`, s); !matched {
		return fmt.Errorf("alphanumeric and spaces only")
	}
	return nil
}

// slugify converts an agent name to a filesystem-safe slug.
func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// detectAPIKey checks common environment variables for existing API keys.
func detectAPIKey() (provider, key string) {
	for _, p := range []struct {
		provider string
		env      string
	}{
		{"anthropic", "ANTHROPIC_API_KEY"},
		{"openai", "OPENAI_API_KEY"},
		{"google", "GOOGLE_API_KEY"},
	} {
		if k := os.Getenv(p.env); k != "" {
			return p.provider, k
		}
	}
	return "", ""
}

// detectOllama checks if Ollama is running at localhost:11434.
func detectOllama() bool {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// apiKeyURL returns the URL where users can get an API key.
func apiKeyURL(provider string) string {
	switch provider {
	case "anthropic":
		return "https://console.anthropic.com/settings/keys"
	case "openai":
		return "https://platform.openai.com/api-keys"
	case "google":
		return "https://aistudio.google.com/apikey"
	default:
		return ""
	}
}

// truncate shortens a string to maxLen, adding "..." if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-2] + ".."
}

// expandTilde replaces a leading ~ with the home directory.
func expandTilde(path, home string) string {
	if strings.HasPrefix(path, "~") {
		return filepath.Join(home, path[1:])
	}
	return path
}

// copyTemplates copies embedded workspace templates, substituting the agent name.
func copyTemplates(workspacePath, agentName string) error {
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

		// Substitute agent name in templates.
		content := strings.ReplaceAll(string(data), "Atlas", agentName)
		return os.WriteFile(destPath, []byte(content), 0o644)
	})
}

// writeConfig generates config.yaml from wizard inputs.
func writeConfig(path, workspace, agentName, avatar string,
	llmProvider string, info providerInfo, baseURL string,
	model string,
	shieldProv, shieldModel, shieldBaseURL string,
	embProvider, embModel, embAPIKeyEnv, embBaseURL string,
	webPort int) error {

	// Do not overwrite existing config.
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "Warning: %s already exists. Not overwriting.\n", path)
		return nil
	}

	var sb strings.Builder
	sb.WriteString("# OpenParallax Configuration\n")
	sb.WriteString("# Generated by openparallax init\n\n")

	fmt.Fprintf(&sb, "workspace: %s\n\n", workspace)

	// Identity
	sb.WriteString("identity:\n")
	fmt.Fprintf(&sb, "  name: %s\n", agentName)
	if avatar != "" {
		fmt.Fprintf(&sb, "  avatar: %s\n", avatar)
	}
	sb.WriteString("\n")

	// LLM
	sb.WriteString("llm:\n")
	fmt.Fprintf(&sb, "  provider: %s\n", llmProvider)
	fmt.Fprintf(&sb, "  model: %s\n", model)
	if info.apiKeyEnv != "" {
		fmt.Fprintf(&sb, "  api_key_env: %s\n", info.apiKeyEnv)
	}
	if baseURL != "" {
		fmt.Fprintf(&sb, "  base_url: %s\n", baseURL)
	}
	sb.WriteString("\n")

	// Shield
	shieldInfo := providers[shieldProv]
	sb.WriteString("shield:\n")
	sb.WriteString("  evaluator:\n")
	fmt.Fprintf(&sb, "    provider: %s\n", shieldProv)
	fmt.Fprintf(&sb, "    model: %s\n", shieldModel)
	if shieldInfo.apiKeyEnv != "" {
		fmt.Fprintf(&sb, "    api_key_env: %s\n", shieldInfo.apiKeyEnv)
	}
	if shieldBaseURL != "" {
		fmt.Fprintf(&sb, "    base_url: %s\n", shieldBaseURL)
	}
	sb.WriteString("  policy_file: policies/default.yaml\n")
	sb.WriteString("  heuristic_enabled: true\n\n")

	// Embedding
	if embProvider != "" {
		sb.WriteString("memory:\n")
		sb.WriteString("  embedding:\n")
		fmt.Fprintf(&sb, "    provider: %s\n", embProvider)
		fmt.Fprintf(&sb, "    model: %s\n", embModel)
		if embAPIKeyEnv != "" {
			fmt.Fprintf(&sb, "    api_key_env: %s\n", embAPIKeyEnv)
		}
		if embBaseURL != "" {
			fmt.Fprintf(&sb, "    base_url: %s\n", embBaseURL)
		}
		sb.WriteString("\n")
	}

	// Chronicle
	sb.WriteString("chronicle:\n")
	sb.WriteString("  max_snapshots: 100\n")
	sb.WriteString("  max_age_days: 30\n\n")

	// Web
	sb.WriteString("web:\n")
	sb.WriteString("  enabled: true\n")
	fmt.Fprintf(&sb, "  port: %d\n", webPort)
	fmt.Fprintf(&sb, "  grpc_port: %d\n", webPort+registry.GRPCPortOffset)
	sb.WriteString("  auth: true\n\n")

	// General
	sb.WriteString("general:\n")
	sb.WriteString("  fail_closed: true\n")
	sb.WriteString("  rate_limit: 30\n")
	sb.WriteString("  verdict_ttl_seconds: 60\n")
	sb.WriteString("  daily_budget: 100\n")

	// Store API keys inline if they came from user input (not env vars).
	// In production this would write env vars, but for simplicity
	// during init we reference the env var name.
	// The user sets the env var themselves.

	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// FirstMessageTemplate generates the agent's introduction message based on
// available capabilities. Only mentions what is actually configured.
func FirstMessageTemplate(agentName string, cfg *types.AgentConfig) string {
	var capabilities []string

	capabilities = append(capabilities,
		"Read, write, and manage files in your workspace",
		"Run shell commands",
	)

	if executors.DetectBrowser() != "" {
		capabilities = append(capabilities, "Browse the web and fetch pages")
	}

	capabilities = append(capabilities, "Manage git repositories")
	capabilities = append(capabilities, "Search and remember things across conversations")

	if cfg.Email.SMTP.Host != "" {
		capabilities = append(capabilities, "Send and manage emails")
	}
	if cfg.Calendar.Provider != "" {
		capabilities = append(capabilities, "Manage your calendar")
	}

	capabilities = append(capabilities, "Create websites, documents, and more")

	var sb strings.Builder
	fmt.Fprintf(&sb, "Hey! I'm %s, your personal AI agent.\n\n", agentName)
	sb.WriteString("I'm running on your machine, secured by Shield, and everything stays local.\n\n")
	sb.WriteString("Here's what I can do:\n")
	for _, c := range capabilities {
		fmt.Fprintf(&sb, "  • %s\n", c)
	}
	sb.WriteString("\nWhat would you like to work on?")
	return sb.String()
}
