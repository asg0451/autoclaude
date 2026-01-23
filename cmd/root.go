package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "autoclaude",
	Short: "Orchestrate Claude Code in a coder-critic loop",
	Long: `autoclaude orchestrates Claude Code in a coder→critic loop with TODO-based task tracking.

Commands:
  init     Set up prompts and initial TODOs for a project
  run      Start the coder→critic loop
  resume   Resume from last saved state after interruption
  status   Display current progress and state
  prune    Clean up and organize the TODO list
  watch    Watch progress with auto-refresh`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags can be added here
}
