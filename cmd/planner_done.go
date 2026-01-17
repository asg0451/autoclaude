package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"go.coldcutz.net/autoclaude/internal/claude"
	"go.coldcutz.net/autoclaude/internal/config"
)

// PlannerHookInput represents the input from Claude Code stop hook
type PlannerHookInput struct {
	SessionID            string `json:"session_id"`
	TranscriptPath       string `json:"transcript_path"`
	StopHookActive       bool   `json:"stop_hook_active"`
	StopReason           string `json:"stop_reason"`
	LastAssistantMessage string `json:"last_assistant_message"`
}

var plannerDoneCmd = &cobra.Command{
	Use:    "_planner-done",
	Short:  "Internal: called by stop hook when planner stops",
	Hidden: true,
	RunE:   runPlannerDone,
}

func init() {
	rootCmd.AddCommand(plannerDoneCmd)
}

func runPlannerDone(cmd *cobra.Command, args []string) error {
	// Read stop hook input from stdin
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return outputPlannerAllow()
	}

	var hookInput PlannerHookInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &hookInput); err != nil {
			// Not fatal, might not have input
		}
	}

	// Prevent infinite loops
	if hookInput.StopHookActive {
		return outputPlannerAllow()
	}

	// Only kill Claude if planning is complete (user confirmed the plan)
	// If the file doesn't exist, let Claude continue working with the user
	if config.IsPlanningComplete() {
		claude.KillClaude()
	}

	return outputPlannerAllow()
}

// outputPlannerAllow outputs JSON to allow Claude to stop
func outputPlannerAllow() error {
	output := StopHookOutput{}
	data, _ := json.Marshal(output)
	fmt.Println(string(data))
	return nil
}
