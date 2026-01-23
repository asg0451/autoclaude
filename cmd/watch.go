package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.coldcutz.net/autoclaude/internal/state"
)

var watchInterval int

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch autoclaude progress with auto-refresh",
	Long: `Display the current status of autoclaude with auto-refresh.
Shows the current step and iteration.
Refreshes automatically every n seconds (default: 2).`,
	RunE: runWatch,
}

func init() {
	rootCmd.AddCommand(watchCmd)
	watchCmd.Flags().IntVarP(&watchInterval, "interval", "i", 2, "Refresh interval in seconds")
}

func runWatch(cmd *cobra.Command, args []string) error {
	if !state.Exists() {
		return fmt.Errorf("autoclaude not initialized. Run 'autoclaude init' first")
	}

	// Validate interval
	if watchInterval < 1 {
		return fmt.Errorf("interval must be at least 1 second")
	}

	interval := time.Duration(watchInterval) * time.Second

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Clear screen once at start
	fmt.Print("\033[H\033[2J")

	// Initial display
	if err := displayWatch(ctx); err != nil {
		return err
	}

	// Start refresh ticker
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := displayWatch(ctx); err != nil {
				return err
			}
		case <-ctx.Done():
			fmt.Println("\nWatch stopped.")
			return nil
		}
	}
}

func displayWatch(_ context.Context) error {
	// Clear screen and move cursor to top-left
	fmt.Print("\033[H\033[2J")

	// Load state
	s, err := state.Load()
	if err != nil {
		fmt.Printf("Error loading state: %v\n", err)
		return nil
	}

	// Header with timestamp
	now := time.Now().Format("2006-01-02 15:04:05")
	fmt.Println("=== autoclaude Watch ===")
	fmt.Printf("Updated: %s (refreshing every %ds)\n", now, watchInterval)
	fmt.Println()

	// Current step and status
	stepColor := stepColor(s.Step)
	fmt.Printf("Step:       %s%s\033[0m\n", stepColor, s.Step)
	fmt.Printf("Iteration:  %d/%d\n", s.Iteration, s.MaxIterations)

	// Check pending tasks status
	pendingTasksData, err := os.ReadFile(state.PendingTasksPath())
	if err != nil {
		fmt.Println("\n► Pending tasks: unknown")
	} else {
		pending := strings.TrimSpace(string(pendingTasksData))
		if pending == "yes" {
			fmt.Println("\n\033[1;33m► Pending tasks: yes\033[0m")
		} else {
			fmt.Println("\n\033[1;32m► Pending tasks: no\033[0m")
		}
	}

	// Stats footer if available
	if s.Stats != nil {
		fmt.Println()
		fmt.Println("=== Stats ===")
		if s.Stats.ClaudeRuns > 0 {
			fmt.Printf("Claude runs: %d\n", s.Stats.ClaudeRuns)
		}
		if s.Stats.CriticApprovals > 0 || s.Stats.CriticRejections > 0 {
			fmt.Printf("Critic: %d approvals, %d rejections\n",
				s.Stats.CriticApprovals, s.Stats.CriticRejections)
		}
	}

	// Instructions at bottom
	fmt.Println()
	fmt.Println("\033[90mPress Ctrl+C to exit\033[0m")

	return nil
}

func stepColor(step state.Step) string {
	switch step {
	case state.StepCoder:
		return "\033[1;36m" // cyan
	case state.StepCritic:
		return "\033[1;35m" // magenta
	case state.StepEvaluator:
		return "\033[1;33m" // yellow
	case state.StepDone:
		return "\033[1;32m" // green
	default:
		return "\033[1;37m" // white
	}
}
