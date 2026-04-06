// Package engine implements the privileged execution engine that evaluates
// and executes agent-proposed actions. The engine runs as a separate OS process
// and communicates with the agent via gRPC.
package engine

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/openparallax/openparallax/audit"
	"github.com/openparallax/openparallax/chronicle"
	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/internal/agent"
	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/engine/executors"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/oauth"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	pb "github.com/openparallax/openparallax/internal/types/pb"
	"github.com/openparallax/openparallax/llm"
	"github.com/openparallax/openparallax/mcp"
	"github.com/openparallax/openparallax/memory"
	memsqlite "github.com/openparallax/openparallax/memory/sqlite"
	"github.com/openparallax/openparallax/shield"
	"google.golang.org/grpc"
)

// Engine is the execution engine and gRPC server.
type Engine struct {
	pb.UnimplementedAgentServiceServer
	pb.UnimplementedClientServiceServer
	pb.UnimplementedSubAgentServiceServer

	cfg           *types.AgentConfig
	llm           llm.Provider
	modelRegistry *llm.Registry
	log           *logging.Logger
	agent         *agent.Agent
	executors     *executors.Registry
	shield        *shield.Pipeline
	enricher      *shield.MetadataEnricher
	chronicle     *chronicle.Chronicle
	memory        *memory.Manager
	audit         *audit.Logger
	verifier      *Verifier
	db            *storage.DB
	mcpManager    *mcp.Manager

	tier3Manager      *Tier3Manager
	approvalNotifier  ApprovalNotifier
	channelController ChannelController
	subAgentManager   *SubAgentManager
	oauthManager    *oauth.Manager
	broadcaster     *EventBroadcaster

	server   *grpc.Server
	listener net.Listener

	agentStream   pb.AgentService_RunSessionServer
	currentMsgOTR bool

	sandboxStatus      sandboxInfo
	heartbeatSessionID string
	agentAuthToken     string

	backgroundCtx    context.Context
	backgroundCancel context.CancelFunc
	backgroundWG     sync.WaitGroup

	mu       sync.Mutex
	shutdown bool
}

// sandboxInfo holds the kernel sandbox state for API reporting.
type sandboxInfo struct {
	Active     bool
	Mode       string
	Version    int
	Filesystem bool
	Network    bool
	Reason     string
}

// New creates an Engine from a config file path. When verbose is true,
// diagnostic output for each pipeline stage is written to engine.log.
func New(configPath string, verbose bool) (*Engine, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("config load: %w", err)
	}

	logLevel := logging.LevelInfo
	if verbose {
		logLevel = logging.LevelDebug
	}
	logPath := filepath.Join(cfg.Workspace, ".openparallax", "engine.log")
	log, err := logging.New(logPath, logLevel)
	if err != nil {
		log = logging.Nop()
	}

	provider, err := llm.NewProvider(cfg.LLM)
	if err != nil {
		return nil, fmt.Errorf("llm provider: %w", err)
	}

	// Build multi-model registry from config.
	modelReg := llm.NewRegistry()
	for _, m := range cfg.Models {
		if regErr := modelReg.Register(llm.ModelEntry{
			Name: m.Name, Provider: m.Provider,
			Model: m.Model, APIKeyEnv: m.APIKeyEnv, BaseURL: m.BaseURL,
		}); regErr != nil {
			log.Warn("model_register_failed", "name", m.Name, "error", regErr)
		}
	}
	if cfg.Roles.Chat != "" {
		_ = modelReg.SetRole("chat", cfg.Roles.Chat)
	}
	if cfg.Roles.Shield != "" {
		_ = modelReg.SetRole("shield", cfg.Roles.Shield)
	}
	if cfg.Roles.Embedding != "" {
		_ = modelReg.SetRole("embedding", cfg.Roles.Embedding)
	}
	if cfg.Roles.SubAgent != "" {
		_ = modelReg.SetRole("sub_agent", cfg.Roles.SubAgent)
	}
	if cfg.Roles.Image != "" {
		_ = modelReg.SetRole("image", cfg.Roles.Image)
	}
	if cfg.Roles.Video != "" {
		_ = modelReg.SetRole("video", cfg.Roles.Video)
	}

	dbPath := filepath.Join(cfg.Workspace, ".openparallax", "openparallax.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	db.RepairSessions()
	canaryToken := readCanaryToken(cfg.Workspace)

	// Create OAuth manager (nil-safe if no providers configured).
	var oauthMgr *oauth.Manager
	oauthProviders := buildOAuthProviders(cfg)
	if len(oauthProviders) > 0 {
		oauthMgr, _ = oauth.NewManager(db, canaryToken, oauthProviders, log)
	}

	registry := executors.NewRegistry(cfg.Workspace, cfg, oauthMgr, log)
	if len(cfg.Tools.DisabledGroups) > 0 {
		registry.Groups.DisableGroups(cfg.Tools.DisabledGroups)
		log.Info("tools_groups_disabled", "groups", strings.Join(cfg.Tools.DisabledGroups, ", "))
	}

	configDir := filepath.Dir(configPath)
	policyFile := resolveFilePath(cfg.Shield.PolicyFile, configDir, cfg.Workspace)
	promptPath := resolveFilePath("prompts/evaluator-v1.md", configDir, cfg.Workspace)

	// Validate security-critical files exist.
	if _, statErr := os.Stat(policyFile); statErr != nil {
		return nil, fmt.Errorf("shield policy file not found: %s — run 'openparallax init' to create it", policyFile)
	}
	if cfg.Shield.Evaluator.Provider != "" {
		if _, statErr := os.Stat(promptPath); statErr != nil {
			return nil, fmt.Errorf("shield evaluator prompt not found: %s — run 'openparallax init' to create it", promptPath)
		}
	}

	// Warn if skills directory is empty.
	skillsDir := filepath.Join(cfg.Workspace, "skills")
	if entries, readErr := os.ReadDir(skillsDir); readErr != nil || len(entries) == 0 {
		log.Warn("no_skills_found", "message", "no skills found — agent runs without domain-specific guidance")
	}

	shieldPipeline, err := shield.NewPipeline(shield.Config{
		PolicyFile:       policyFile,
		OnnxThreshold:    cfg.Shield.OnnxThreshold,
		HeuristicEnabled: cfg.Shield.HeuristicEnabled,
		ClassifierAddr:   cfg.Shield.ClassifierAddr,
		Evaluator:        &cfg.Shield.Evaluator,
		CanaryToken:      canaryToken,
		PromptPath:       promptPath,
		FailClosed:       cfg.General.FailClosed,
		RateLimit:        cfg.General.RateLimit,
		VerdictTTL:       cfg.General.VerdictTTLSeconds,
		DailyBudget:      cfg.General.DailyBudget,
		Log:              nil, // Shield uses its own logging internally
	})
	if err != nil {
		return nil, fmt.Errorf("shield init: %w", err)
	}

	auditPath := filepath.Join(cfg.Workspace, ".openparallax", "audit.jsonl")
	auditLogger, err := audit.NewLogger(auditPath)
	if err != nil {
		return nil, fmt.Errorf("audit logger: %w", err)
	}
	auditLogger.SetDB(db)

	chron, err := chronicle.New(cfg.Workspace, cfg.Chronicle, db)
	if err != nil {
		return nil, fmt.Errorf("chronicle: %w", err)
	}

	memStore := memsqlite.NewStore(db)
	mem := memory.NewManager(cfg.Workspace, memStore, provider)
	registry.RegisterMemory(mem)

	// Initialize vector search pipeline (chunk indexing + embeddings).
	embCfg := cfg.Memory.Embedding
	embedder := memory.NewEmbeddingProvider(memory.EmbeddingConfig{
		Provider:  embCfg.Provider,
		Model:     embCfg.Model,
		APIKeyEnv: embCfg.APIKeyEnv,
		BaseURL:   embCfg.BaseURL,
	})
	embDimension := 0
	if embedder != nil {
		embDimension = embedder.Dimension()
	}
	vectorSearcher := memory.NewVectorSearcher(memStore, log, embDimension)
	indexer := memory.NewIndexer(memStore, embedder, vectorSearcher, log)

	// Index workspace files on startup (skips unchanged files via content hash).
	indexCtx, indexCancel := context.WithTimeout(context.Background(), 60*time.Second)
	indexer.IndexWorkspace(indexCtx, cfg.Workspace)
	indexCancel()

	if embedder != nil {
		log.Info("vector_search_enabled", "provider", embCfg.Provider, "model", embCfg.Model)
	} else {
		log.Info("vector_search_disabled", "reason", "no embedding provider configured")
	}

	ag := agent.NewAgent(provider, cfg.Workspace, mem, cfg.Skills.Disabled)
	ag.Context.OutputSanitization = cfg.General.OutputSanitization

	// MCP manager (optional — only if servers are configured).
	var mcpMgr *mcp.Manager
	if len(cfg.MCP.Servers) > 0 {
		mcpMgr = mcp.NewManager(cfg.MCP.Servers, log)
	}

	eng := &Engine{
		cfg:           cfg,
		llm:           provider,
		modelRegistry: modelReg,
		log:           log,
		agent:         ag,
		executors:     registry,
		shield:        shieldPipeline,
		enricher:      shield.NewMetadataEnricher(),
		chronicle:     chron,
		memory:        mem,
		audit:         auditLogger,
		verifier:      NewVerifier(),
		db:            db,
		mcpManager:    mcpMgr,
		tier3Manager:  NewTier3Manager(cfg.Shield.Tier3.MaxPerHour, cfg.Shield.Tier3.TimeoutSeconds),
		oauthManager:  oauthMgr,
		broadcaster:   NewEventBroadcaster(),
	}
	eng.backgroundCtx, eng.backgroundCancel = context.WithCancel(context.Background())

	// Start llm_usage retention in background.
	go func() {
		if pruned, pruneErr := db.PruneLLMUsage(90); pruneErr != nil {
			log.Warn("llm_usage_prune_failed", "error", pruneErr)
		} else if pruned > 0 {
			log.Info("llm_usage_pruned", "rows", pruned)
		}

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-eng.backgroundCtx.Done():
				return
			case <-ticker.C:
				if pruned, pruneErr := db.PruneLLMUsage(90); pruneErr != nil {
					log.Warn("llm_usage_prune_failed", "error", pruneErr)
				} else if pruned > 0 {
					log.Info("llm_usage_pruned", "rows", pruned)
				}
			}
		}
	}()

	// Start file watcher for automatic reindexing on memory file changes.
	if watchErr := memory.StartWatcher(eng.backgroundCtx, cfg.Workspace, indexer, log); watchErr != nil {
		log.Warn("memory_watcher_failed", "error", watchErr)
	}

	// Discover MCP tools and register them as loadable groups so the LLM
	// can call load_tools(["mcp:<server>"]) to access external tools.
	if mcpMgr != nil {
		ctx, cancel := context.WithTimeout(eng.backgroundCtx, 30*time.Second)
		serverTools := mcpMgr.DiscoverToolsByServer(ctx)
		cancel()
		if len(serverTools) > 0 {
			registry.Groups.RegisterMCPTools(serverTools)
			for name, tools := range serverTools {
				log.Info("mcp_tools_registered", "server", name, "tools", len(tools))
			}
		}
	}

	return eng, nil
}

// Start begins the gRPC server. If a non-zero port is provided, the server
// binds to that port; otherwise a dynamic port is allocated by the OS.
func (e *Engine) Start(listenPort ...int) (int, error) {
	addr := "localhost:0"
	if len(listenPort) > 0 && listenPort[0] > 0 {
		addr = fmt.Sprintf("localhost:%d", listenPort[0])
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		// Fall back to dynamic port if the requested port is in use.
		if addr != "localhost:0" {
			lis, err = net.Listen("tcp", "localhost:0")
			if err != nil {
				return 0, fmt.Errorf("failed to listen: %w", err)
			}
		} else {
			return 0, fmt.Errorf("failed to listen: %w", err)
		}
	}
	e.listener = lis

	e.server = grpc.NewServer()
	pb.RegisterAgentServiceServer(e.server, e)
	pb.RegisterClientServiceServer(e.server, e)
	pb.RegisterSubAgentServiceServer(e.server, e)

	go func() {
		_ = e.server.Serve(lis)
	}()

	tcpAddr, ok := lis.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("listener address is not TCP")
	}
	e.log.Info("grpc_server_started", "port", tcpAddr.Port)
	return tcpAddr.Port, nil
}

// Stop gracefully shuts down the engine. Background tasks (session
// summarization) get a 5-second grace period before forced cancellation.
func (e *Engine) Stop() {
	e.mu.Lock()
	e.shutdown = true
	e.mu.Unlock()

	// Cancel background tasks and wait up to 5 seconds for in-flight work.
	if e.backgroundCancel != nil {
		e.backgroundCancel()
	}
	done := make(chan struct{})
	go func() {
		e.backgroundWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		e.log.Warn("background_tasks_timeout", "message", "in-flight background tasks did not finish within 5s")
	}

	if e.subAgentManager != nil {
		e.subAgentManager.Shutdown()
	}
	if e.mcpManager != nil {
		e.mcpManager.ShutdownAll()
	}
	if e.server != nil {
		e.server.GracefulStop()
	}
	if e.audit != nil {
		_ = e.audit.Close()
	}
	if e.db != nil {
		_ = e.db.Close()
	}
	e.log.Info("grpc_server_shutdown")
}

// Port returns the port the engine is listening on, or 0 if not started.
func (e *Engine) Port() int {
	if e.listener == nil {
		return 0
	}
	tcpAddr, ok := e.listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0
	}
	return tcpAddr.Port
}

// --- Accessors for the web server ---

// DB returns the storage database.
func (e *Engine) DB() *storage.DB { return e.db }

// Memory returns the memory manager.
func (e *Engine) Memory() *memory.Manager { return e.memory }

// Config returns the agent configuration.
func (e *Engine) Config() *types.AgentConfig { return e.cfg }

// Log returns the logger.
func (e *Engine) Log() *logging.Logger { return e.log }

// ModelRegistry returns the multi-model registry.
func (e *Engine) ModelRegistry() *llm.Registry { return e.modelRegistry }

// LLMModel returns the configured LLM model name.
func (e *Engine) LLMModel() string { return e.llm.Model() }

// subAgentMaxRounds returns the configured max LLM calls per sub-agent.
func (e *Engine) subAgentMaxRounds() int {
	if e.cfg.Agents.MaxRounds > 0 {
		return e.cfg.Agents.MaxRounds
	}
	return 20
}

// ToolList returns all available tools grouped by name.
func (e *Engine) ToolList() []map[string]string {
	schemas := e.executors.AllToolSchemas()
	tools := make([]map[string]string, 0, len(schemas))
	for _, s := range schemas {
		tools = append(tools, map[string]string{
			"name":        s.Name,
			"description": s.Description,
		})
	}
	return tools
}

// ShieldStatus returns the current Shield operational state.
func (e *Engine) ShieldStatus() map[string]any {
	s := e.shield.Status()
	return map[string]any{
		"active":        s.Active,
		"tier2_used":    s.Tier2Used,
		"tier2_budget":  s.Tier2Budget,
		"tier2_enabled": s.Tier2Enabled,
	}
}

// SandboxStatus returns the current kernel sandbox state for the Agent process.
func (e *Engine) SandboxStatus() map[string]any {
	s := e.sandboxStatus
	return map[string]any{
		"active":     s.Active,
		"mode":       s.Mode,
		"version":    s.Version,
		"filesystem": s.Filesystem,
		"network":    s.Network,
		"reason":     s.Reason,
	}
}

// SetSandboxStatus stores the sandbox probe result for API reporting.
func (e *Engine) SetSandboxStatus(active bool, mode string, version int, filesystem, network bool, reason string) {
	e.sandboxStatus = sandboxInfo{
		Active:     active,
		Mode:       mode,
		Version:    version,
		Filesystem: filesystem,
		Network:    network,
		Reason:     reason,
	}
}

// OAuthManager returns the OAuth2 token manager, or nil if not configured.
func (e *Engine) OAuthManager() *oauth.Manager { return e.oauthManager }

// Tier3 returns the Tier 3 human-in-the-loop manager.
func (e *Engine) Tier3() *Tier3Manager { return e.tier3Manager }

// Broadcaster returns the event broadcaster for subscribing clients.
func (e *Engine) Broadcaster() *EventBroadcaster { return e.broadcaster }

// ConfigPath returns the path to the config.yaml file.
func (e *Engine) ConfigPath() string {
	return filepath.Join(e.cfg.Workspace, "config.yaml")
}

// SubAgentManager returns the sub-agent manager.
func (e *Engine) SubAgentManager() *SubAgentManager { return e.subAgentManager }

// SetupSubAgents creates the sub-agent manager and registers the executor.
// Called from internal_engine.go after the gRPC port is known.
func (e *Engine) SetupSubAgents(grpcAddr string) {
	e.subAgentManager = NewSubAgentManager(e, grpcAddr, 5)
	adapter := NewSubAgentManagerAdapter(e.subAgentManager)
	e.executors.RegisterSubAgents(adapter)
}

// MCPServerStatus returns the status of all configured MCP servers.
func (e *Engine) MCPServerStatus() []map[string]any {
	if e.mcpManager == nil {
		return nil
	}
	return e.mcpManager.ServerStatus()
}

// UpdateIdentity applies new agent identity settings in-memory.
func (e *Engine) UpdateIdentity(name, avatar string) {
	if name != "" {
		e.cfg.Identity.Name = name
	}
	if avatar != "" {
		e.cfg.Identity.Avatar = avatar
	}
}

// UpdateShieldBudget changes the daily Tier 2 budget in-memory.
func (e *Engine) UpdateShieldBudget(budget int) {
	e.cfg.General.DailyBudget = budget
	e.shield.UpdateBudget(budget)
}

// LogPath returns the path to the engine log file.
func (e *Engine) LogPath() string {
	return filepath.Join(e.cfg.Workspace, ".openparallax", "engine.log")
}

// Audit returns the audit logger for recording security events.
func (e *Engine) Audit() *audit.Logger { return e.audit }

// SetAgentAuthToken sets the ephemeral token the agent must present on connect.
func (e *Engine) SetAgentAuthToken(token string) {
	e.mu.Lock()
	e.agentAuthToken = token
	e.mu.Unlock()
}

// AuditPath returns the path to the audit JSONL file.
func (e *Engine) AuditPath() string {
	return filepath.Join(e.cfg.Workspace, ".openparallax", "audit.jsonl")
}

// --- Conversion helpers ---

// buildOAuthProviders creates OAuth provider configs from the agent config.
func buildOAuthProviders(cfg *types.AgentConfig) map[string]oauth.ProviderConfig {
	providers := make(map[string]oauth.ProviderConfig)
	if cfg.OAuth.Google != nil && cfg.OAuth.Google.ClientID != "" {
		providers["google"] = oauth.ProviderConfig{
			ClientID:     cfg.OAuth.Google.ClientID,
			ClientSecret: cfg.OAuth.Google.ClientSecret,
		}
	}
	if cfg.OAuth.Microsoft != nil && cfg.OAuth.Microsoft.ClientID != "" {
		providers["microsoft"] = oauth.ProviderConfig{
			ClientID:     cfg.OAuth.Microsoft.ClientID,
			ClientSecret: cfg.OAuth.Microsoft.ClientSecret,
			TenantID:     cfg.OAuth.Microsoft.TenantID,
		}
	}
	return providers
}

func readCanaryToken(workspace string) string {
	canaryPath := filepath.Join(workspace, ".openparallax", "canary.token")
	data, err := os.ReadFile(canaryPath)
	if err == nil && len(data) > 0 {
		return string(data)
	}
	token, _ := crypto.GenerateCanary()
	return token
}

func resolveFilePath(path, configDir, workspace string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	for _, base := range []string{configDir, workspace, "."} {
		candidate := filepath.Join(base, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join(configDir, path)
}
