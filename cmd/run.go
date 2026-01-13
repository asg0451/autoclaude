package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.coldcutz.net/autoclaude/internal/claude"
	"go.coldcutz.net/autoclaude/internal/config"
	"go.coldcutz.net/autoclaude/internal/prompt"
	"go.coldcutz.net/autoclaude/internal/state"
)

const maxFixRetries = 3

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the coder-critic loop",
	Long: `Start the autoclaude coder-critic loop.

The loop proceeds as follows:
1. Coder: Works on highest priority TODO
2. Critic: Reviews changes
   - APPROVED or MINOR_ISSUES: Move to next TODO
   - NEEDS_FIXES: Coder retries (up to 3 times)
3. Repeat until all TODOs complete
4. Evaluator: Final check that goal is met

Each phase runs in a fresh Claude session to maintain quality.`,
	RunE: runRun,
}

func init() {
	rootCmd.AddCommand(runCmd)
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

	// Reset state for new run
	s.Step = state.StepCoder
	s.Iteration = 0
	s.RetryCount = 0
	s.LastError = ""
	if err := s.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	autoclaudePath, err := GetExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get autoclaude path: %w", err)
	}

	// Build prompt params for generating fix prompts
	params := prompt.PromptParams{
		Goal:    s.Goal,
		TestCmd: s.TestCmd,
	}

	fmt.Println("Starting autoclaude loop...")
	fmt.Printf("  Goal: %s\n", s.Goal)
	fmt.Printf("  Test command: %s\n", s.TestCmd)
	fmt.Println()

	// Enable stop hook for all phases (kills Claude when it stops to return control)
	if err := config.SetupStopHook(autoclaudePath); err != nil {
		return fmt.Errorf("failed to setup stop hook: %w", err)
	}
	defer config.RemoveStopHook(autoclaudePath)

	// Outer loop: process TODOs until all complete (no limit)
	for hasIncompleteTodos() {
		s.Iteration++
		s.RetryCount = 0
		s.Save()

		// Get next TODO and save it as current (before coder checks it off)
		currentTodo := state.GetNextTodo()
		state.SetCurrentTodo(currentTodo)

		// === CODER PHASE ===
		fmt.Printf("\n=== TODO %d: CODER ===\n", s.Iteration)
		fmt.Printf("  Working on: %s\n", currentTodo)
		s.Step = state.StepCoder
		s.Save()
		s.UpdateStatus(fmt.Sprintf("Working on: %s", currentTodo))

		coderPrompt, _ := prompt.LoadCoder()
		promptPath, _ := prompt.WriteCurrentPrompt(coderPrompt)
		if err := runClaudePhase(promptPath); err != nil {
			return fmt.Errorf("coder phase failed: %w", err)
		}

		// Inner loop: critic review with fix retries (max 3)
		for retry := 0; retry < maxFixRetries; retry++ {
			// === CRITIC PHASE ===
			fmt.Printf("=== TODO %d: CRITIC (attempt %d/%d) ===\n", s.Iteration, retry+1, maxFixRetries)
			s.Step = state.StepCritic
			s.RetryCount = retry
			s.Save()
			s.UpdateStatus("Running critic review...")

			state.ClearCriticVerdict()

			criticPrompt, _ := prompt.LoadCritic()
			promptPath, _ = prompt.WriteCurrentPrompt(criticPrompt)
			if err := runClaudePhase(promptPath); err != nil {
				return fmt.Errorf("critic phase failed: %w", err)
			}

			verdict, content := state.GetCriticVerdict()

			switch verdict {
			case state.VerdictApproved:
				fmt.Println("  ✓ Critic: APPROVED")
				goto nextTodo

			case state.VerdictMinorIssues:
				fmt.Println("  ✓ Critic: MINOR_ISSUES (added to TODOs for later)")
				goto nextTodo

			case state.VerdictNeedsFixes:
				fmt.Printf("  ✗ Critic: NEEDS_FIXES (retry %d/%d)\n", retry+1, maxFixRetries)
				if retry < maxFixRetries-1 {
					// Run fixer with critic's feedback
					fmt.Println("=== FIXER ===")
					s.Step = state.StepCoder
					s.Save()
					s.UpdateStatus(fmt.Sprintf("Fixing: %s", currentTodo))

					// Use state.GetCurrentTodo() to read from file (robust across restarts)
					fixerPrompt := prompt.GenerateFixer(params, content, state.GetCurrentTodo())
					promptPath, _ = prompt.WriteCurrentPrompt(fixerPrompt)
					if err := runClaudePhase(promptPath); err != nil {
						return fmt.Errorf("fixer phase failed: %w", err)
					}
				}

			default:
				fmt.Println("  ? Critic: No clear verdict, assuming needs review")
				if retry < maxFixRetries-1 {
					continue
				}
			}
		}

		// Exhausted retries
		fmt.Printf("  ⚠ Max retries (%d) reached for TODO %d, moving on\n", maxFixRetries, s.Iteration)

	nextTodo:
		state.ClearCurrentTodo()
	}

	// === EVALUATOR PHASE ===
	fmt.Println("\n=== EVALUATOR ===")
	s.Step = state.StepEvaluator
	s.Save()
	s.UpdateStatus("Running evaluator...")

	evalPrompt, _ := prompt.LoadEvaluator()
	promptPath, _ := prompt.WriteCurrentPrompt(evalPrompt)
	if err := runClaudePhase(promptPath); err != nil {
		return fmt.Errorf("evaluator phase failed: %w", err)
	}

	// Check if evaluator added more TODOs
	if hasIncompleteTodos() {
		fmt.Println("Evaluator added more TODOs. Continuing...")
		return runRun(cmd, args) // Recursive call to process new TODOs
	}

	s.Step = state.StepDone
	s.Save()
	s.UpdateStatus("Complete!")
	fmt.Println("\n=== COMPLETE ===")

	return nil
}

// runClaudePhase runs a Claude session in the foreground and waits for it to complete
func runClaudePhase(promptFile string) error {
	return claude.RunInteractiveWithPromptFile(promptFile, "acceptEdits")
}

// hasIncompleteTodos checks if there are incomplete TODOs
func hasIncompleteTodos() bool {
	data, err := os.ReadFile(state.TodoPath())
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "- [ ]")
}
