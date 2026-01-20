package cmd

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
	"go.coldcutz.net/autoclaude/internal/claude"
	"go.coldcutz.net/autoclaude/internal/config"
	"go.coldcutz.net/autoclaude/internal/prompt"
	"go.coldcutz.net/autoclaude/internal/state"
)

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

	// Initialize stats if not present
	if s.Stats == nil {
		s.Stats = &state.Stats{}
	}

	// Clean up working directory
	if hasUncommittedChanges() {
		fmt.Println("Cleaning up uncommitted changes...")
		if err := cleanWorkingDirectory(); err != nil {
			return fmt.Errorf("failed to clean working directory: %w", err)
		}
	}

	fmt.Println("Resuming autoclaude loop...")
	fmt.Printf("  Current step: %s\n", s.Step)
	fmt.Printf("  TODO iteration: %d\n", s.Iteration)
	if currentTodo := state.GetCurrentTodo(); currentTodo != "(unknown)" {
		fmt.Printf("  Current TODO: %s\n", currentTodo)
	}
	fmt.Println()

	autoclaudePath, err := GetExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get autoclaude path: %w", err)
	}

	// Build prompt params
	params := prompt.PromptParams{
		Goal:    s.Goal,
		TestCmd: s.TestCmd,
	}

	// Enable stop hook
	if err := config.SetupStopHook(autoclaudePath); err != nil {
		return fmt.Errorf("failed to setup stop hook: %w", err)
	}
	defer config.RemoveStopHook(autoclaudePath)

	// Resume from current step - run ONE phase then continue into main loop
	switch s.Step {
	case state.StepCoder:
		// Re-run coder for current TODO
		fmt.Printf("=== RESUMING CODER ===\n")
		s.UpdateStatus(fmt.Sprintf("Resuming: %s", state.GetCurrentTodo()))

		commitBefore := getCommitHash()
		coderPrompt, _ := prompt.LoadCoder()
		promptPath, _ := prompt.WriteCurrentPrompt(coderPrompt)
		if err := runClaudePhase(promptPath, s.Stats); err != nil {
			return fmt.Errorf("coder phase failed: %w", err)
		}
		checkCommitCreated(commitBefore, "Coder")

	case state.StepCritic:
		// Run critic for current TODO, then continue
		fmt.Printf("=== RESUMING CRITIC ===\n")
		s.UpdateStatus("Resuming critic review...")

		state.ClearCriticVerdict()
		criticPrompt, _ := prompt.LoadCritic()
		promptPath, _ := prompt.WriteCurrentPrompt(criticPrompt)
		if err := runClaudePhase(promptPath, s.Stats); err != nil {
			return fmt.Errorf("critic phase failed: %w", err)
		}

		verdict, content := state.GetCriticVerdict()
		if verdict == state.VerdictNeedsFixes {
			// Run fixer
			fmt.Println("=== FIXER ===")
			s.Step = state.StepCoder
			s.Save()
			s.Stats.FixAttempts++

			fixerCommitBefore := getCommitHash()
			fixerPrompt := prompt.GenerateFixer(params, content, state.GetCurrentTodo())
			promptPath, _ = prompt.WriteCurrentPrompt(fixerPrompt)
			if err := runClaudePhase(promptPath, s.Stats); err != nil {
				return fmt.Errorf("fixer phase failed: %w", err)
			}
			checkCommitCreated(fixerCommitBefore, "Fixer")
		} else if verdict == state.VerdictApproved || verdict == state.VerdictMinorIssues {
			s.Stats.TodosCompleted++
			state.ClearCurrentTodo()
		}

	case state.StepEvaluator:
		// Run evaluator
		fmt.Println("=== RESUMING EVALUATOR ===")
		s.UpdateStatus("Resuming evaluator...")

		// Clean up any stale evaluation_complete file
		config.RemoveEvaluationComplete()

		// Set up evaluator stop hook
		if err := config.SetupEvaluatorStopHook(autoclaudePath); err != nil {
			return fmt.Errorf("failed to setup evaluator stop hook: %w", err)
		}

		evalPrompt, _ := prompt.LoadEvaluator()
		promptPath, _ := prompt.WriteCurrentPrompt(evalPrompt)
		if err := runClaudePhase(promptPath, s.Stats); err != nil {
			config.RemoveEvaluatorStopHook(autoclaudePath)
			config.RemoveEvaluationComplete()
			return fmt.Errorf("evaluator phase failed: %w", err)
		}

		// Clean up
		config.RemoveEvaluatorStopHook(autoclaudePath)
		config.RemoveEvaluationComplete()
	}

	// Now continue with the normal run loop for remaining TODOs
	return continueRunLoop(s, params, autoclaudePath)
}

// continueRunLoop continues the main loop after resuming
func continueRunLoop(s *state.State, params prompt.PromptParams, autoclaudePath string) error {
	// Process remaining TODOs
	for hasIncompleteTodos() {
		s.Iteration++
		s.RetryCount = 0
		s.Save()

		currentTodo := state.GetNextTodo()
		state.SetCurrentTodo(currentTodo)
		s.Stats.TodosAttempted++

		// === CODER PHASE ===
		fmt.Printf("\n=== TODO %d: CODER ===\n", s.Iteration)
		fmt.Printf("  Working on: %s\n", currentTodo)
		s.Step = state.StepCoder
		s.Save()
		s.UpdateStatus(fmt.Sprintf("Working on: %s", currentTodo))

		commitBefore := getCommitHash()
		coderPrompt, _ := prompt.LoadCoder()
		promptPath, _ := prompt.WriteCurrentPrompt(coderPrompt)
		if err := runClaudePhase(promptPath, s.Stats); err != nil {
			return fmt.Errorf("coder phase failed: %w", err)
		}
		checkCommitCreated(commitBefore, "Coder")

		// Inner loop: critic review with fix retries
		wasApproved := false
		for retry := 0; retry < maxFixRetries; retry++ {
			fmt.Printf("=== TODO %d: CRITIC (attempt %d/%d) ===\n", s.Iteration, retry+1, maxFixRetries)
			s.Step = state.StepCritic
			s.RetryCount = retry
			s.Save()
			s.UpdateStatus("Running critic review...")

			state.ClearCriticVerdict()

			criticPrompt, _ := prompt.LoadCritic()
			promptPath, _ = prompt.WriteCurrentPrompt(criticPrompt)
			if err := runClaudePhase(promptPath, s.Stats); err != nil {
				return fmt.Errorf("critic phase failed: %w", err)
			}

			verdict, content := state.GetCriticVerdict()

			switch verdict {
			case state.VerdictApproved:
				fmt.Println("  ✓ Critic: APPROVED")
				s.Stats.CriticApprovals++
				s.Stats.TodosCompleted++
				if retry > 0 {
					s.Stats.FixSuccesses++
				}
				wasApproved = true
				goto nextTodo

			case state.VerdictMinorIssues:
				fmt.Println("  ✓ Critic: MINOR_ISSUES (added to TODOs for later)")
				s.Stats.CriticMinor++
				s.Stats.TodosCompleted++
				if retry > 0 {
					s.Stats.FixSuccesses++
				}
				wasApproved = true
				goto nextTodo

			case state.VerdictNeedsFixes:
				fmt.Printf("  ✗ Critic: NEEDS_FIXES (retry %d/%d)\n", retry+1, maxFixRetries)
				s.Stats.CriticRejections++
				if retry < maxFixRetries-1 {
					fmt.Println("=== FIXER ===")
					s.Step = state.StepCoder
					s.Save()
					s.UpdateStatus(fmt.Sprintf("Fixing: %s", currentTodo))
					s.Stats.FixAttempts++

					fixerCommitBefore := getCommitHash()
					fixerPrompt := prompt.GenerateFixer(params, content, state.GetCurrentTodo())
					promptPath, _ = prompt.WriteCurrentPrompt(fixerPrompt)
					if err := runClaudePhase(promptPath, s.Stats); err != nil {
						return fmt.Errorf("fixer phase failed: %w", err)
					}
					checkCommitCreated(fixerCommitBefore, "Fixer")
				}

			default:
				fmt.Println("  ? Critic: No clear verdict, assuming needs review")
				if retry < maxFixRetries-1 {
					continue
				}
			}
		}

		if !wasApproved {
			fmt.Printf("  ⚠ Max retries (%d) reached for TODO %d, moving on\n", maxFixRetries, s.Iteration)
		}

	nextTodo:
		state.ClearCurrentTodo()
		s.Save()
	}

	// === EVALUATOR PHASE ===
	fmt.Println("\n=== EVALUATOR ===")
	s.Step = state.StepEvaluator
	s.Save()
	s.UpdateStatus("Running evaluator...")

	// Clean up any stale evaluation_complete file
	config.RemoveEvaluationComplete()

	// Set up evaluator stop hook (only kills Claude when evaluation_complete file exists)
	if err := config.SetupEvaluatorStopHook(autoclaudePath); err != nil {
		return fmt.Errorf("failed to setup evaluator stop hook: %w", err)
	}

	evalPrompt, _ := prompt.LoadEvaluator()
	promptPath, _ := prompt.WriteCurrentPrompt(evalPrompt)
	if err := runClaudePhase(promptPath, s.Stats); err != nil {
		config.RemoveEvaluatorStopHook(autoclaudePath)
		config.RemoveEvaluationComplete()
		return fmt.Errorf("evaluator phase failed: %w", err)
	}

	// Clean up hook and marker
	config.RemoveEvaluatorStopHook(autoclaudePath)
	config.RemoveEvaluationComplete()

	// Check if evaluator added more TODOs (user requested more work)
	if hasIncompleteTodos() {
		fmt.Println("User requested more work. Continuing...")
		return continueRunLoop(s, params, autoclaudePath)
	}

	s.Step = state.StepDone
	s.Save()
	s.UpdateStatus("Complete!")
	fmt.Println("\n=== COMPLETE ===")

	printStats(s.Stats)

	return nil
}

// hasUncommittedChanges checks if there are uncommitted changes in the working directory
func hasUncommittedChanges() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(output) > 0
}

// cleanWorkingDirectory resets the working directory to the last commit
func cleanWorkingDirectory() error {
	// Reset tracked files
	resetCmd := exec.Command("git", "checkout", ".")
	if err := resetCmd.Run(); err != nil {
		return fmt.Errorf("git checkout failed: %w", err)
	}

	// Remove untracked files (but not ignored files)
	cleanCmd := exec.Command("git", "clean", "-fd")
	if err := cleanCmd.Run(); err != nil {
		return fmt.Errorf("git clean failed: %w", err)
	}

	return nil
}
