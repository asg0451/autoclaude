package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
	"github.com/spf13/cobra"
	"go.coldcutz.net/autoclaude/internal/claude"
	"go.coldcutz.net/autoclaude/internal/config"
	"go.coldcutz.net/autoclaude/internal/prompt"
	"go.coldcutz.net/autoclaude/internal/state"
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

	// Step 0: Ensure git repo exists
	if !isGitRepo() {
		fmt.Println("  Initializing git repository...")
		if err := initGitRepo(); err != nil {
			return fmt.Errorf("failed to initialize git repo: %w", err)
		}
	}

	// Create prompt parameters
	params := prompt.PromptParams{
		Goal:        goal,
		TestCmd:     initTestCmd,
		Constraints: initConstraints,
	}

	// Step 1: Create .autoclaude directory structure first
	fmt.Println("  Creating .autoclaude directory...")
	if err := state.InitDir(goal, initTestCmd); err != nil {
		return fmt.Errorf("failed to create .autoclaude directory: %w", err)
	}

	// Step 1b: Generate language-specific coding guidelines
	fmt.Println("  Detecting languages...")
	langs := state.DetectLanguages()
	if len(langs) == 0 {
		// No languages detected, ask user
		fmt.Println("  No languages detected in repo.")
		userLangs, err := promptForLanguages()
		if err != nil {
			return fmt.Errorf("failed to get languages: %w", err)
		}
		langs = userLangs
	}
	if len(langs) > 0 {
		fmt.Printf("  Languages: %v\n", langs)
	}
	fmt.Println("  Generating coding guidelines...")
	if err := state.WriteGuidelinesForLanguages(langs); err != nil {
		return fmt.Errorf("failed to write coding guidelines: %w", err)
	}

	// Step 2: Generate and save prompts
	fmt.Println("  Generating prompts...")
	if err := prompt.SavePrompts(params); err != nil {
		return fmt.Errorf("failed to save prompts: %w", err)
	}

	// Step 3: Set up permissions (but NOT stop hook - that's only for run)
	fmt.Println("  Configuring Claude settings...")
	if err := config.SetupPermissions(); err != nil {
		return fmt.Errorf("failed to setup settings: %w", err)
	}

	// Step 4: Create initial state
	fmt.Println("  Creating initial state...")
	s := state.NewState(goal, initTestCmd, initConstraints, initMaxIterations)
	if err := s.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Step 5: Run planner to generate initial TODOs (interactive, inline with acceptEdits mode)
	if !initSkipPlanner {
		fmt.Println("  Running collaborative planner...")
		fmt.Println("  (Claude will work with you to clarify the design)")
		fmt.Println()

		// Clean up any stale planning_complete file from previous runs
		config.RemovePlanningComplete()

		// Save planner prompt to file
		plannerPath, err := prompt.SavePlannerPrompt(params)
		if err != nil {
			return fmt.Errorf("failed to save planner prompt: %w", err)
		}

		// Get autoclaude path for stop hook
		autoclaudePath, err := GetExecutablePath()
		if err != nil {
			return fmt.Errorf("failed to get autoclaude path: %w", err)
		}

		// Set up planner stop hook (only kills Claude when planning_complete file exists)
		if err := config.SetupPlannerStopHook(autoclaudePath); err != nil {
			return fmt.Errorf("failed to setup planner stop hook: %w", err)
		}

		// Run Claude inline with acceptEdits permission mode
		if err := claude.RunInteractiveWithPromptFile(plannerPath, "acceptEdits"); err != nil {
			// Clean up hook even on error
			config.RemovePlannerStopHook(autoclaudePath)
			config.RemovePlanningComplete()
			return fmt.Errorf("failed to run planner: %w", err)
		}

		// Remove planner stop hook and planning_complete marker
		if err := config.RemovePlannerStopHook(autoclaudePath); err != nil {
			// Non-fatal, continue
		}
		config.RemovePlanningComplete()

		// Commit all planner output
		fmt.Println("  Committing planner output...")
		if err := commitPlannerOutput(); err != nil {
			fmt.Printf("  Warning: failed to commit planner output: %v\n", err)
		}
	}

	fmt.Println()
	fmt.Println("Initialization complete!")
	fmt.Println()
	fmt.Println("Created:")
	fmt.Printf("  %s  (prompts & tracking)\n", config.AutoclaudeDir)
	fmt.Printf("  %s  (permissions & hooks)\n", config.SettingsPath())
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review .autoclaude/TODO.md")
	fmt.Println("  2. Run: autoclaude run")

	return nil
}

func gatherRequirements(existingGoal string) (goal, testCmd, constraints string, err error) {
	rl, err := readline.New("")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer rl.Close()

	// Goal
	if existingGoal != "" {
		goal = existingGoal
		fmt.Printf("Goal: %s\n", goal)
	} else {
		rl.SetPrompt("What is the goal? ")
		goal, err = rl.Readline()
		if err != nil {
			return "", "", "", fmt.Errorf("failed to read goal: %w", err)
		}
	}

	// Test command
	if initTestCmd != "" {
		testCmd = initTestCmd
		fmt.Printf("Test command: %s\n", testCmd)
	} else {
		rl.SetPrompt("What command verifies success? (e.g., make test, go test ./...) ")
		testCmd, err = rl.Readline()
		if err != nil {
			return "", "", "", fmt.Errorf("failed to read test command: %w", err)
		}
	}

	// Constraints
	if initConstraints != "" {
		constraints = initConstraints
		fmt.Printf("Constraints: %s\n", constraints)
	} else {
		rl.SetPrompt("Any constraints or rules? (press Enter to skip) ")
		constraints, err = rl.Readline()
		if err != nil {
			return "", "", "", fmt.Errorf("failed to read constraints: %w", err)
		}
	}

	return goal, testCmd, constraints, nil
}

// getExecutablePath returns the absolute path to the autoclaude binary
func getExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(exe)
}

// GetExecutablePath is exported for use by other commands
func GetExecutablePath() (string, error) {
	return getExecutablePath()
}

// isGitRepo checks if the current directory is a git repository
func isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// initGitRepo initializes a new git repository
func initGitRepo() error {
	cmd := exec.Command("git", "init")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// commitPlannerOutput commits all files created by the planner
func commitPlannerOutput() error {
	// Stage all changes
	addCmd := exec.Command("git", "add", "-A")
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	// Check if there's anything to commit
	statusCmd := exec.Command("git", "status", "--porcelain")
	output, err := statusCmd.Output()
	if err != nil {
		return fmt.Errorf("git status failed: %w", err)
	}
	if len(output) == 0 {
		return nil // Nothing to commit
	}

	// Commit
	commitCmd := exec.Command("git", "commit", "-m", "autoclaude: initial plan and TODOs")
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	return nil
}

// promptForLanguages asks the user which languages will be used
func promptForLanguages() ([]state.Language, error) {
	rl, err := readline.New("")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer rl.Close()

	fmt.Println("  Available: go, rust, python, node/typescript")
	rl.SetPrompt("  Which language(s) will you use? (comma-separated, or Enter to skip) ")
	input, err := rl.Readline()
	if err != nil {
		return nil, err
	}

	if input == "" {
		return nil, nil
	}

	var langs []state.Language
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if lang, ok := state.ParseLanguage(part); ok {
			langs = append(langs, lang)
		} else if part != "" {
			fmt.Printf("  (unknown language: %s, skipping)\n", part)
		}
	}

	return langs, nil
}
