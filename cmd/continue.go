package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.coldcutz.net/autoclaude/internal/prompt"
	"go.coldcutz.net/autoclaude/internal/state"
)

// StopHookInput represents the input from Claude Code stop hook
type StopHookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	StopHookActive bool   `json:"stop_hook_active"`
}

// StopHookOutput represents the output for Claude Code stop hook
type StopHookOutput struct {
	Decision string `json:"decision,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

var continueCmd = &cobra.Command{
	Use:    "_continue",
	Short:  "Internal: continue the loop (called by stop hook)",
	Hidden: true,
	RunE:   runContinue,
}

func init() {
	rootCmd.AddCommand(continueCmd)
}

func runContinue(cmd *cobra.Command, args []string) error {
	// Read stop hook input from stdin
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return outputBlock("Failed to read stdin")
	}

	var hookInput StopHookInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &hookInput); err != nil {
			// Not fatal, might not have input
		}
	}

	// Prevent infinite loops
	if hookInput.StopHookActive {
		return outputAllow()
	}

	// Load state
	if !state.Exists() {
		return outputAllow() // No state, just let Claude stop
	}

	s, err := state.Load()
	if err != nil {
		return outputAllow() // Can't load state, let stop
	}

	// Handle based on current step
	switch s.Step {
	case state.StepCoder:
		return handleCoderDone(s)
	case state.StepCritic:
		return handleCriticDone(s)
	case state.StepEvaluator:
		return handleEvaluatorDone(s)
	case state.StepDone:
		return outputAllow()
	default:
		return outputAllow()
	}
}

func handleCoderDone(s *state.State) error {
	// Auto-commit changes
	if err := gitCommit("autoclaude: coder iteration " + fmt.Sprintf("%d", s.Iteration)); err != nil {
		// Log but don't fail
		appendToNotes(fmt.Sprintf("Failed to auto-commit: %v", err))
	} else {
		s.LastCommit = getLastCommitHash()
	}

	// Transition to critic
	s.Step = state.StepCritic
	if err := s.Save(); err != nil {
		return outputBlock(fmt.Sprintf("Failed to save state: %v", err))
	}

	if err := s.UpdateStatus("Running critic review..."); err != nil {
		// Non-fatal
	}

	// Load and run critic
	criticPrompt, err := prompt.LoadCritic()
	if err != nil {
		return outputBlock(fmt.Sprintf("Failed to load critic prompt: %v", err))
	}

	return outputBlockWithCommand(criticPrompt, "Review the changes made by the coder")
}

func handleCriticDone(s *state.State) error {
	// Check if there are remaining TODOs
	hasTodos, err := hasIncompleteTodos()
	if err != nil {
		appendToNotes(fmt.Sprintf("Failed to check TODOs: %v", err))
	}

	if hasTodos {
		// More work to do, check iteration limit
		if s.Iteration >= s.MaxIterations {
			s.Step = state.StepDone
			if err := s.Save(); err != nil {
				return outputBlock(fmt.Sprintf("Failed to save state: %v", err))
			}
			s.UpdateStatus("Max iterations reached. Review .autoclaude/TODO.md for remaining items.")
			return outputAllow()
		}

		// Continue with next iteration
		s.Iteration++
		s.Step = state.StepCoder
		s.RetryCount = 0
		if err := s.Save(); err != nil {
			return outputBlock(fmt.Sprintf("Failed to save state: %v", err))
		}

		s.UpdateStatus(fmt.Sprintf("Starting iteration %d/%d...", s.Iteration, s.MaxIterations))

		coderPrompt, err := prompt.LoadCoder()
		if err != nil {
			return outputBlock(fmt.Sprintf("Failed to load coder prompt: %v", err))
		}

		return outputBlockWithCommand(coderPrompt, "Continue working on remaining TODOs")
	}

	// No more TODOs, run evaluator
	s.Step = state.StepEvaluator
	if err := s.Save(); err != nil {
		return outputBlock(fmt.Sprintf("Failed to save state: %v", err))
	}

	s.UpdateStatus("All TODOs complete. Running evaluator...")

	evalPrompt, err := prompt.LoadEvaluator()
	if err != nil {
		return outputBlock(fmt.Sprintf("Failed to load evaluator prompt: %v", err))
	}

	return outputBlockWithCommand(evalPrompt, "Evaluate if the goal is fully achieved")
}

func handleEvaluatorDone(s *state.State) error {
	// Check if there are new TODOs (evaluator might have added some)
	hasTodos, _ := hasIncompleteTodos()

	if hasTodos && s.Iteration < s.MaxIterations {
		// Evaluator added more work, continue
		s.Iteration++
		s.Step = state.StepCoder
		if err := s.Save(); err != nil {
			return outputBlock(fmt.Sprintf("Failed to save state: %v", err))
		}

		s.UpdateStatus("Evaluator added more TODOs. Continuing...")

		coderPrompt, err := prompt.LoadCoder()
		if err != nil {
			return outputBlock(fmt.Sprintf("Failed to load coder prompt: %v", err))
		}

		return outputBlockWithCommand(coderPrompt, "Work on newly added TODOs")
	}

	// Done!
	s.Step = state.StepDone
	if err := s.Save(); err != nil {
		return outputBlock(fmt.Sprintf("Failed to save state: %v", err))
	}

	s.UpdateStatus("Goal complete!")

	return outputAllow()
}

// outputAllow outputs JSON to allow Claude to stop
func outputAllow() error {
	output := StopHookOutput{}
	data, _ := json.Marshal(output)
	fmt.Println(string(data))
	return nil
}

// outputBlock outputs JSON to block Claude from stopping
func outputBlock(reason string) error {
	output := StopHookOutput{
		Decision: "block",
		Reason:   reason,
	}
	data, _ := json.Marshal(output)
	fmt.Println(string(data))
	return nil
}

// outputBlockWithCommand blocks and provides a new command/prompt to execute
func outputBlockWithCommand(prompt, reason string) error {
	fullReason := fmt.Sprintf("%s\n\n%s", reason, prompt)
	return outputBlock(fullReason)
}

// gitCommit makes a git commit with the given message
func gitCommit(message string) error {
	// Check if there are changes to commit
	statusCmd := exec.Command("git", "status", "--porcelain")
	output, err := statusCmd.Output()
	if err != nil {
		return err
	}

	if len(strings.TrimSpace(string(output))) == 0 {
		return nil // No changes to commit
	}

	// Add all changes
	addCmd := exec.Command("git", "add", "-A")
	if err := addCmd.Run(); err != nil {
		return err
	}

	// Commit
	commitCmd := exec.Command("git", "commit", "-m", message)
	return commitCmd.Run()
}

// getLastCommitHash gets the hash of the last commit
func getLastCommitHash() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// hasIncompleteTodos checks if there are incomplete TODOs
func hasIncompleteTodos() (bool, error) {
	data, err := os.ReadFile(state.TodoPath())
	if err != nil {
		return false, err
	}

	content := string(data)
	// Look for unchecked checkboxes
	return strings.Contains(content, "- [ ]"), nil
}

// appendToNotes appends a note to NOTES.md
func appendToNotes(note string) error {
	f, err := os.OpenFile(state.NotesPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, err = f.WriteString(fmt.Sprintf("\n- [%s] %s\n", timestamp, note))
	return err
}
