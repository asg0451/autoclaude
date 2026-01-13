package cmd

import (
	"fmt"
	"os"
	"os/exec"
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
	s.Stats = &state.Stats{} // Initialize fresh stats
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

		// Inner loop: critic review with fix retries (max 3)
		wasApproved := false
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
					// Run fixer with critic's feedback
					fmt.Println("=== FIXER ===")
					s.Step = state.StepCoder
					s.Save()
					s.UpdateStatus(fmt.Sprintf("Fixing: %s", currentTodo))
					s.Stats.FixAttempts++

					fixerCommitBefore := getCommitHash()
					// Use state.GetCurrentTodo() to read from file (robust across restarts)
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

		// Exhausted retries
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

	evalPrompt, _ := prompt.LoadEvaluator()
	promptPath, _ := prompt.WriteCurrentPrompt(evalPrompt)
	if err := runClaudePhase(promptPath, s.Stats); err != nil {
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

	// Print stats summary
	printStats(s.Stats)

	return nil
}

// runClaudePhase runs a Claude session in the foreground and waits for it to complete
func runClaudePhase(promptFile string, stats *state.Stats) error {
	if stats != nil {
		stats.ClaudeRuns++
	}
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

// getCommitHash returns the current HEAD commit hash (short form)
func getCommitHash() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// checkCommitCreated verifies a new commit was made; if not, forces one
func checkCommitCreated(beforeHash, phase string) {
	afterHash := getCommitHash()
	if afterHash == beforeHash {
		fmt.Printf("  ⚠ %s did not commit, forcing commit...\n", phase)
		forceCommit(phase)
	}
}

// forceCommit stages all changes and commits them
func forceCommit(phase string) {
	// Check if there are any changes to commit
	statusCmd := exec.Command("git", "status", "--porcelain")
	output, err := statusCmd.Output()
	if err != nil || len(strings.TrimSpace(string(output))) == 0 {
		fmt.Println("  (no changes to commit)")
		return
	}

	// Stage all changes
	addCmd := exec.Command("git", "add", "-A")
	if err := addCmd.Run(); err != nil {
		fmt.Printf("  ✗ Failed to stage changes: %v\n", err)
		return
	}

	// Commit
	msg := fmt.Sprintf("autoclaude: %s changes (auto-committed)", strings.ToLower(phase))
	commitCmd := exec.Command("git", "commit", "-m", msg)
	if err := commitCmd.Run(); err != nil {
		fmt.Printf("  ✗ Failed to commit: %v\n", err)
		return
	}

	fmt.Printf("  ✓ Auto-committed as: %s\n", getCommitHash())
}

// printStats displays run statistics
func printStats(stats *state.Stats) {
	if stats == nil {
		return
	}

	fmt.Println("\n─── Run Statistics ───")
	fmt.Printf("  Claude invocations:  %d\n", stats.ClaudeRuns)
	fmt.Printf("  TODOs attempted:     %d\n", stats.TodosAttempted)
	fmt.Printf("  TODOs completed:     %d\n", stats.TodosCompleted)
	fmt.Println()
	fmt.Printf("  Critic verdicts:\n")
	fmt.Printf("    Approved:          %d\n", stats.CriticApprovals)
	fmt.Printf("    Minor issues:      %d\n", stats.CriticMinor)
	fmt.Printf("    Needs fixes:       %d\n", stats.CriticRejections)
	fmt.Println()
	fmt.Printf("  Fix attempts:        %d\n", stats.FixAttempts)
	fmt.Printf("  Fix successes:       %d\n", stats.FixSuccesses)

	// Calculate rates
	totalReviews := stats.CriticApprovals + stats.CriticMinor + stats.CriticRejections
	if totalReviews > 0 {
		acceptRate := float64(stats.CriticApprovals+stats.CriticMinor) / float64(totalReviews) * 100
		fmt.Printf("\n  First-pass accept rate: %.1f%%\n", acceptRate)
	}
	if stats.FixAttempts > 0 {
		fixRate := float64(stats.FixSuccesses) / float64(stats.FixAttempts) * 100
		fmt.Printf("  Fix success rate:       %.1f%%\n", fixRate)
	}
	fmt.Println("──────────────────────")
}
