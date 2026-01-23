package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.coldcutz.net/autoclaude/internal/claude"
	"go.coldcutz.net/autoclaude/internal/prompt"
	"go.coldcutz.net/autoclaude/internal/state"
)

var (
	pruneDryRun    bool
	pruneAggressive bool
	pruneVerbose   bool
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Clean up and organize the TODO list",
	Long: `Prune the TODO list by grouping similar items, auto-completing stale issues,
and deduplicating semantically similar items.

This helps maintain TODO hygiene during long-running autoclaude sessions.

Operations:
- Group similar low-priority items into single TODOs
- Auto-complete stale minor issues that are no longer relevant
- Deduplicate semantically similar items`,
	RunE: runPrune,
}

func init() {
	rootCmd.AddCommand(pruneCmd)
	pruneCmd.Flags().BoolVar(&pruneDryRun, "dry-run", false, "Show proposed changes without applying them")
	pruneCmd.Flags().BoolVar(&pruneAggressive, "aggressive", false, "Enable more aggressive pruning")
	pruneCmd.Flags().BoolVar(&pruneVerbose, "verbose", false, "Show detailed output")
}

func runPrune(cmd *cobra.Command, args []string) error {
	// Check if claude is installed
	if err := claude.CheckInstalled(); err != nil {
		return err
	}

	// Check if initialized
	if !state.Exists() {
		return fmt.Errorf("autoclaude not initialized. Run 'autoclaude init' first")
	}

	// Load state to get goal and test command
	s, err := state.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Check if TODO.md exists
	if _, err := os.ReadFile(state.TodoPath()); err != nil {
		return fmt.Errorf("failed to read TODO.md: %w", err)
	}

	// Build prompt params
	params := prompt.PromptParams{
		Goal:    s.Goal,
		TestCmd: s.TestCmd,
	}
	if pruneAggressive {
		params.PrunerMode = "aggressive"
	}

	// Generate pruner prompt
	prunerPrompt := prompt.GeneratePruner(params)

	if pruneDryRun {
		fmt.Println("=== Prune Dry Run ===")
		fmt.Println("This would prune the TODO list with the following settings:")
		fmt.Printf("  Mode: %s\n", map[bool]string{true: "aggressive", false: "normal"}[pruneAggressive])
		fmt.Println("\nPrompt that would be used:")
		fmt.Println("---")
		fmt.Println(prunerPrompt)
		fmt.Println("---")
		fmt.Println("\nTo actually prune, run 'autoclaude prune' without --dry-run")
		return nil
	}

	if pruneVerbose {
		fmt.Println("=== Pruning TODO List ===")
		fmt.Printf("  Mode: %s\n", map[bool]string{true: "aggressive", false: "normal"}[pruneAggressive])
		fmt.Println("  Analyzing TODOs for grouping, completion, and deduplication...")
		fmt.Println()
	}

	// Write prompt to file and run Claude
	promptPath, err := prompt.WriteCurrentPrompt(prunerPrompt)
	if err != nil {
		return fmt.Errorf("failed to write pruner prompt: %w", err)
	}

	// Run Claude in acceptEdits mode
	if err := claude.RunInteractiveWithPromptFile(promptPath, "acceptEdits", ""); err != nil {
		return fmt.Errorf("pruner phase failed: %w", err)
	}

	if pruneVerbose {
		fmt.Println("\n=== Pruning Complete ===")
		fmt.Println("Check TODO.md for the pruning summary at the end of the file.")
	}

	// Update state with last prune time
	s.LastPruneAt = 0 // Will be set to current time by the caller for inline pruning
	s.TodosSincePrune = 0
	if err := s.Save(); err != nil {
		fmt.Printf("Warning: failed to save state: %v\n", err)
	}

	return nil
}

// runInlinePruner runs the pruner programmatically (not via CLI command)
// Used by the main run loop for periodic pruning
func runInlinePruner(s *state.State, aggressive bool) error {
	fmt.Println("\n=== Running Periodic TODO Pruning ===")

	// Build prompt params
	params := prompt.PromptParams{
		Goal:    s.Goal,
		TestCmd: s.TestCmd,
	}
	if aggressive {
		params.PrunerMode = "aggressive"
	}

	// Generate pruner prompt
	prunerPrompt := prompt.GeneratePruner(params)

	// Write prompt to file
	promptPath, err := prompt.WriteCurrentPrompt(prunerPrompt)
	if err != nil {
		return fmt.Errorf("failed to write pruner prompt: %w", err)
	}

	// Run Claude in acceptEdits mode
	if err := claude.RunInteractiveWithPromptFile(promptPath, "acceptEdits", ""); err != nil {
		return fmt.Errorf("pruner phase failed: %w", err)
	}

	fmt.Println("  ✓ Pruning complete")
	return nil
}
