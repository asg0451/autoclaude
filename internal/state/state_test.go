package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewState(t *testing.T) {
	s := NewState("test goal", "go test", "no constraints", 5)

	if s.Goal != "test goal" {
		t.Errorf("expected goal 'test goal', got %q", s.Goal)
	}
	if s.TestCmd != "go test" {
		t.Errorf("expected testCmd 'go test', got %q", s.TestCmd)
	}
	if s.Constraints != "no constraints" {
		t.Errorf("expected constraints 'no constraints', got %q", s.Constraints)
	}
	if s.MaxIterations != 5 {
		t.Errorf("expected maxIterations 5, got %d", s.MaxIterations)
	}
	if s.Step != StepCoder {
		t.Errorf("expected step StepCoder, got %q", s.Step)
	}
	if s.Iteration != 1 {
		t.Errorf("expected iteration 1, got %d", s.Iteration)
	}
}

func TestStateSaveLoad(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// Create .autoclaude directory
	os.MkdirAll(AutoclaudeDir, 0755)

	s := NewState("save test", "make test", "", 3)
	s.Iteration = 2
	s.Step = StepCritic

	if err := s.Save(); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if loaded.Goal != s.Goal {
		t.Errorf("loaded goal %q != saved goal %q", loaded.Goal, s.Goal)
	}
	if loaded.Step != s.Step {
		t.Errorf("loaded step %q != saved step %q", loaded.Step, s.Step)
	}
	if loaded.Iteration != s.Iteration {
		t.Errorf("loaded iteration %d != saved iteration %d", loaded.Iteration, s.Iteration)
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	if Exists() {
		t.Error("expected Exists() to return false before creating state")
	}

	os.MkdirAll(AutoclaudeDir, 0755)
	s := NewState("test", "test", "", 1)
	s.Save()

	if !Exists() {
		t.Error("expected Exists() to return true after creating state")
	}
}

func TestGetNextTodo(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	os.MkdirAll(AutoclaudeDir, 0755)

	todoContent := `# TODOs

## Pending
- [x] Completed task
- [ ] First incomplete task
- [ ] Second incomplete task
`
	os.WriteFile(TodoPath(), []byte(todoContent), 0644)

	next := GetNextTodo()
	if next != "First incomplete task" {
		t.Errorf("expected 'First incomplete task', got %q", next)
	}
}

func TestGetNextTodoEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	os.MkdirAll(AutoclaudeDir, 0755)

	todoContent := `# TODOs

## Completed
- [x] All done
`
	os.WriteFile(TodoPath(), []byte(todoContent), 0644)

	next := GetNextTodo()
	if next != "(no incomplete TODOs)" {
		t.Errorf("expected '(no incomplete TODOs)', got %q", next)
	}
}

func TestCurrentTodo(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	os.MkdirAll(AutoclaudeDir, 0755)

	SetCurrentTodo("Test TODO item")
	got := GetCurrentTodo()
	if got != "Test TODO item" {
		t.Errorf("expected 'Test TODO item', got %q", got)
	}

	ClearCurrentTodo()
	got = GetCurrentTodo()
	if got != "(unknown)" {
		t.Errorf("expected '(unknown)' after clear, got %q", got)
	}
}

func TestCriticVerdict(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	os.MkdirAll(AutoclaudeDir, 0755)

	tests := []struct {
		content  string
		expected CriticVerdict
	}{
		{"APPROVED\n\nLooks good!", VerdictApproved},
		{"NEEDS_FIXES\n\n## Issues\n- Bug found", VerdictNeedsFixes},
		{"MINOR_ISSUES\n\nSome style issues", VerdictMinorIssues},
		{"Something else", VerdictUnknown},
	}

	for _, tt := range tests {
		os.WriteFile(CriticVerdictPath(), []byte(tt.content), 0644)
		verdict, _ := GetCriticVerdict()
		if verdict != tt.expected {
			t.Errorf("for content %q, expected %q, got %q", tt.content[:10], tt.expected, verdict)
		}
	}

	ClearCriticVerdict()
	verdict, _ := GetCriticVerdict()
	if verdict != VerdictUnknown {
		t.Errorf("expected VerdictUnknown after clear, got %q", verdict)
	}
}

func TestInitDir(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	err := InitDir("test goal", "go test ./...")
	if err != nil {
		t.Fatalf("InitDir failed: %v", err)
	}

	// Check that files were created
	files := []string{
		TodoPath(),
		NotesPath(),
		StatusPath(),
	}

	for _, f := range files {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}

	// Check STATUS.md contains goal
	data, _ := os.ReadFile(StatusPath())
	if !contains(string(data), "test goal") {
		t.Error("STATUS.md should contain the goal")
	}
}

func TestUpdateStatus(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	os.MkdirAll(AutoclaudeDir, 0755)

	s := NewState("status test", "make test", "", 3)
	s.Iteration = 2
	s.Step = StepCritic
	s.RetryCount = 1

	SetCurrentTodo("Current task")

	err := s.UpdateStatus("Running tests...")
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	data, _ := os.ReadFile(StatusPath())
	content := string(data)

	if !contains(content, "critic") {
		t.Error("STATUS.md should contain step name")
	}
	if !contains(content, "TODO #:") {
		t.Error("STATUS.md should contain TODO number")
	}
	if !contains(content, "Current task") {
		t.Error("STATUS.md should contain current todo")
	}
	if !contains(content, "Running tests...") {
		t.Error("STATUS.md should contain update message")
	}
}

func TestIncrementIteration(t *testing.T) {
	s := NewState("test", "test", "", 3)
	s.Iteration = 1

	if !s.IncrementIteration() {
		t.Error("should be able to increment from 1 to 2")
	}
	if s.Iteration != 2 {
		t.Errorf("expected iteration 2, got %d", s.Iteration)
	}

	s.IncrementIteration() // 3
	if s.IncrementIteration() {
		t.Error("should not be able to increment past max")
	}
}

func contains(s, substr string) bool {
	return filepath.Clean(s) != "" && len(s) > 0 && len(substr) > 0 &&
		(len(s) >= len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
