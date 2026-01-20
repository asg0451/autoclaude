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

// EvaluatorHookInput represents the input from Claude Code stop hook
type EvaluatorHookInput struct {
	SessionID            string `json:"session_id"`
	TranscriptPath       string `json:"transcript_path"`
	StopHookActive       bool   `json:"stop_hook_active"`
	StopReason           string `json:"stop_reason"`
	LastAssistantMessage string `json:"last_assistant_message"`
}

var evaluatorDoneCmd = &cobra.Command{
	Use:    "_evaluator-done",
	Short:  "Internal: called by stop hook when evaluator stops",
	Hidden: true,
	RunE:   runEvaluatorDone,
}

func init() {
	rootCmd.AddCommand(evaluatorDoneCmd)
}

func runEvaluatorDone(cmd *cobra.Command, args []string) error {
	// Read stop hook input from stdin
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return outputEvaluatorAllow()
	}

	var hookInput EvaluatorHookInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &hookInput); err != nil {
			// Not fatal, might not have input
		}
	}

	// Prevent infinite loops
	if hookInput.StopHookActive {
		return outputEvaluatorAllow()
	}

	// Only kill Claude if evaluation is complete (user confirmed)
	// If the file doesn't exist, let Claude continue working with the user
	if config.IsEvaluationComplete() {
		claude.KillClaude()
	}

	return outputEvaluatorAllow()
}

// outputEvaluatorAllow outputs JSON to allow Claude to stop
func outputEvaluatorAllow() error {
	output := StopHookOutput{}
	data, _ := json.Marshal(output)
	fmt.Println(string(data))
	return nil
}
