package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const SessionName = "autoclaude"

// SessionExists checks if a tmux session with our name exists
func SessionExists() bool {
	cmd := exec.Command("tmux", "has-session", "-t", SessionName)
	return cmd.Run() == nil
}

// CreateSession creates a new tmux session
func CreateSession(workDir string) error {
	cmd := exec.Command("tmux", "new-session", "-d", "-s", SessionName, "-c", workDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// KillSession kills the tmux session
func KillSession() error {
	cmd := exec.Command("tmux", "kill-session", "-t", SessionName)
	return cmd.Run()
}

// SendCommand sends a command to the tmux session
func SendCommand(command string) error {
	// Send command and Enter together - don't use -l flag so shell operators work
	cmd := exec.Command("tmux", "send-keys", "-t", SessionName, command, "Enter")
	return cmd.Run()
}

// Attach attaches to the tmux session (replaces current process)
func Attach() error {
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found: %w", err)
	}

	args := []string{"tmux", "attach-session", "-t", SessionName}
	return syscall.Exec(tmuxPath, args, os.Environ())
}

// AttachAndWait attaches to the tmux session and waits for it to end
// Returns when the session no longer exists
func AttachAndWait() error {
	// Attach in foreground (not exec, so we return when done)
	cmd := exec.Command("tmux", "attach-session", "-t", SessionName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// This blocks until the user detaches or the session ends
	err := cmd.Run()

	// If session ended, that's fine
	if !SessionExists() {
		return nil
	}

	return err
}

// WaitForSessionEnd polls until the session no longer exists
func WaitForSessionEnd() {
	for SessionExists() {
		// Small sleep to avoid busy loop
		exec.Command("sleep", "0.5").Run()
	}
}

// RunInSession creates a session if needed and runs a command in it
func RunInSession(workDir, command string) error {
	// Kill existing session if it exists
	if SessionExists() {
		if err := KillSession(); err != nil {
			// Ignore errors, session might have just ended
		}
	}

	// Create new session
	if err := CreateSession(workDir); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Send the command
	if err := SendCommand(command); err != nil {
		return fmt.Errorf("failed to send command to tmux: %w", err)
	}

	return nil
}

// RunAndAttach creates a session, runs a command, and attaches
func RunAndAttach(workDir, command string) error {
	if err := RunInSession(workDir, command); err != nil {
		return err
	}

	return Attach()
}

// RunClaudeWithPromptFile runs Claude with a prompt read from a file
// This avoids shell quoting issues with long prompts by creating a runner script
func RunClaudeWithPromptFile(workDir, promptFile string, planMode bool) error {
	// Kill existing session if it exists
	if SessionExists() {
		if err := KillSession(); err != nil {
			// Ignore errors
		}
	}

	// Create a runner script that properly handles the prompt
	runnerPath := filepath.Join(workDir, ".autoclaude", "run_claude.sh")
	var claudeCmd string
	if planMode {
		claudeCmd = "claude --permission-mode plan"
	} else {
		claudeCmd = "claude"
	}

	scriptContent := fmt.Sprintf(`#!/bin/bash
set -e
PROMPT_FILE=%q
if [ ! -f "$PROMPT_FILE" ]; then
    echo "ERROR: Prompt file not found: $PROMPT_FILE"
    echo "Press Enter to exit..."
    read
    exit 1
fi
echo "Starting Claude with prompt from: $PROMPT_FILE"
echo "---"
%s -- "$(cat "$PROMPT_FILE")"
echo ""
echo "--- Claude session ended ---"
echo "Press Enter to continue..."
read
`, promptFile, claudeCmd)

	if err := os.WriteFile(runnerPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write runner script: %w", err)
	}

	// Create new session
	if err := CreateSession(workDir); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Run the script
	if err := SendCommand(runnerPath); err != nil {
		return fmt.Errorf("failed to send command to tmux: %w", err)
	}

	return AttachAndWait()
}

// IsInsideTmux checks if we're already running inside tmux
func IsInsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

// GetSessionPID gets the PID of the tmux server for our session
func GetSessionPID() (string, error) {
	cmd := exec.Command("tmux", "display-message", "-t", SessionName, "-p", "#{pid}")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
