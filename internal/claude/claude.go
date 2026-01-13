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
