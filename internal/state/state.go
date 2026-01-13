package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
}

const (
	AutoclaudeDir = ".autoclaude"
	StateFile     = "state.json"
	TodoFile      = "TODO.md"
	NotesFile     = "NOTES.md"
	StatusFile    = "STATUS.md"
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

	return nil
}

// UpdateStatus updates the STATUS.md file with current state
func (s *State) UpdateStatus(message string) error {
	content := fmt.Sprintf(`# Status

**Current Step:** %s (iteration %d/%d)
**Goal:** %s
**Test Command:** %s

## Latest Update
%s
`, s.Step, s.Iteration, s.MaxIterations, s.Goal, s.TestCmd, message)

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
