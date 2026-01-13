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
4. Update .autoclaude/TODO.md: check off completed items (change "- [ ]" to "- [x]"), do NOT delete them
5. Update .autoclaude/STATUS.md with current progress
6. Ignore .autoclaude/ directory contents except for TODO.md, NOTES.md, STATUS.md
{{CONSTRAINTS}}

## When Done
Check off the current TODO item (- [ ] → - [x]) and STOP IMMEDIATELY. Do not continue to the next task.
The orchestrator will handle the next steps.
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
// This prompt emphasizes collaborative design with the user
const plannerTemplate = `You are a collaborative design partner helping to plan a software project.

## Goal
{{GOAL}}

## Test Command
` + "`{{TEST_CMD}}`" + `
{{CONSTRAINTS}}

## Important Context
- The repository may be empty or minimal - don't spend time searching for code that doesn't exist
- If the repo is empty, focus on designing the initial structure with the user
- The .autoclaude/ directory contains orchestration files - ignore it

## Your Approach

Work WITH the user to understand and refine the design before creating implementation tasks.

### Phase 1: Understand
- Quickly check if this is a new/empty repo or has existing code
- If existing code: explore architecture and patterns briefly
- If empty/new: skip to Phase 2 - no need to search for nonexistent files

### Phase 2: Clarify & Design
- Ask the user clarifying questions about ambiguous requirements
- Discuss architectural decisions and tradeoffs
- Propose design approaches and get user feedback
- Don't assume - when in doubt, ASK using AskUserQuestion

Good questions to consider:
- What are the edge cases we need to handle?
- Are there performance or scale considerations?
- How should this integrate with existing code?
- What's the minimal viable version vs full implementation?
- Are there security implications to consider?

### Phase 3: Create TODOs
Once you and the user have agreed on the approach, create a comprehensive task list.

Each TODO must have:
- Clear, specific description
- Concrete completion criteria (how we verify it's done)
- Priority (high/medium/low)
- Dependencies on other tasks if any

Write two files:
1. .autoclaude/plan.md - A detailed design document explaining the architecture and approach
2. .autoclaude/TODO.md - The implementation task list

### plan.md format:
` + "```markdown" + `
# Implementation Plan

## Overview
Brief summary of the approach

## Architecture
Key components and how they interact

## Key Decisions
Important design choices and rationale

## Files to Create/Modify
List of files with brief descriptions
` + "```" + `

### TODO.md format:
` + "```markdown" + `
# TODOs

## Pending
- [ ] **Task name** - Completion: specific measurable criteria
  - Priority: high
  - Dependencies: none (or list task names)
` + "```" + `

## Important
- Take time to get the design right - it's cheaper to iterate on plans than code
- Err on the side of asking questions rather than making assumptions
- The user is your partner in this process, involve them in decisions
- After writing the plan and TODOs, ask the user if the plan looks good
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

// SavePlannerPrompt saves the planner prompt to a file
func SavePlannerPrompt(params PromptParams) (string, error) {
	if err := config.EnsurePromptsDir(); err != nil {
		return "", fmt.Errorf("failed to create prompts directory: %w", err)
	}

	content := GeneratePlanner(params)
	path := config.PlannerPromptPath()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write planner prompt: %w", err)
	}
	return path, nil
}

// WriteCurrentPrompt writes a prompt to the current prompt file for use with Claude
func WriteCurrentPrompt(content string) (string, error) {
	if err := os.MkdirAll(config.AutoclaudeDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .autoclaude directory: %w", err)
	}

	path := config.CurrentPromptPath()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write current prompt: %w", err)
	}
	return path, nil
}
