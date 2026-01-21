package prompt

import (
	"os"
	"strings"
	"testing"

	"go.coldcutz.net/autoclaude/internal/config"
)

func TestGenerateCoder(t *testing.T) {
	params := PromptParams{
		Goal:        "Build a web server",
		TestCmd:     "go test ./...",
		Constraints: "Must be production ready",
	}

	content := GenerateCoder(params)

	if !strings.Contains(content, "Build a web server") {
		t.Error("coder prompt should contain goal")
	}
	if !strings.Contains(content, "go test ./...") {
		t.Error("coder prompt should contain test command")
	}
	if !strings.Contains(content, "Must be production ready") {
		t.Error("coder prompt should contain constraints")
	}
	if !strings.Contains(content, "TODO.md") {
		t.Error("coder prompt should reference TODO.md")
	}
	if !strings.Contains(content, "git add .") {
		t.Error("coder prompt should mention git add .")
	}
	if !strings.Contains(content, "coding-guidelines.md") {
		t.Error("coder prompt should reference coding-guidelines.md")
	}
}

func TestGenerateCoderNoConstraints(t *testing.T) {
	params := PromptParams{
		Goal:    "Simple goal",
		TestCmd: "make test",
	}

	content := GenerateCoder(params)

	if strings.Contains(content, "Additional Constraints") {
		t.Error("coder prompt should not have constraints section when empty")
	}
}

func TestGenerateCritic(t *testing.T) {
	params := PromptParams{
		Goal:    "Build API",
		TestCmd: "npm test",
	}

	content := GenerateCritic(params)

	if !strings.Contains(content, "Build API") {
		t.Error("critic prompt should contain goal")
	}
	if !strings.Contains(content, "npm test") {
		t.Error("critic prompt should contain test command")
	}
	if !strings.Contains(content, "APPROVED") {
		t.Error("critic prompt should mention APPROVED verdict")
	}
	if !strings.Contains(content, "NEEDS_FIXES") {
		t.Error("critic prompt should mention NEEDS_FIXES verdict")
	}
	if !strings.Contains(content, "MINOR_ISSUES") {
		t.Error("critic prompt should mention MINOR_ISSUES verdict")
	}
	if !strings.Contains(content, "critic_verdict.md") {
		t.Error("critic prompt should reference verdict file")
	}
	if !strings.Contains(content, "coding-guidelines.md") {
		t.Error("critic prompt should reference coding-guidelines.md")
	}
}

func TestGenerateFixer(t *testing.T) {
	params := PromptParams{
		Goal:    "Fix bugs",
		TestCmd: "pytest",
	}
	fixInstructions := "The function crashes on null input"
	currentTodo := "Handle null input in parser"

	content := GenerateFixer(params, fixInstructions, currentTodo)

	if !strings.Contains(content, "Fix bugs") {
		t.Error("fixer prompt should contain goal")
	}
	if !strings.Contains(content, "pytest") {
		t.Error("fixer prompt should contain test command")
	}
	if !strings.Contains(content, "The function crashes on null input") {
		t.Error("fixer prompt should contain fix instructions")
	}
	if !strings.Contains(content, "Handle null input in parser") {
		t.Error("fixer prompt should contain current todo")
	}
	if !strings.Contains(content, "CRITICAL") {
		t.Error("fixer prompt should have CRITICAL section")
	}
	if !strings.Contains(content, "git add .") {
		t.Error("fixer prompt should mention git add .")
	}
}

func TestGenerateEvaluator(t *testing.T) {
	params := PromptParams{
		Goal:    "Complete project",
		TestCmd: "cargo test",
	}

	content := GenerateEvaluator(params)

	if !strings.Contains(content, "Complete project") {
		t.Error("evaluator prompt should contain goal")
	}
	if !strings.Contains(content, "cargo test") {
		t.Error("evaluator prompt should contain test command")
	}
	if !strings.Contains(content, "evaluation_complete") {
		t.Error("evaluator prompt should mention evaluation_complete marker file")
	}
	if !strings.Contains(content, "AskUserQuestion") {
		t.Error("evaluator prompt should mention AskUserQuestion")
	}
}

func TestGeneratePlanner(t *testing.T) {
	params := PromptParams{
		Goal:        "Design system",
		TestCmd:     "make test",
		Constraints: "Use microservices",
	}

	content := GeneratePlanner(params)

	if !strings.Contains(content, "Design system") {
		t.Error("planner prompt should contain goal")
	}
	if !strings.Contains(content, "make test") {
		t.Error("planner prompt should contain test command")
	}
	if !strings.Contains(content, "Use microservices") {
		t.Error("planner prompt should contain constraints")
	}
	if !strings.Contains(content, "plan.md") {
		t.Error("planner prompt should mention plan.md")
	}
	if !strings.Contains(content, "TODO.md") {
		t.Error("planner prompt should mention TODO.md")
	}
	if !strings.Contains(content, "AskUserQuestion") {
		t.Error("planner prompt should mention AskUserQuestion")
	}
}

func TestSavePrompts(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	params := PromptParams{
		Goal:    "Test save",
		TestCmd: "test",
	}

	err := SavePrompts(params)
	if err != nil {
		t.Fatalf("SavePrompts failed: %v", err)
	}

	// Check files exist
	files := []string{
		config.CoderPromptPath(),
		config.CriticPromptPath(),
		config.EvaluatorPromptPath(),
	}

	for _, f := range files {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}

	// Check content
	data, _ := os.ReadFile(config.CoderPromptPath())
	if !strings.Contains(string(data), "Test save") {
		t.Error("coder prompt file should contain goal")
	}
}

func TestLoadPrompts(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	params := PromptParams{
		Goal:    "Load test",
		TestCmd: "echo test",
	}

	SavePrompts(params)

	coder, err := LoadCoder()
	if err != nil {
		t.Fatalf("LoadCoder failed: %v", err)
	}
	if !strings.Contains(coder, "Load test") {
		t.Error("loaded coder should contain goal")
	}

	critic, err := LoadCritic()
	if err != nil {
		t.Fatalf("LoadCritic failed: %v", err)
	}
	if !strings.Contains(critic, "Load test") {
		t.Error("loaded critic should contain goal")
	}

	evaluator, err := LoadEvaluator()
	if err != nil {
		t.Fatalf("LoadEvaluator failed: %v", err)
	}
	if !strings.Contains(evaluator, "Load test") {
		t.Error("loaded evaluator should contain goal")
	}
}

func TestSavePlannerPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	params := PromptParams{
		Goal:    "Planner test",
		TestCmd: "test",
	}

	path, err := SavePlannerPrompt(params)
	if err != nil {
		t.Fatalf("SavePlannerPrompt failed: %v", err)
	}

	if path != config.PlannerPromptPath() {
		t.Errorf("unexpected path: %s", path)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "Planner test") {
		t.Error("planner prompt file should contain goal")
	}
}

func TestWriteCurrentPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	content := "This is the current prompt content"

	path, err := WriteCurrentPrompt(content)
	if err != nil {
		t.Fatalf("WriteCurrentPrompt failed: %v", err)
	}

	if path != config.CurrentPromptPath() {
		t.Errorf("unexpected path: %s", path)
	}

	data, _ := os.ReadFile(path)
	if string(data) != content {
		t.Error("current prompt file should have exact content")
	}
}

func TestPromptContainsFileToolWarnings(t *testing.T) {
	params := PromptParams{Goal: "test", TestCmd: "test"}

	prompts := []struct {
		name    string
		content string
	}{
		{"coder", GenerateCoder(params)},
		{"critic", GenerateCritic(params)},
		{"fixer", GenerateFixer(params, "fix", "todo")},
		{"evaluator", GenerateEvaluator(params)},
		{"planner", GeneratePlanner(params)},
	}

	for _, p := range prompts {
		if !strings.Contains(p.content, "NEVER use cat") {
			t.Errorf("%s prompt should warn against using cat", p.name)
		}
		if !strings.Contains(p.content, "awk") {
			t.Errorf("%s prompt should warn against using awk", p.name)
		}
	}
}

func TestCoderPromptCommitInstructions(t *testing.T) {
	params := PromptParams{Goal: "test", TestCmd: "test"}
	content := GenerateCoder(params)

	if !strings.Contains(content, ".autoclaude/") {
		t.Error("coder should mention committing .autoclaude/")
	}
	if !strings.Contains(content, "EVERYTHING") {
		t.Error("coder should emphasize committing everything")
	}
}

func TestFixerPromptCommitInstructions(t *testing.T) {
	params := PromptParams{Goal: "test", TestCmd: "test"}
	content := GenerateFixer(params, "fix", "todo")

	if !strings.Contains(content, ".autoclaude/") {
		t.Error("fixer should mention committing .autoclaude/")
	}
	if !strings.Contains(content, "EVERYTHING") {
		t.Error("fixer should emphasize committing everything")
	}
}

func TestCriticPromptMinorIssuesTodo(t *testing.T) {
	params := PromptParams{Goal: "test", TestCmd: "test"}
	content := GenerateCritic(params)

	if !strings.Contains(content, "MUST add each minor issue") {
		t.Error("critic should instruct adding minor issues as TODOs")
	}
	if !strings.Contains(content, "Priority: low") {
		t.Error("critic should show TODO format with priority")
	}
}
