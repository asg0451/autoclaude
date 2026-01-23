package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.coldcutz.net/autoclaude/internal/claude"
	"go.coldcutz.net/autoclaude/internal/state"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate TODO.md to native task system (one-time)",
	Long: `Migrate existing TODO.md items to Claude Code's native task system.

This is a one-time migration for projects that were using the old TODO.md format.
After migration, TODO.md is backed up and the native task tools are used instead.`,
	RunE: runMigrate,
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

type parsedTodo struct {
	Subject  string
	Status   string // "pending" or "completed"
	Priority string
}

func runMigrate(cmd *cobra.Command, args []string) error {
	// Check if claude is installed
	if err := claude.CheckInstalled(); err != nil {
		return err
	}

	// Check if initialized
	if !state.Exists() {
		return fmt.Errorf("autoclaude not initialized. Run 'autoclaude init' first")
	}

	// Check for TODO.md
	data, err := os.ReadFile(state.TodoPath())
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No TODO.md found - nothing to migrate.")
			return nil
		}
		return fmt.Errorf("failed to read TODO.md: %w", err)
	}

	// Parse tasks
	todos := parseTodoMd(string(data))
	if len(todos) == 0 {
		fmt.Println("No tasks found in TODO.md - nothing to migrate.")
		return nil
	}

	fmt.Printf("Found %d tasks to migrate.\n", len(todos))

	// Generate migration prompt
	prompt := generateMigrationPrompt(todos)

	// Run Claude to create native tasks
	if err := claude.RunInteractive(prompt, "acceptEdits", ""); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Backup old file
	backupPath := state.TodoPath() + ".bak"
	if err := os.Rename(state.TodoPath(), backupPath); err != nil {
		fmt.Printf("Warning: could not backup TODO.md: %v\n", err)
	} else {
		fmt.Printf("Backed up TODO.md to %s\n", backupPath)
	}

	fmt.Printf("Migrated %d tasks to native task system.\n", len(todos))
	return nil
}

func parseTodoMd(content string) []parsedTodo {
	var todos []parsedTodo
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- [") {
			continue
		}

		status := "pending"
		if strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "- [X]") {
			status = "completed"
		}

		// Extract subject
		text := trimmed
		text = strings.TrimPrefix(text, "- [ ] ")
		text = strings.TrimPrefix(text, "- [x] ")
		text = strings.TrimPrefix(text, "- [X] ")

		// Check next line for priority
		priority := "medium"
		if i+1 < len(lines) {
			nextLine := strings.TrimSpace(lines[i+1])
			if strings.HasPrefix(nextLine, "- Priority:") {
				priority = strings.TrimSpace(strings.TrimPrefix(nextLine, "- Priority:"))
			}
		}

		todos = append(todos, parsedTodo{Subject: text, Status: status, Priority: priority})
	}
	return todos
}

func generateMigrationPrompt(todos []parsedTodo) string {
	var sb strings.Builder
	sb.WriteString("Migrate the following tasks to Claude Code's native task system.\n\n")
	sb.WriteString("For each task:\n")
	sb.WriteString("1. Use TaskCreate to create the task with the subject and description\n")
	sb.WriteString("2. For completed tasks, immediately use TaskUpdate to mark them as completed\n\n")
	sb.WriteString("Tasks to migrate:\n\n")

	pendingCount := 0
	for i, t := range todos {
		fmt.Fprintf(&sb, "%d. Subject: %s\n", i+1, t.Subject)
		fmt.Fprintf(&sb, "   Status: %s\n", t.Status)
		fmt.Fprintf(&sb, "   Priority: %s\n\n", t.Priority)
		if t.Status == "pending" {
			pendingCount++
		}
	}

	sb.WriteString("\nAfter creating all tasks:\n")
	if pendingCount > 0 {
		sb.WriteString("- Write 'yes' to .autoclaude/pending_tasks (there are pending tasks)\n")
	} else {
		sb.WriteString("- Write 'no' to .autoclaude/pending_tasks (all tasks are completed)\n")
	}
	sb.WriteString("- Then STOP immediately.\n")

	return sb.String()
}
