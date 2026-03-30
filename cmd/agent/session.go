package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/spf13/cobra"
)

var sessionConfigPath string

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage sessions",
}

var sessionDeleteCmd = &cobra.Command{
	Use:          "delete <id>",
	Short:        "Delete a session and its messages",
	SilenceUsage: true,
	Args:         cobra.MaximumNArgs(1),
	RunE:         runSessionDelete,
}

var sessionDeleteAll bool

func init() {
	sessionCmd.PersistentFlags().StringVarP(&sessionConfigPath, "config", "c", "", "path to config.yaml")
	sessionDeleteCmd.Flags().BoolVar(&sessionDeleteAll, "all", false, "delete all sessions")
	sessionCmd.AddCommand(sessionDeleteCmd)
	rootCmd.AddCommand(sessionCmd)
}

func runSessionDelete(_ *cobra.Command, args []string) error {
	cfgPath := sessionConfigPath
	if cfgPath == "" {
		cfgPath = findConfig()
	}
	if cfgPath == "" {
		return fmt.Errorf("workspace not found: use --config")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	dbPath := filepath.Join(cfg.Workspace, ".openparallax", "openparallax.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	if sessionDeleteAll {
		fmt.Print("Delete ALL sessions? This cannot be undone. (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(answer)) != "y" {
			fmt.Println("Cancelled.")
			return nil
		}

		sessions, err := db.ListSessions()
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}
		for _, s := range sessions {
			_ = db.DeleteSession(s.ID)
		}
		fmt.Printf("Deleted %d sessions.\n", len(sessions))
		return nil
	}

	if len(args) == 0 {
		return fmt.Errorf("specify a session ID, or use --all")
	}

	id := args[0]
	sess, err := db.GetSession(id)
	if err != nil {
		return fmt.Errorf("session not found: %s", id)
	}

	if err := db.DeleteSession(id); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	title := sess.Title
	if title == "" {
		title = "Untitled"
	}
	fmt.Printf("Deleted session %q (%s)\n", title, id)
	return nil
}
