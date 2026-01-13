package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.coldcutz.net/autoclaude/internal/state"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current autoclaude status",
	Long:  `Display the current status of the autoclaude loop, including progress on TODOs and recent notes.`,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	if !state.Exists() {
		return fmt.Errorf("autoclaude not initialized. Run 'autoclaude init' first")
	}

	s, err := state.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Print state info
	fmt.Println("=== autoclaude Status ===")
	fmt.Println()
	fmt.Printf("Step:       %s\n", s.Step)
	fmt.Printf("Iteration:  %d/%d\n", s.Iteration, s.MaxIterations)
	fmt.Printf("Goal:       %s\n", s.Goal)
	fmt.Printf("Test Cmd:   %s\n", s.TestCmd)
	if s.LastCommit != "" {
		fmt.Printf("Last Commit: %s\n", s.LastCommit)
	}
	if s.LastError != "" {
		fmt.Printf("Last Error: %s\n", s.LastError)
	}

	// Print TODO progress
	fmt.Println()
	fmt.Println("=== TODOs ===")
	printTodoProgress()

	// Print recent notes
	fmt.Println()
	fmt.Println("=== Recent Notes ===")
	printRecentNotes(5)

	return nil
}

func printTodoProgress() {
	data, err := os.ReadFile(state.TodoPath())
	if err != nil {
		fmt.Println("  (no TODO.md found)")
		return
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	var completed, incomplete int
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "- [X]") {
			completed++
		} else if strings.HasPrefix(trimmed, "- [ ]") {
			incomplete++
		}
	}

	total := completed + incomplete
	if total == 0 {
		fmt.Println("  No TODOs found")
		return
	}

	fmt.Printf("  Progress: %d/%d completed (%.0f%%)\n", completed, total, float64(completed)/float64(total)*100)
	fmt.Println()

	// Print incomplete TODOs
	if incomplete > 0 {
		fmt.Println("  Remaining:")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- [ ]") {
				// Extract task name
				taskName := strings.TrimPrefix(trimmed, "- [ ] ")
				if idx := strings.Index(taskName, " - "); idx > 0 {
					taskName = taskName[:idx]
				}
				fmt.Printf("    • %s\n", taskName)
			}
		}
	}
}

func printRecentNotes(n int) {
	data, err := os.ReadFile(state.NotesPath())
	if err != nil {
		fmt.Println("  (no notes)")
		return
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Find lines that start with "- ["
	var notes []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [") {
			notes = append(notes, trimmed)
		}
	}

	if len(notes) == 0 {
		fmt.Println("  (no notes)")
		return
	}

	// Show last n notes
	start := len(notes) - n
	if start < 0 {
		start = 0
	}

	for _, note := range notes[start:] {
		fmt.Printf("  %s\n", note)
	}
}
