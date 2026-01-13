package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.coldcutz.net/autoclaude/internal/claude"
	"go.coldcutz.net/autoclaude/internal/prompt"
	"go.coldcutz.net/autoclaude/internal/state"
)

// claude is used for CheckInstalled

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume from saved state",
	Long: `Resume the autoclaude loop from the last saved state.

Use this after:
- An error or interruption
- Manual intervention
- To continue after reviewing changes`,
	RunE: runResume,
}

func init() {
	rootCmd.AddCommand(resumeCmd)
}

func runResume(cmd *cobra.Command, args []string) error {
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

	// Check if already done
	if s.Step == state.StepDone {
		fmt.Println("Previous run completed. Use 'autoclaude run' to start a new run.")
		return nil
	}

	// Clear retry count for fresh start
	s.RetryCount = 0
	if err := s.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Println("Resuming autoclaude loop...")
	fmt.Printf("  Current step: %s\n", s.Step)
	fmt.Printf("  Iteration: %d/%d\n", s.Iteration, s.MaxIterations)
	fmt.Println()

	// Get appropriate prompt based on current step
	var promptContent string
	switch s.Step {
	case state.StepCoder:
		promptContent, err = prompt.LoadCoder()
		if err != nil {
			return fmt.Errorf("failed to load coder prompt: %w", err)
		}
		s.UpdateStatus(fmt.Sprintf("Resuming coder (iteration %d)...", s.Iteration))

	case state.StepCritic:
		promptContent, err = prompt.LoadCritic()
		if err != nil {
			return fmt.Errorf("failed to load critic prompt: %w", err)
		}
		s.UpdateStatus("Resuming critic review...")

	case state.StepEvaluator:
		promptContent, err = prompt.LoadEvaluator()
		if err != nil {
			return fmt.Errorf("failed to load evaluator prompt: %w", err)
		}
		s.UpdateStatus("Resuming evaluator...")

	default:
		return fmt.Errorf("unexpected state: %s", s.Step)
	}

	// Write prompt to file
	promptPath, err := prompt.WriteCurrentPrompt(promptContent)
	if err != nil {
		return fmt.Errorf("failed to write current prompt: %w", err)
	}

	// Run Claude directly in foreground
	if err := claude.RunInteractiveWithPromptFile(promptPath, "acceptEdits"); err != nil {
		return fmt.Errorf("failed to resume: %w", err)
	}

	return nil
}
