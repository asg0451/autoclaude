package tmux

import (
	"fmt"
	"os"
	"os/exec"
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
	// Escape any single quotes in the command
	escapedCmd := strings.ReplaceAll(command, "'", "'\\''")
	cmd := exec.Command("tmux", "send-keys", "-t", SessionName, escapedCmd, "Enter")
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
