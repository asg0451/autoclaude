package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.coldcutz.net/autoclaude/internal/claude"
	"go.coldcutz.net/autoclaude/internal/prompt"
	"go.coldcutz.net/autoclaude/internal/state"
	"go.coldcutz.net/autoclaude/internal/tmux"
)

var runMaxIterations int

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the coder-critic loop",
	Long: `Start the autoclaude coder-critic loop in a tmux session.

The loop proceeds as follows:
1. Coder: Works on TODOs from .autoclaude/TODO.md
2. Critic: Reviews changes, approves or requests fixes
3. Repeat until all TODOs complete or max iterations reached
4. Evaluator: Checks if overall goal is met

You can interact with Claude during execution (e.g., for permissions).
The stop hook orchestrates transitions between steps.`,
	RunE: runRun,
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().IntVarP(&runMaxIterations, "max-iterations", "m", 0, "Override max iterations (0 = use init value)")
}

func runRun(cmd *cobra.Command, args []string) error {
	// Check if claude is installed
	if err := claude.CheckInstalled(); err != nil {
		return err
	}

	// Check if initialized
	if !state.Exists() {
		return fmt.Errorf("autoclaude not initialized. Run 'autoclaude init' first")
	}

	// Load state
	s, err := state.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Override max iterations if specified
	if runMaxIterations > 0 {
		s.MaxIterations = runMaxIterations
	}

	// Reset to start of loop
	s.Step = state.StepCoder
	s.Iteration = 1
	s.RetryCount = 0
	s.LastError = ""

	if err := s.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Update status
	if err := s.UpdateStatus("Starting coder-critic loop..."); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	fmt.Println("Starting autoclaude loop...")
	fmt.Printf("  Goal: %s\n", s.Goal)
	fmt.Printf("  Test command: %s\n", s.TestCmd)
	fmt.Printf("  Max iterations: %d\n", s.MaxIterations)
	fmt.Println()
	fmt.Println("Starting coder in tmux session...")
	fmt.Println("(You can interact with Claude for permissions)")
	fmt.Println()

	// Load coder prompt
	coderPrompt, err := prompt.LoadCoder()
	if err != nil {
		return fmt.Errorf("failed to load coder prompt: %w", err)
	}

	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Build and run command
	claudeCmd := claude.BuildCommand(coderPrompt)
	if err := tmux.RunAndAttach(wd, claudeCmd); err != nil {
		return fmt.Errorf("failed to run coder: %w", err)
	}

	return nil
}
