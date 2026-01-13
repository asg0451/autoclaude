package prompt

import (
	"fmt"
	"os"
	"strings"

	"go.coldcutz.net/autoclaude/internal/config"
)

// PromptParams holds parameters for generating prompts
type PromptParams struct {
	Goal        string
	TestCmd     string
	Constraints string
}

// coderTemplate is the template for the coder prompt
const coderTemplate = `You are working on: {{GOAL}}

## Current TODOs
Read .autoclaude/TODO.md and work on the highest priority incomplete item.

## Rules
1. Run tests after changes: ` + "`{{TEST_CMD}}`" + `
2. Do NOT declare success until tests pass
3. Commit after completing each task with a descriptive message
4. Update .autoclaude/TODO.md: mark items complete, add new items if discovered
5. Update .autoclaude/STATUS.md with current progress
{{CONSTRAINTS}}

## When Done
Mark the current TODO as complete and stop. The orchestrator will continue.
`

// criticTemplate is the template for the critic prompt
const criticTemplate = `You are a code reviewer. Review the latest changes in this repository.

## Goal Context
The overall goal is: {{GOAL}}

## Review Checklist
1. Correctness: Does the code work as intended?
2. Tests: Are there adequate tests? Do they pass? Run: ` + "`{{TEST_CMD}}`" + `
3. Security: Any vulnerabilities introduced?
4. Edge cases: Are they handled?

## Actions
- If correctness issues found: Describe specific fixes needed, then stop
- If minor issues/tech debt found: Add to .autoclaude/NOTES.md
- If changes are good: Say "APPROVED" and stop

Be thorough but pragmatic. Focus on correctness over style.
`

// evaluatorTemplate is the template for the evaluator prompt
const evaluatorTemplate = `You are evaluating if the goal is achieved.

## Goal
{{GOAL}}

## Test Command
` + "`{{TEST_CMD}}`" + `

## Instructions
1. Review .autoclaude/TODO.md - are all critical items complete?
2. Run the test suite: ` + "`{{TEST_CMD}}`" + `
3. Manually verify the goal is met by examining the implementation

## Actions
- If goal is fully achieved: Say "GOAL_COMPLETE" and stop
- If more work needed: Add new TODOs to .autoclaude/TODO.md and say "CONTINUING"
`

// plannerTemplate is the template for the initial planner (used during init)
const plannerTemplate = `You are planning a project. The goal is:

{{GOAL}}

Test command: ` + "`{{TEST_CMD}}`" + `
{{CONSTRAINTS}}

## Your Task
1. Analyze the codebase to understand current state
2. Ask clarifying questions using AskUserQuestion tool if scope is unclear
3. Break down the goal into specific, actionable TODOs
4. Each TODO must have:
   - Clear description
   - Completion criteria (how do we know it's done?)
   - Priority (high/medium/low)

Write the TODOs to .autoclaude/TODO.md in the format shown below, then stop.

## TODO Format
` + "```" + `
# TODOs

## Pending
- [ ] **Task name** - Completion: specific criteria
  - Priority: high|medium|low
` + "```" + `
`

// expandTemplate replaces template variables with values
func expandTemplate(template string, params PromptParams) string {
	result := template
	result = strings.ReplaceAll(result, "{{GOAL}}", params.Goal)
	result = strings.ReplaceAll(result, "{{TEST_CMD}}", params.TestCmd)

	constraintsSection := ""
	if params.Constraints != "" {
		constraintsSection = fmt.Sprintf("\n## Additional Constraints\n%s", params.Constraints)
	}
	result = strings.ReplaceAll(result, "{{CONSTRAINTS}}", constraintsSection)

	return result
}

// GenerateCoder generates the coder prompt
func GenerateCoder(params PromptParams) string {
	return expandTemplate(coderTemplate, params)
}

// GenerateCritic generates the critic prompt
func GenerateCritic(params PromptParams) string {
	return expandTemplate(criticTemplate, params)
}

// GenerateEvaluator generates the evaluator prompt
func GenerateEvaluator(params PromptParams) string {
	return expandTemplate(evaluatorTemplate, params)
}

// GeneratePlanner generates the planner prompt (for init)
func GeneratePlanner(params PromptParams) string {
	return expandTemplate(plannerTemplate, params)
}

// SavePrompts saves all prompts to the prompts directory
func SavePrompts(params PromptParams) error {
	if err := config.EnsurePromptsDir(); err != nil {
		return fmt.Errorf("failed to create prompts directory: %w", err)
	}

	// Save coder prompt
	coderContent := GenerateCoder(params)
	if err := os.WriteFile(config.CoderPromptPath(), []byte(coderContent), 0644); err != nil {
		return fmt.Errorf("failed to write coder prompt: %w", err)
	}

	// Save critic prompt
	criticContent := GenerateCritic(params)
	if err := os.WriteFile(config.CriticPromptPath(), []byte(criticContent), 0644); err != nil {
		return fmt.Errorf("failed to write critic prompt: %w", err)
	}

	// Save evaluator prompt
	evalContent := GenerateEvaluator(params)
	if err := os.WriteFile(config.EvaluatorPromptPath(), []byte(evalContent), 0644); err != nil {
		return fmt.Errorf("failed to write evaluator prompt: %w", err)
	}

	return nil
}

// LoadCoder loads the coder prompt from file
func LoadCoder() (string, error) {
	data, err := os.ReadFile(config.CoderPromptPath())
	if err != nil {
		return "", fmt.Errorf("failed to read coder prompt: %w", err)
	}
	return string(data), nil
}

// LoadCritic loads the critic prompt from file
func LoadCritic() (string, error) {
	data, err := os.ReadFile(config.CriticPromptPath())
	if err != nil {
		return "", fmt.Errorf("failed to read critic prompt: %w", err)
	}
	return string(data), nil
}

// LoadEvaluator loads the evaluator prompt from file
func LoadEvaluator() (string, error) {
	data, err := os.ReadFile(config.EvaluatorPromptPath())
	if err != nil {
		return "", fmt.Errorf("failed to read evaluator prompt: %w", err)
	}
	return string(data), nil
}
