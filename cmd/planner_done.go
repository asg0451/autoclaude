package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// PlannerHookInput represents the input from Claude Code stop hook
type PlannerHookInput struct {
	SessionID       string `json:"session_id"`
	TranscriptPath  string `json:"transcript_path"`
	StopHookActive  bool   `json:"stop_hook_active"`
	StopReason      string `json:"stop_reason"`
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

	// Check if the planner is asking for confirmation (indicates plan is complete)
	// The planner ends with asking if the plan looks good
	if hookInput.LastAssistantMessage != "" {
		msg := strings.ToLower(hookInput.LastAssistantMessage)
		if strings.Contains(msg, "plan look") ||
		   strings.Contains(msg, "ready") ||
		   strings.Contains(msg, "proceed") ||
		   strings.Contains(msg, "adjust") {
			// Plan is complete, allow exit
			return outputPlannerAllow()
		}
	}

	// Default: allow exit
	return outputPlannerAllow()
}

// outputPlannerAllow outputs JSON to allow Claude to stop
func outputPlannerAllow() error {
	output := StopHookOutput{}
	data, _ := json.Marshal(output)
	fmt.Println(string(data))
	return nil
}
