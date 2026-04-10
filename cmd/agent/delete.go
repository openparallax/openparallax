package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/openparallax/openparallax/internal/registry"
	"github.com/spf13/cobra"
)

var (
	deleteYes bool
)

var deleteCmd = &cobra.Command{
	Use:          "delete <name>",
	Short:        "Delete an agent and its workspace",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runDelete,
}

func init() {
	deleteCmd.Flags().BoolVarP(&deleteYes, "yes", "y", false, "skip confirmation prompt")
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(_ *cobra.Command, args []string) error {
	name := args[0]

	regPath, err := registry.DefaultPath()
	if err != nil {
		return err
	}
	reg, err := registry.Load(regPath)
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	rec, ok := reg.Lookup(name)
	if !ok {
		return fmt.Errorf("agent %q not found", name)
	}

	if registry.IsRunning(rec.Workspace) {
		return fmt.Errorf("agent %q is running — stop it first: openparallax stop %s", rec.Name, rec.Slug)
	}

	if !deleteYes {
		fmt.Printf("This will permanently delete %s and all its data (sessions, memory, config).\n", rec.Name)
		fmt.Printf("Type '%s' to confirm: ", rec.Slug)
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return fmt.Errorf("cancelled")
		}
		input := strings.TrimSpace(scanner.Text())
		if !strings.EqualFold(input, rec.Slug) {
			return fmt.Errorf("confirmation failed — expected %q, got %q", rec.Slug, input)
		}
	}

	// Remove workspace directory.
	if err := os.RemoveAll(rec.Workspace); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not remove workspace %s: %s\n", rec.Workspace, err)
	}

	// Remove from registry.
	if err := reg.Remove(rec.Slug); err != nil {
		return fmt.Errorf("remove from registry: %w", err)
	}

	fmt.Printf("Agent %s deleted.\n", rec.Name)
	return nil
}
