package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/memory"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/spf13/cobra"
)

var memoryConfigPath string

var memoryCmd = &cobra.Command{
	Use:          "memory",
	Short:        "View and search workspace memory",
	SilenceUsage: true,
	RunE:         runMemoryList,
}

var memoryShowCmd = &cobra.Command{
	Use:          "show [file]",
	Short:        "Print a memory file's content",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runMemoryShow,
}

var memorySearchCmd = &cobra.Command{
	Use:          "search [query]",
	Short:        "Full-text search across memory",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runMemorySearch,
}

func init() {
	memoryCmd.Flags().StringVarP(&memoryConfigPath, "config", "c", "", "path to config.yaml")
	memoryCmd.AddCommand(memoryShowCmd)
	memoryCmd.AddCommand(memorySearchCmd)
	rootCmd.AddCommand(memoryCmd)
}

func openMemory(cfgPath string) (*memory.Manager, *types.AgentConfig, error) {
	if cfgPath == "" {
		resolved, err := resolveConfig(nil, "")
		if err != nil {
			return nil, nil, err
		}
		cfgPath = resolved
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, err
	}
	db, err := storage.Open(filepath.Join(cfg.Workspace, ".openparallax", "openparallax.db"))
	if err != nil {
		return nil, nil, err
	}
	mgr := memory.NewManager(cfg.Workspace, db, nil)
	return mgr, cfg, nil
}

func runMemoryList(cmd *cobra.Command, args []string) error {
	_, cfg, err := openMemory(memoryConfigPath)
	if err != nil {
		return err
	}

	fmt.Println("Memory files:")
	for _, ft := range types.AllMemoryFiles {
		path := filepath.Join(cfg.Workspace, string(ft))
		info, statErr := os.Stat(path)
		if statErr != nil {
			fmt.Printf("  %-16s  (not found)\n", ft)
		} else {
			fmt.Printf("  %-16s  %d bytes\n", ft, info.Size())
		}
	}
	return nil
}

func runMemoryShow(cmd *cobra.Command, args []string) error {
	mgr, _, err := openMemory(memoryConfigPath)
	if err != nil {
		return err
	}

	content, readErr := mgr.Read(types.MemoryFileType(args[0]))
	if readErr != nil {
		return fmt.Errorf("file not found: %s", args[0])
	}

	fmt.Print(content)
	return nil
}

func runMemorySearch(cmd *cobra.Command, args []string) error {
	mgr, _, err := openMemory(memoryConfigPath)
	if err != nil {
		return err
	}

	results, searchErr := mgr.Search(args[0], 20)
	if searchErr != nil {
		return searchErr
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	for _, r := range results {
		fmt.Printf("[%s] %s\n  %s\n\n", r.Path, r.Section, r.Snippet)
	}
	return nil
}
