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
3. Commit ALL changes (including .autoclaude/) after each task: ` + "`git add . && git commit -m \"message\"`" + ` - the dot means EVERYTHING
4. Update .autoclaude/TODO.md: check off completed items (change "- [ ]" to "- [x]"), do NOT delete them
5. Update .autoclaude/STATUS.md with current progress
6. ALWAYS use the Read and Write/Edit tools for file operations - NEVER use cat, echo, or heredocs to write files
7. AVOID using awk - it triggers an unskippable permissions check
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
AVOID using awk - it triggers an unskippable permissions check.

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
AVOID using awk - it triggers an unskippable permissions check.

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
5. Commit ALL changes (including .autoclaude/) with: ` + "`git add . && git commit -m \"message\"`" + ` - the dot means EVERYTHING
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
AVOID using awk - it triggers an unskippable permissions check.

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

You are a design partner, not just a task executor. Your job is to deeply understand what the user wants before writing any code. Ask MANY questions. Have a real conversation. The more you understand upfront, the better the implementation will be.

### Phase 1: Understand the Codebase
- Quickly check if this is a new/empty repo or has existing code
- If existing code: explore architecture, patterns, and conventions
- If empty/new: skip to Phase 2

### Phase 2: Deep Discovery (THIS IS THE MOST IMPORTANT PHASE)

Before proposing ANY solution, have a thorough conversation with the user. Ask questions across multiple rounds - don't try to ask everything at once. Build understanding incrementally.

**Requirements & Scope:**
- What problem are we actually solving? What's the pain point?
- Who are the users? What are their skill levels?
- What does success look like? How will we know we're done?
- What's explicitly OUT of scope?
- Are there existing solutions? Why aren't they sufficient?
- What's the timeline/urgency? MVP vs polished?

**Technical Decisions:**
- What languages/frameworks are preferred and why?
- Are there existing patterns in the codebase we should follow?
- What are the performance requirements? Expected load/scale?
- What environments will this run in? (local, cloud, containers, etc.)
- What dependencies are acceptable? Any we should avoid?
- How should errors be handled? Logging? Monitoring?

**Data & State:**
- What data do we need to store? For how long?
- What's the source of truth? Where does data come from?
- Are there consistency requirements? Transactions?
- What happens if data is lost or corrupted?

**Integration & Interfaces:**
- What will interact with this? APIs? CLI? UI? Other services?
- What input formats do we need to support?
- What output formats are expected?
- Are there existing APIs or contracts we need to conform to?
- Authentication/authorization requirements?

**Edge Cases & Error Handling:**
- What happens when things go wrong?
- What are the failure modes? How do we recover?
- What inputs might be malformed or malicious?
- What if external services are unavailable?

**Testing & Quality:**
- What testing approach? Unit? Integration? E2E?
- Are there specific scenarios that MUST work?
- What's the bar for code quality? Linting? Type safety?

**Deployment & Operations:**
- How will this be deployed?
- Configuration management? Environment variables? Files?
- How do we handle upgrades? Backwards compatibility?
- Observability needs? Metrics? Tracing?

**User Experience (if applicable):**
- What should the happy path feel like?
- What feedback should users get during operations?
- How do we communicate errors to users?
- Are there accessibility requirements?

Don't ask ALL of these - pick the ones relevant to this specific project. But DO ask multiple rounds of questions. After each answer, you may have follow-up questions. That's good! Keep digging until you truly understand.

Use the AskUserQuestion tool liberally - it's your primary way to have this conversation.

### Phase 3: Propose & Iterate

Once you understand the requirements:
- Propose a high-level approach
- Explain your reasoning and tradeoffs
- ASK if this matches their expectations
- Be ready to revise based on feedback
- Discuss alternatives if the user has concerns

### Phase 4: Create TODOs

ONLY after the user has approved the approach, create the implementation plan.

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
- AVOID using awk - it triggers an unskippable permissions check

## When Planning is Complete
After the user confirms the plan is good:
1. Write the file ` + "`.autoclaude/planning_complete`" + ` with content "done"
2. Exit immediately - the orchestrator will take over from here
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
