package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.coldcutz.net/autoclaude/internal/claude"
	"go.coldcutz.net/autoclaude/internal/config"
	"go.coldcutz.net/autoclaude/internal/prompt"
	"go.coldcutz.net/autoclaude/internal/state"
)

var devEvalCmd = &cobra.Command{
	Use:    "dev-eval",
	Short:  "Debug: run just the evaluator phase",
	Hidden: true,
	RunE:   runDevEval,
}

func init() {
	rootCmd.AddCommand(devEvalCmd)
}

func runDevEval(cmd *cobra.Command, args []string) error {
	if err := claude.CheckInstalled(); err != nil {
		return err
	}

	if !state.Exists() {
		return fmt.Errorf("autoclaude not initialized. Run 'autoclaude init' first")
	}

	s, err := state.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	fmt.Println("=== DEV: EVALUATOR ===")
	fmt.Printf("Goal: %s\n", s.Goal)
	fmt.Printf("Test command: %s\n", s.TestCmd)
	fmt.Println()

	// Clean up any stale evaluation_complete file
	config.RemoveEvaluationComplete()

	// Get autoclaude path for stop hook
	autoclaudePath, err := GetExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get autoclaude path: %w", err)
	}

	// Set up evaluator stop hook
	if err := config.SetupEvaluatorStopHook(autoclaudePath); err != nil {
		return fmt.Errorf("failed to setup evaluator stop hook: %w", err)
	}

	evalPrompt, err := prompt.LoadEvaluator()
	if err != nil {
		config.RemoveEvaluatorStopHook(autoclaudePath)
		return fmt.Errorf("failed to load evaluator prompt: %w", err)
	}

	promptPath, err := prompt.WriteCurrentPrompt(evalPrompt)
	if err != nil {
		config.RemoveEvaluatorStopHook(autoclaudePath)
		return fmt.Errorf("failed to write prompt: %w", err)
	}

	fmt.Printf("Prompt file: %s\n", promptPath)
	fmt.Println()

	if err := claude.RunInteractiveWithPromptFile(promptPath, "acceptEdits"); err != nil {
		config.RemoveEvaluatorStopHook(autoclaudePath)
		config.RemoveEvaluationComplete()
		return fmt.Errorf("evaluator failed: %w", err)
	}

	// Clean up
	config.RemoveEvaluatorStopHook(autoclaudePath)

	// Report result
	if config.IsEvaluationComplete() {
		fmt.Println("\n=== EVALUATION COMPLETE ===")
		fmt.Println("User confirmed done.")
		config.RemoveEvaluationComplete()
	} else if hasIncompleteTodos() {
		fmt.Println("\n=== MORE WORK NEEDED ===")
		fmt.Println("Evaluator added TODOs or user requested changes.")
	} else {
		fmt.Println("\n=== EVALUATOR EXITED ===")
		fmt.Println("No evaluation_complete marker and no incomplete TODOs.")
	}

	return nil
}
