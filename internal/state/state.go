package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Step represents the current step in the coder-critic loop
type Step string

const (
	StepCoder     Step = "coder"
	StepCritic    Step = "critic"
	StepEvaluator Step = "evaluator"
	StepDone      Step = "done"
)

// State holds the current loop state
type State struct {
	Step          Step   `json:"step"`
	Iteration     int    `json:"iteration"`
	MaxIterations int    `json:"maxIterations"`
	Goal          string `json:"goal"`
	TestCmd       string `json:"testCmd"`
	Constraints   string `json:"constraints,omitempty"`
	LastCommit    string `json:"lastCommit,omitempty"`
	RetryCount    int    `json:"retryCount,omitempty"`
	LastError     string `json:"lastError,omitempty"`
	Stats         *Stats `json:"stats,omitempty"`
}

// Stats tracks diagnostic information about the run
type Stats struct {
	ClaudeRuns      int `json:"claudeRuns"`      // Total Claude invocations
	TodosCompleted  int `json:"todosCompleted"`  // TODOs successfully completed
	TodosAttempted  int `json:"todosAttempted"`  // TODOs attempted
	CriticApprovals int `json:"criticApprovals"` // Times critic said APPROVED
	CriticRejections int `json:"criticRejections"` // Times critic said NEEDS_FIXES
	CriticMinor     int `json:"criticMinor"`     // Times critic said MINOR_ISSUES
	FixAttempts     int `json:"fixAttempts"`     // Number of fix attempts
	FixSuccesses    int `json:"fixSuccesses"`    // Fixes that led to approval
}

const (
	AutoclaudeDir     = ".autoclaude"
	StateFile         = "state.json"
	TodoFile          = "TODO.md"
	NotesFile         = "NOTES.md"
	StatusFile        = "STATUS.md"
	CriticVerdictFile = "critic_verdict.md"
	CurrentTodoFile   = "current_todo.txt"
)

// StateDir returns the path to the .autoclaude directory
func StateDir() string {
	return AutoclaudeDir
}

// StatePath returns the path to the state.json file
func StatePath() string {
	return filepath.Join(AutoclaudeDir, StateFile)
}

// TodoPath returns the path to the TODO.md file
func TodoPath() string {
	return filepath.Join(AutoclaudeDir, TodoFile)
}

// NotesPath returns the path to the NOTES.md file
func NotesPath() string {
	return filepath.Join(AutoclaudeDir, NotesFile)
}

// StatusPath returns the path to the STATUS.md file
func StatusPath() string {
	return filepath.Join(AutoclaudeDir, StatusFile)
}

// CriticVerdictPath returns the path to the critic_verdict.md file
func CriticVerdictPath() string {
	return filepath.Join(AutoclaudeDir, CriticVerdictFile)
}

// CriticVerdict represents the critic's decision
type CriticVerdict string

const (
	VerdictApproved    CriticVerdict = "APPROVED"
	VerdictNeedsFixes  CriticVerdict = "NEEDS_FIXES"
	VerdictMinorIssues CriticVerdict = "MINOR_ISSUES"
	VerdictUnknown     CriticVerdict = "UNKNOWN"
)

// GetCriticVerdict reads and parses the critic verdict file
// Returns the verdict and the full content (for fix instructions)
func GetCriticVerdict() (CriticVerdict, string) {
	data, err := os.ReadFile(CriticVerdictPath())
	if err != nil {
		return VerdictUnknown, ""
	}
	content := string(data)

	if len(content) >= 11 && content[:11] == "NEEDS_FIXES" {
		return VerdictNeedsFixes, content
	}
	if len(content) >= 12 && content[:12] == "MINOR_ISSUES" {
		return VerdictMinorIssues, content
	}
	if len(content) >= 8 && content[:8] == "APPROVED" {
		return VerdictApproved, content
	}
	return VerdictUnknown, content
}

// ClearCriticVerdict removes the critic verdict file
func ClearCriticVerdict() {
	os.Remove(CriticVerdictPath())
}

// CurrentTodoPath returns the path to the current_todo.txt file
func CurrentTodoPath() string {
	return filepath.Join(AutoclaudeDir, CurrentTodoFile)
}

// GetNextTodo returns the first incomplete TODO item from TODO.md
func GetNextTodo() string {
	data, err := os.ReadFile(TodoPath())
	if err != nil {
		return "(unknown)"
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") {
			// Return the TODO text without the checkbox
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "- [ ]"))
		}
	}
	return "(no incomplete TODOs)"
}

// SetCurrentTodo saves the current TODO being worked on to a file
func SetCurrentTodo(todo string) {
	os.WriteFile(CurrentTodoPath(), []byte(todo), 0644)
}

// GetCurrentTodo returns the TODO currently being worked on (from file)
func GetCurrentTodo() string {
	data, err := os.ReadFile(CurrentTodoPath())
	if err != nil {
		return "(unknown)"
	}
	return strings.TrimSpace(string(data))
}

// ClearCurrentTodo removes the current todo file
func ClearCurrentTodo() {
	os.Remove(CurrentTodoPath())
}

// NewState creates a new state with default values
func NewState(goal, testCmd, constraints string, maxIterations int) *State {
	return &State{
		Step:          StepCoder,
		Iteration:     1,
		MaxIterations: maxIterations,
		Goal:          goal,
		TestCmd:       testCmd,
		Constraints:   constraints,
	}
}

// Load loads state from the state file
func Load() (*State, error) {
	data, err := os.ReadFile(StatePath())
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &s, nil
}

// Save saves the state to the state file
func (s *State) Save() error {
	if err := os.MkdirAll(AutoclaudeDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(StatePath(), data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// Exists checks if a state file exists
func Exists() bool {
	_, err := os.Stat(StatePath())
	return err == nil
}

// InitDir creates the .autoclaude directory structure with initial files
func InitDir(goal, testCmd string) error {
	if err := os.MkdirAll(AutoclaudeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .autoclaude directory: %w", err)
	}

	// Create TODO.md
	todoContent := `# TODOs

## In Progress

## Pending

## Completed
`
	if err := os.WriteFile(TodoPath(), []byte(todoContent), 0644); err != nil {
		return fmt.Errorf("failed to create TODO.md: %w", err)
	}

	// Create NOTES.md
	notesContent := `# Notes

Technical debt, observations, and other notes from the critic.
`
	if err := os.WriteFile(NotesPath(), []byte(notesContent), 0644); err != nil {
		return fmt.Errorf("failed to create NOTES.md: %w", err)
	}

	// Create STATUS.md
	statusContent := fmt.Sprintf(`# Status

**Current Step:** initializing
**Goal:** %s
**Test Command:** %s

## Latest Update
Initialized autoclaude. Ready to run.
`, goal, testCmd)
	if err := os.WriteFile(StatusPath(), []byte(statusContent), 0644); err != nil {
		return fmt.Errorf("failed to create STATUS.md: %w", err)
	}

	// Create .claudeignore to prevent Claude from reading autoclaude internals
	claudeignoreContent := `# Ignore autoclaude internal files - these are for orchestration only
.autoclaude/
`
	claudeignorePath := ".claudeignore"
	// Only create if it doesn't exist, or append if it does
	if _, err := os.Stat(claudeignorePath); os.IsNotExist(err) {
		if err := os.WriteFile(claudeignorePath, []byte(claudeignoreContent), 0644); err != nil {
			return fmt.Errorf("failed to create .claudeignore: %w", err)
		}
	}

	return nil
}

// UpdateStatus updates the STATUS.md file with current state
func (s *State) UpdateStatus(message string) error {
	currentTodo := GetCurrentTodo()
	todoSection := ""
	if currentTodo != "(unknown)" && currentTodo != "" {
		todoSection = fmt.Sprintf("**Current TODO:** %s\n", currentTodo)
	}

	retryInfo := ""
	if s.Step == StepCritic && s.RetryCount > 0 {
		retryInfo = fmt.Sprintf(" (attempt %d/%d)", s.RetryCount+1, 3)
	}

	content := fmt.Sprintf(`# Status

**Current Step:** %s%s
**TODO #:** %d
%s**Goal:** %s
**Test Command:** %s

## Latest Update
%s
`, s.Step, retryInfo, s.Iteration, todoSection, s.Goal, s.TestCmd, message)

	return os.WriteFile(StatusPath(), []byte(content), 0644)
}

// NextStep advances to the next step in the state machine
func (s *State) NextStep(approved bool, fixInstructions string) Step {
	switch s.Step {
	case StepCoder:
		s.Step = StepCritic
		s.RetryCount = 0
	case StepCritic:
		if approved {
			// Check if more TODOs remain (caller should check TODO.md)
			// For now, assume we go back to coder or to evaluator
			s.Step = StepCoder
		} else {
			// Critic rejected, go back to coder with fixes
			s.Step = StepCoder
			s.LastError = fixInstructions
		}
	case StepEvaluator:
		// Evaluator decides if done or more work needed
		// Caller handles this based on evaluator output
		s.Step = StepCoder
	}
	return s.Step
}

// IncrementIteration increments the iteration counter
func (s *State) IncrementIteration() bool {
	s.Iteration++
	return s.Iteration <= s.MaxIterations
}

// ShouldRetry checks if we should retry after a failure
func (s *State) ShouldRetry() bool {
	return s.RetryCount < 1
}

// RecordRetry records a retry attempt
func (s *State) RecordRetry(err string) {
	s.RetryCount++
	s.LastError = err
}
