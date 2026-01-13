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
	"go.coldcutz.net/autoclaude/internal/tmux"
)

var runMaxIterations int

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the coder-critic loop",
	Long: `Start the autoclaude coder-critic loop.

The loop proceeds as follows:
1. Coder: Works on TODOs (with stop hook for auto-commit)
2. Critic: Reviews changes in fresh session (no hook)
3. Repeat until all TODOs complete or max iterations reached
4. Evaluator: Checks if overall goal is met

Each phase runs in a fresh Claude session to maintain quality.`,
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

	autoclaudePath, err := GetExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get autoclaude path: %w", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	fmt.Println("Starting autoclaude loop...")
	fmt.Printf("  Goal: %s\n", s.Goal)
	fmt.Printf("  Test command: %s\n", s.TestCmd)
	fmt.Printf("  Max iterations: %d\n", s.MaxIterations)
	fmt.Println()

	// Main orchestration loop
	for s.Iteration <= s.MaxIterations {
		// === CODER PHASE ===
		fmt.Printf("=== Iteration %d/%d: CODER ===\n", s.Iteration, s.MaxIterations)
		s.Step = state.StepCoder
		s.Save()
		s.UpdateStatus(fmt.Sprintf("Running coder (iteration %d)...", s.Iteration))

		// Enable stop hook for coder (for auto-commit)
		if err := config.SetupStopHook(autoclaudePath); err != nil {
			return fmt.Errorf("failed to setup stop hook: %w", err)
		}

		// Run coder
		coderPrompt, _ := prompt.LoadCoder()
		promptPath, _ := prompt.WriteCurrentPrompt(coderPrompt)
		if err := runClaudePhase(wd, promptPath); err != nil {
			return fmt.Errorf("coder phase failed: %w", err)
		}

		// Remove stop hook before critic
		if err := config.RemoveStopHook(autoclaudePath); err != nil {
			// Non-fatal, continue
		}

		// === CRITIC PHASE ===
		fmt.Printf("=== Iteration %d/%d: CRITIC ===\n", s.Iteration, s.MaxIterations)
		s.Step = state.StepCritic
		s.Save()
		s.UpdateStatus("Running critic review...")

		// Run critic (fresh session, no hook)
		criticPrompt, _ := prompt.LoadCritic()
		promptPath, _ = prompt.WriteCurrentPrompt(criticPrompt)
		if err := runClaudePhase(wd, promptPath); err != nil {
			return fmt.Errorf("critic phase failed: %w", err)
		}

		// Check if there are remaining TODOs
		if !hasIncompleteTodos() {
			break // Move to evaluator
		}

		s.Iteration++
		s.Save()
	}

	// === EVALUATOR PHASE ===
	fmt.Println("=== EVALUATOR ===")
	s.Step = state.StepEvaluator
	s.Save()
	s.UpdateStatus("Running evaluator...")

	evalPrompt, _ := prompt.LoadEvaluator()
	promptPath, _ := prompt.WriteCurrentPrompt(evalPrompt)
	if err := runClaudePhase(wd, promptPath); err != nil {
		return fmt.Errorf("evaluator phase failed: %w", err)
	}

	// Check if evaluator added more TODOs
	if hasIncompleteTodos() && s.Iteration < s.MaxIterations {
		fmt.Println("Evaluator added more TODOs. Run 'autoclaude run' again to continue.")
	} else {
		s.Step = state.StepDone
		s.Save()
		s.UpdateStatus("Complete!")
		fmt.Println("=== COMPLETE ===")
	}

	return nil
}

// runClaudePhase runs a Claude session and waits for it to complete
func runClaudePhase(workDir, promptFile string) error {
	// Create runner script that keeps shell alive for debugging
	runnerPath := workDir + "/.autoclaude/run_claude.sh"
	scriptContent := fmt.Sprintf(`#!/bin/bash
set -e
PROMPT_FILE=%q
if [ ! -f "$PROMPT_FILE" ]; then
    echo "ERROR: Prompt file not found: $PROMPT_FILE"
    read -p "Press Enter to exit..."
    exit 1
fi
echo "Running claude with prompt from: $PROMPT_FILE"
claude -- "$(cat "$PROMPT_FILE")"
EXIT_CODE=$?
echo ""
echo "Claude exited with code: $EXIT_CODE"
echo "Press Enter to continue to next phase..."
read
`, promptFile)
	if err := os.WriteFile(runnerPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write runner script: %w", err)
	}

	// Kill any existing session
	if tmux.SessionExists() {
		tmux.KillSession()
	}

	// Create session and run command
	if err := tmux.CreateSession(workDir); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	if err := tmux.SendCommand(runnerPath); err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}

	// Attach and wait for completion
	fmt.Println("(Attaching to tmux session)")
	return tmux.AttachAndWait()
}

// hasIncompleteTodos checks if there are incomplete TODOs
func hasIncompleteTodos() bool {
	data, err := os.ReadFile(state.TodoPath())
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "- [ ]")
}
