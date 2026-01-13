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

## Context
- Read .autoclaude/plan.md for the overall architecture and design decisions
- Read .autoclaude/TODO.md for the task list
- Read .autoclaude/coding-guidelines.md for language-specific coding standards

Work on the highest priority incomplete item in TODO.md.

## Rules
1. Run tests after changes: ` + "`{{TEST_CMD}}`" + `
2. Do NOT declare success until tests pass
3. Commit ALL changes after completing each task: ` + "`git add . && git commit -m \"descriptive message\"`" + `
4. Update .autoclaude/TODO.md: check off completed items (change "- [ ]" to "- [x]"), do NOT delete them
5. Update .autoclaude/STATUS.md with current progress
6. ALWAYS use the Read and Write/Edit tools for file operations - NEVER use cat, echo, or heredocs to write files
{{CONSTRAINTS}}

## When Done
Check off the current TODO item (- [ ] → - [x]) and STOP IMMEDIATELY. Do not continue to the next task.
The orchestrator will handle the next steps.
`

// criticTemplate is the template for the critic prompt
const criticTemplate = `You are a code reviewer. Review the latest changes in this repository.

## Context
- Goal: {{GOAL}}
- Architecture: Read .autoclaude/plan.md for design decisions
- Standards: Read .autoclaude/coding-guidelines.md for language-specific requirements

## Review Checklist
1. Correctness: Does the code work as intended?
2. Tests: Are there adequate tests? Do they pass? Run: ` + "`{{TEST_CMD}}`" + `
3. Security: Any vulnerabilities introduced?
4. Edge cases: Are they handled?
5. Coding guidelines: Does the code follow .autoclaude/coding-guidelines.md?

## Important
ALWAYS use the Read and Write/Edit tools for file operations - NEVER use cat, echo, or heredocs to write files.

## Actions
After your review, write your verdict to .autoclaude/critic_verdict.md:

**If APPROVED** (code is correct, tests pass):
` + "```" + `
APPROVED

Brief summary of what was reviewed.
` + "```" + `

**If NEEDS_FIXES** (blocking issues - tests fail, bugs, security issues):
` + "```" + `
NEEDS_FIXES

## Issues
- Issue 1: detailed description
- Issue 2: detailed description

## Test Output (if relevant)
<paste failing test output here>

## Reproduction (if you created one)
If you wrote code/tests to reproduce the issue, include the file path here.
DO NOT delete reproduction code - keep it for the fixer to use.

## How to Fix
Specific instructions for the coder to fix these issues.
` + "```" + `

**If MINOR_ISSUES** (non-blocking - style, tech debt, nice-to-haves):
` + "```" + `
MINOR_ISSUES

Brief summary of minor issues found.
` + "```" + `
Then you MUST add each minor issue as a new TODO item to .autoclaude/TODO.md under "## Pending":
` + "```" + `
- [ ] **Fix: <issue description>** - Completion: <specific criteria>
  - Priority: low
` + "```" + `

Be thorough but pragmatic. NEEDS_FIXES is only for blocking issues that prevent the code from working correctly.
`

// fixerTemplate is the template for when coder needs to fix issues found by critic
const fixerTemplate = `You are fixing issues found during code review.

## IMPORTANT
Use the Read and Write/Edit tools for ALL file operations.
NEVER use cat, echo, heredocs, or shell redirection to write files.

## Context
- Goal: {{GOAL}}
- Architecture: Read .autoclaude/plan.md for design decisions
- Standards: Read .autoclaude/coding-guidelines.md for language-specific requirements
- Current TODO being fixed: {{CURRENT_TODO}}

## Critic Feedback
The critic found the following issues that must be fixed:

{{FIX_INSTRUCTIONS}}

Note: If the critic created reproduction code/tests to demonstrate the issue, those files still exist.
Use them to verify your fix works before committing.

## Rules
1. Fix ONLY the issues described above for the current TODO
2. Run tests after changes: ` + "`{{TEST_CMD}}`" + `
3. Do NOT declare success until tests pass
4. Do NOT move on to other TODOs - focus only on fixing these issues
5. Commit ALL changes with: ` + "`git add . && git commit -m \"descriptive message\"`" + `
6. ALWAYS use the Read and Write/Edit tools for file operations - NEVER use cat, echo, or heredocs to write files

## When Done
Once the issues are fixed and tests pass, STOP IMMEDIATELY.
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

## Important
ALWAYS use the Read and Write/Edit tools for file operations - NEVER use cat, echo, or heredocs to write files.

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
- For Go projects: What should the module name be? (e.g., github.com/user/project)
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
- ALWAYS use the Read and Write/Edit tools for file operations - NEVER use cat, echo, or heredocs to write files
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

// GenerateFixer generates the fixer prompt with critic feedback
func GenerateFixer(params PromptParams, fixInstructions string, currentTodo string) string {
	result := fixerTemplate
	result = strings.ReplaceAll(result, "{{GOAL}}", params.Goal)
	result = strings.ReplaceAll(result, "{{TEST_CMD}}", params.TestCmd)
	result = strings.ReplaceAll(result, "{{FIX_INSTRUCTIONS}}", fixInstructions)
	result = strings.ReplaceAll(result, "{{CURRENT_TODO}}", currentTodo)
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
