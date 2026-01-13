package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go.coldcutz.net/autoclaude/internal/claude"
	"go.coldcutz.net/autoclaude/internal/config"
	"go.coldcutz.net/autoclaude/internal/prompt"
	"go.coldcutz.net/autoclaude/internal/state"
	"go.coldcutz.net/autoclaude/internal/tmux"
)

var (
	initInteractive   bool
	initTestCmd       string
	initConstraints   string
	initMaxIterations int
	initSkipPlanner   bool
)

var initCmd = &cobra.Command{
	Use:   "init [goal]",
	Short: "Initialize autoclaude for a project",
	Long: `Initialize autoclaude by setting up prompts, TODOs, and configuration.

In interactive mode (--interactive), you'll be asked for:
  - Goal description
  - Test command
  - Constraints/rules

You can also provide these via flags or positional argument.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().BoolVarP(&initInteractive, "interactive", "i", false, "Run in interactive mode")
	initCmd.Flags().StringVarP(&initTestCmd, "test-cmd", "t", "", "Test command to verify changes")
	initCmd.Flags().StringVarP(&initConstraints, "constraints", "c", "", "Additional constraints or rules")
	initCmd.Flags().IntVarP(&initMaxIterations, "max-iterations", "m", 3, "Maximum coder-critic iterations")
	initCmd.Flags().BoolVar(&initSkipPlanner, "skip-planner", false, "Skip running the planner to generate initial TODOs")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if claude is installed
	if err := claude.CheckInstalled(); err != nil {
		return err
	}

	var goal string
	if len(args) > 0 {
		goal = args[0]
	}

	// Gather requirements
	if initInteractive || goal == "" {
		var err error
		goal, initTestCmd, initConstraints, err = gatherRequirements(goal)
		if err != nil {
			return err
		}
	}

	if goal == "" {
		return fmt.Errorf("goal is required. Provide as argument or use --interactive")
	}

	if initTestCmd == "" {
		return fmt.Errorf("test command is required. Use --test-cmd or --interactive")
	}

	fmt.Println("Initializing autoclaude...")

	// Create prompt parameters
	params := prompt.PromptParams{
		Goal:        goal,
		TestCmd:     initTestCmd,
		Constraints: initConstraints,
	}

	// Step 1: Generate and save prompts
	fmt.Println("  Generating prompts...")
	if err := prompt.SavePrompts(params); err != nil {
		return fmt.Errorf("failed to save prompts: %w", err)
	}

	// Step 2: Create .autoclaude directory structure
	fmt.Println("  Creating .autoclaude directory...")
	if err := state.InitDir(goal, initTestCmd); err != nil {
		return fmt.Errorf("failed to create .autoclaude directory: %w", err)
	}

	// Step 3: Set up permissions and stop hook
	fmt.Println("  Configuring Claude settings...")
	autoclaudePath, err := getExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get autoclaude path: %w", err)
	}
	if err := config.SetupSettings(autoclaudePath); err != nil {
		return fmt.Errorf("failed to setup settings: %w", err)
	}

	// Step 4: Create initial state
	fmt.Println("  Creating initial state...")
	s := state.NewState(goal, initTestCmd, initConstraints, initMaxIterations)
	if err := s.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Step 5: Run planner to generate initial TODOs (interactive in tmux)
	if !initSkipPlanner {
		fmt.Println("  Running planner to generate TODOs...")
		fmt.Println("  (You can interact with Claude to clarify scope)")
		fmt.Println()

		plannerPrompt := prompt.GeneratePlanner(params)

		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		claudeCmd := claude.BuildCommand(plannerPrompt)
		if err := tmux.RunAndAttach(wd, claudeCmd); err != nil {
			return fmt.Errorf("failed to run planner: %w", err)
		}
	}

	fmt.Println()
	fmt.Println("Initialization complete!")
	fmt.Println()
	fmt.Println("Created:")
	fmt.Printf("  %s  (prompts)\n", config.PromptsDir)
	fmt.Printf("  %s  (state & tracking)\n", state.AutoclaudeDir)
	fmt.Printf("  %s  (permissions & hooks)\n", config.SettingsPath())
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review .autoclaude/TODO.md")
	fmt.Println("  2. Run: autoclaude run")

	return nil
}

func gatherRequirements(existingGoal string) (goal, testCmd, constraints string, err error) {
	reader := bufio.NewReader(os.Stdin)

	// Goal
	if existingGoal != "" {
		goal = existingGoal
		fmt.Printf("Goal: %s\n", goal)
	} else {
		fmt.Print("What is the goal? ")
		goal, err = reader.ReadString('\n')
		if err != nil {
			return "", "", "", fmt.Errorf("failed to read goal: %w", err)
		}
		goal = strings.TrimSpace(goal)
	}

	// Test command
	if initTestCmd != "" {
		testCmd = initTestCmd
		fmt.Printf("Test command: %s\n", testCmd)
	} else {
		fmt.Print("What command verifies success? (e.g., make test, go test ./...) ")
		testCmd, err = reader.ReadString('\n')
		if err != nil {
			return "", "", "", fmt.Errorf("failed to read test command: %w", err)
		}
		testCmd = strings.TrimSpace(testCmd)
	}

	// Constraints
	if initConstraints != "" {
		constraints = initConstraints
		fmt.Printf("Constraints: %s\n", constraints)
	} else {
		fmt.Print("Any constraints or rules? (press Enter to skip) ")
		constraints, err = reader.ReadString('\n')
		if err != nil {
			return "", "", "", fmt.Errorf("failed to read constraints: %w", err)
		}
		constraints = strings.TrimSpace(constraints)
	}

	return goal, testCmd, constraints, nil
}

func getExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(exe)
}
