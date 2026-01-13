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
	"go.coldcutz.net/autoclaude/internal/claude"
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
	Short:  "Internal: called by stop hook when Claude stops",
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
		return outputAllow()
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
		return outputAllow()
	}

	s, err := state.Load()
	if err != nil {
		return outputAllow()
	}

	// Auto-commit any pending changes
	if s.Step == state.StepCoder {
		commitMsg := fmt.Sprintf("autoclaude: coder iteration %d", s.Iteration)
		if err := gitCommit(commitMsg); err != nil {
			appendToNotes(fmt.Sprintf("Failed to auto-commit: %v", err))
		} else {
			s.LastCommit = getLastCommitHash()
		}
	}

	// Save transcript path for potential debugging
	if hookInput.TranscriptPath != "" {
		appendToNotes(fmt.Sprintf("Session ended, transcript: %s", hookInput.TranscriptPath))
	}

	// Mark that this step completed
	s.Save()

	// Kill Claude process to ensure it exits and returns control to autoclaude
	claude.KillClaude()

	// Allow Claude to stop - the outer loop in `run` will handle next steps
	return outputAllow()
}

// outputAllow outputs JSON to allow Claude to stop
func outputAllow() error {
	output := StopHookOutput{}
	data, _ := json.Marshal(output)
	fmt.Println(string(data))
	return nil
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
