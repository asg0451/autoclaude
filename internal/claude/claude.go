package claude

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// BuildCommand builds the command string to run Claude with a prompt
func BuildCommand(prompt string) string {
	// Escape any special characters in the prompt for shell
	escaped := shellEscape(prompt)
	return fmt.Sprintf("claude %s", escaped)
}

// BuildPrintCommand builds a command for non-interactive print mode
func BuildPrintCommand(prompt string) string {
	escaped := shellEscape(prompt)
	return fmt.Sprintf("claude -p %s", escaped)
}

// shellEscape escapes a string for use in shell commands
func shellEscape(s string) string {
	// Use single quotes and escape any single quotes within
	escaped := strings.ReplaceAll(s, "'", "'\"'\"'")
	return fmt.Sprintf("'%s'", escaped)
}

// RunPrint runs Claude in print mode (non-interactive) and returns output
func RunPrint(prompt string) (string, error) {
	cmd := exec.Command("claude", "-p", prompt)
	cmd.Stdin = os.Stdin
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude exited with error: %s\nstderr: %s", exitErr.Error(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to run claude: %w", err)
	}
	return string(output), nil
}

// CheckInstalled checks if the claude CLI is installed
func CheckInstalled() error {
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found in PATH. Please install Claude Code first")
	}
	return nil
}

// PidFile is where we store the current Claude process PID
const PidFile = ".autoclaude/claude.pid"

// buildInteractiveArgs builds the argument list for running Claude interactively
func buildInteractiveArgs(prompt string, permissionMode string, model string) []string {
	args := []string{}
	if permissionMode != "" {
		args = append(args, "--permission-mode", permissionMode)
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, "--", prompt)
	return args
}

// RunInteractive runs Claude interactively with the given prompt and permission mode
// permissionMode can be "acceptEdits", "plan", or empty for default
// model can be "sonnet", "opus", or empty for default
func RunInteractive(prompt string, permissionMode string, model string) error {
	args := buildInteractiveArgs(prompt, permissionMode, model)
	cmd := exec.Command("claude", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	// Save PID so hooks can kill the process if needed
	os.WriteFile(PidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644)
	defer os.Remove(PidFile)

	return cmd.Wait()
}

// KillClaude kills the currently running Claude process using the saved PID
func KillClaude() error {
	data, err := os.ReadFile(PidFile)
	if err != nil {
		return fmt.Errorf("no Claude process found: %w", err)
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return fmt.Errorf("invalid PID: %w", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found: %w", err)
	}

	// Send SIGTERM for graceful shutdown
	return process.Signal(os.Interrupt)
}

// RunInteractiveWithPromptFile runs Claude interactively reading prompt from a file
// model can be "sonnet", "opus", or empty for default
func RunInteractiveWithPromptFile(promptFile string, permissionMode string, model string) error {
	promptData, err := os.ReadFile(promptFile)
	if err != nil {
		return fmt.Errorf("failed to read prompt file: %w", err)
	}
	return RunInteractive(string(promptData), permissionMode, model)
}

// ParseCriticOutput parses critic output to determine if approved
func ParseCriticOutput(output string) (approved bool, fixInstructions string) {
	upper := strings.ToUpper(output)
	if strings.Contains(upper, "APPROVED") {
		return true, ""
	}
	// If not approved, the entire output is the fix instructions
	return false, strings.TrimSpace(output)
}

// ParseEvaluatorOutput parses evaluator output to determine if goal is complete
func ParseEvaluatorOutput(output string) (complete bool) {
	upper := strings.ToUpper(output)
	return strings.Contains(upper, "GOAL_COMPLETE")
}
