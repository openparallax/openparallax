package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:          "mcp",
	Short:        "Manage MCP servers (install, remove, list)",
	SilenceUsage: true,
}

var mcpListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List installed MCP server binaries",
	SilenceUsage: true,
	RunE:         runMCPList,
}

var mcpInstallCmd = &cobra.Command{
	Use:   "install <name>",
	Short: "Install an MCP server from the official repo",
	Long: `Download an MCP server binary to ~/.openparallax/mcp/<name>/.

The binary is fetched from GitHub releases of the openparallax/mcp repo.
After installation, add the server to your config.yaml under mcp.servers.

Example:
  openparallax mcp install rss`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runMCPInstall,
}

var mcpRemoveCmd = &cobra.Command{
	Use:          "remove <name>",
	Short:        "Remove an installed MCP server",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runMCPRemove,
}

func init() {
	mcpCmd.AddCommand(mcpListCmd)
	mcpCmd.AddCommand(mcpInstallCmd)
	mcpCmd.AddCommand(mcpRemoveCmd)
	rootCmd.AddCommand(mcpCmd)
}

func globalMCPDir() (string, error) {
	opHome, err := openparallaxHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(opHome, "mcp"), nil
}

func runMCPList(_ *cobra.Command, _ []string) error {
	dir, err := globalMCPDir()
	if err != nil {
		return err
	}

	entries, readErr := os.ReadDir(dir)
	if readErr != nil || len(entries) == 0 {
		fmt.Println("No MCP servers installed.")
		fmt.Printf("\nInstall directory: %s\n", dir)
		fmt.Println("Install with: openparallax mcp install <name>")
		return nil
	}

	fmt.Printf("Installed MCP servers (%s):\n", dir)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		binaryPath := mcpBinaryPath(dir, name)
		if _, statErr := os.Stat(binaryPath); statErr != nil {
			continue
		}
		info, _ := os.Stat(binaryPath)
		fmt.Printf("  %-20s %s\n", name, formatBytes(info.Size()))
	}
	return nil
}

func runMCPInstall(_ *cobra.Command, args []string) error {
	name := args[0]

	dir, err := globalMCPDir()
	if err != nil {
		return err
	}

	destDir := filepath.Join(dir, name)
	binaryPath := mcpBinaryPath(dir, name)

	if _, statErr := os.Stat(binaryPath); statErr == nil {
		return fmt.Errorf("MCP server %q already installed at %s", name, binaryPath)
	}

	if mkErr := os.MkdirAll(destDir, 0o755); mkErr != nil {
		return fmt.Errorf("create MCP directory: %w", mkErr)
	}

	// Download from openparallax/mcp releases.
	binaryName := mcpBinaryName(name)
	url := fmt.Sprintf(
		"https://github.com/openparallax/mcp/releases/latest/download/%s",
		binaryName,
	)

	fmt.Printf("  Downloading MCP server %q...\n", name)
	if dlErr := fetchFile(url, binaryPath, 5*time.Minute, true); dlErr != nil {
		_ = os.RemoveAll(destDir)
		return fmt.Errorf("download failed: %w", dlErr)
	}

	if runtime.GOOS != "windows" {
		_ = os.Chmod(binaryPath, 0o755)
	}

	info, _ := os.Stat(binaryPath)
	fmt.Printf("  ✓ Installed %s (%s)\n", name, formatBytes(info.Size()))
	fmt.Printf("\n  Add to your config.yaml:\n")
	fmt.Printf("    mcp:\n")
	fmt.Printf("      servers:\n")
	fmt.Printf("        - name: %s\n", name)
	fmt.Printf("          command: %s\n", binaryPath)
	return nil
}

func runMCPRemove(_ *cobra.Command, args []string) error {
	name := args[0]

	dir, err := globalMCPDir()
	if err != nil {
		return err
	}

	serverDir := filepath.Join(dir, name)
	if _, statErr := os.Stat(serverDir); statErr != nil {
		return fmt.Errorf("MCP server %q is not installed", name)
	}

	if rmErr := os.RemoveAll(serverDir); rmErr != nil {
		return fmt.Errorf("remove MCP server: %w", rmErr)
	}

	fmt.Printf("  ✓ Removed MCP server %q\n", name)
	fmt.Println("  Remember to remove the entry from your config.yaml.")
	return nil
}

func mcpBinaryPath(dir, name string) string {
	return filepath.Join(dir, name, mcpBinaryName(name))
}

func mcpBinaryName(name string) string {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	arch := runtime.GOARCH
	return fmt.Sprintf("openparallax-mcp-%s-%s-%s%s", name, runtime.GOOS, arch, suffix)
}
