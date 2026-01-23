package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.coldcutz.net/autoclaude/internal/claude"
	"go.coldcutz.net/autoclaude/internal/config"
	"go.coldcutz.net/autoclaude/internal/prompt"
	"go.coldcutz.net/autoclaude/internal/state"
)

const maxFixRetries = 3

var (
	runCoderSonnet bool
	runPruneInterval int // 0 means use default
)

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
	runCmd.Flags().BoolVar(&runCoderSonnet, "coder-sonnet", false, "Use Sonnet model for coder/fixer phases")
	runCmd.Flags().IntVar(&runPruneInterval, "prune-interval", 0, "Number of TODOs between auto-pruning (0 for default 5, -1 to disable)")
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

	// Determine coder model
	coderModel := ""
	if runCoderSonnet {
		coderModel = "sonnet"
	}

	// Determine prune interval
	pruneInterval := state.DefaultPruneInterval
	if runPruneInterval > 0 {
		pruneInterval = runPruneInterval
	} else if runPruneInterval == -1 {
		pruneInterval = 0 // Disabled
	}

	fmt.Println("Starting autoclaude loop...")
	fmt.Printf("  Goal: %s\n", s.Goal)
	fmt.Printf("  Test command: %s\n", s.TestCmd)
	if coderModel != "" {
		fmt.Printf("  Coder model: %s\n", coderModel)
	}
	fmt.Println()

	// Enable stop hook for all phases (kills Claude when it stops to return control)
	if err := config.SetupStopHook(autoclaudePath); err != nil {
		return fmt.Errorf("failed to setup stop hook: %w", err)
	}
	defer config.RemoveStopHook(autoclaudePath)

	// Outer loop: process tasks until all complete (no limit)
	for hasPendingTasks() {
		s.Iteration++
		s.RetryCount = 0
		s.Save()

		s.Stats.TodosAttempted++

		// === CODER PHASE ===
		fmt.Printf("\n=== TASK %d: CODER ===\n", s.Iteration)
		s.Step = state.StepCoder
		s.Save()
		s.UpdateStatus("Working on next pending task")

		commitBefore := getCommitHash()
		coderPrompt, _ := prompt.LoadCoder()
		promptPath, _ := prompt.WriteCurrentPrompt(coderPrompt)
		if err := runClaudePhase(promptPath, s.Stats, coderModel); err != nil {
			return fmt.Errorf("coder phase failed: %w", err)
		}
		checkCommitCreated(commitBefore, "Coder")

		// Inner loop: critic review with fix retries (max 3)
		wasApproved := false
		for retry := 0; retry < maxFixRetries; retry++ {
			// === CRITIC PHASE ===
			fmt.Printf("=== TASK %d: CRITIC (attempt %d/%d) ===\n", s.Iteration, retry+1, maxFixRetries)
			s.Step = state.StepCritic
			s.RetryCount = retry
			s.Save()
			s.UpdateStatus("Running critic review...")

			state.ClearCriticVerdict()

			criticPrompt, _ := prompt.LoadCritic()
			promptPath, _ = prompt.WriteCurrentPrompt(criticPrompt)
			if err := runClaudePhase(promptPath, s.Stats, ""); err != nil {
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
				goto nextTask

			case state.VerdictMinorIssues:
				fmt.Println("  ✓ Critic: MINOR_ISSUES (added tasks for later)")
				s.Stats.CriticMinor++
				s.Stats.TodosCompleted++
				if retry > 0 {
					s.Stats.FixSuccesses++
				}
				wasApproved = true
				goto nextTask

			case state.VerdictNeedsFixes:
				fmt.Printf("  ✗ Critic: NEEDS_FIXES (retry %d/%d)\n", retry+1, maxFixRetries)
				s.Stats.CriticRejections++
				if retry < maxFixRetries-1 {
					// Run fixer with critic's feedback
					fmt.Println("=== FIXER ===")
					s.Step = state.StepCoder
					s.Save()
					s.UpdateStatus("Fixing current task")
					s.Stats.FixAttempts++

					fixerCommitBefore := getCommitHash()
					fixerPrompt := prompt.GenerateFixer(params, content, "current task")
					promptPath, _ = prompt.WriteCurrentPrompt(fixerPrompt)
					if err := runClaudePhase(promptPath, s.Stats, coderModel); err != nil {
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
			fmt.Printf("  ⚠ Max retries (%d) reached for task %d, moving on\n", maxFixRetries, s.Iteration)
		}

	nextTask:
		s.Save()

		// Check if we need to run periodic pruning
		if pruneInterval > 0 && s.Stats.TodosCompleted > 0 && s.Stats.TodosCompleted%pruneInterval == 0 {
			s.TodosSincePrune = 0
			s.LastPruneAt = time.Now().Unix()
			s.Save()
			if err := runInlinePruner(s, false); err != nil {
				fmt.Printf("  ⚠ Pruning failed: %v\n", err)
			}
		}
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
	if err := runClaudePhase(promptPath, s.Stats, ""); err != nil {
		config.RemoveEvaluatorStopHook(autoclaudePath)
		config.RemoveEvaluationComplete()
		return fmt.Errorf("evaluator phase failed: %w", err)
	}

	// Clean up hook and marker
	config.RemoveEvaluatorStopHook(autoclaudePath)
	config.RemoveEvaluationComplete()

	// Check if evaluator added more tasks (user requested more work)
	if hasPendingTasks() {
		fmt.Println("User requested more work. Continuing...")
		return runRun(cmd, args) // Recursive call to process new tasks
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
// model can be "sonnet", "opus", or empty for default
func runClaudePhase(promptFile string, stats *state.Stats, model string) error {
	if stats != nil {
		stats.ClaudeRuns++
	}
	return claude.RunInteractiveWithPromptFile(promptFile, "acceptEdits", model)
}

// hasPendingTasks checks if there are pending tasks by reading the pending_tasks file
func hasPendingTasks() bool {
	data, err := os.ReadFile(filepath.Join(state.AutoclaudeDir, "pending_tasks"))
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "yes"
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
