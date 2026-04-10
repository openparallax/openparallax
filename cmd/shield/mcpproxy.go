package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/openparallax/openparallax/audit"
	"github.com/openparallax/openparallax/mcp"
	"github.com/openparallax/openparallax/shield"
	"github.com/spf13/cobra"
)

var mcpProxyCmd = &cobra.Command{
	Use:   "mcp-proxy",
	Short: "Start Shield as an MCP security gateway proxy",
	Long: `Start Shield as an MCP proxy that sits between MCP clients and upstream
MCP servers. Every tool call is evaluated through the Shield 3-tier pipeline
before being forwarded to the upstream server.`,
	RunE: runMCPProxy,
}

var mcpProxyConfig string

func init() {
	mcpProxyCmd.Flags().StringVarP(&mcpProxyConfig, "config", "c", "shield.yaml", "Path to shield.yaml config file")
	rootCmd.AddCommand(mcpProxyCmd)
}

func runMCPProxy(_ *cobra.Command, _ []string) error {
	cfg, err := loadShieldConfig(mcpProxyConfig)
	if err != nil {
		return err
	}

	if len(cfg.MCP.Servers) == 0 {
		return fmt.Errorf("no MCP servers configured in %s", mcpProxyConfig)
	}

	pipeline, err := shield.NewPipeline(cfg.toPipelineConfig())
	if err != nil {
		return fmt.Errorf("pipeline init: %w", err)
	}

	var auditLog *audit.Logger
	if cfg.Audit.File != "" {
		auditLog, err = audit.NewLogger(cfg.Audit.File)
		if err != nil {
			return fmt.Errorf("audit init: %w", err)
		}
		defer func() { _ = auditLog.Close() }()
	}

	mcpManager := mcp.NewManager(cfg.MCP.Servers, nil)
	defer mcpManager.ShutdownAll()

	toolMapping := make(map[string]shield.ActionType)
	for k, v := range cfg.MCP.ToolMapping {
		toolMapping[k] = shield.ActionType(v)
	}

	gateway := shield.NewMCPGateway(shield.MCPGatewayConfig{
		Pipeline:    pipeline,
		MCPManager:  mcpManager,
		ToolMapping: toolMapping,
		AuditFunc:   makeAuditFunc(auditLog),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mcpSrv, err := gateway.DiscoverAndServe(ctx)
	if err != nil {
		return fmt.Errorf("gateway init: %w", err)
	}

	httpServer := mcpserver.NewStreamableHTTPServer(mcpSrv)

	fmt.Fprintf(os.Stderr, "Shield MCP proxy listening on %s/mcp\n", cfg.Listen)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigCh
		cancel()
		gateway.Shutdown()
	}()

	if listenErr := httpServer.Start(cfg.Listen); listenErr != nil {
		return listenErr
	}
	return nil
}

func makeAuditFunc(auditLog *audit.Logger) func(entry shield.MCPAuditEntry) {
	if auditLog == nil {
		return nil
	}
	return func(entry shield.MCPAuditEntry) {
		details, _ := json.Marshal(entry)
		_ = auditLog.Log(audit.Entry{
			EventType:  audit.ActionEvaluated,
			ActionType: entry.ActionType,
			Details:    string(details),
			Source:     "mcp-proxy",
		})
	}
}
