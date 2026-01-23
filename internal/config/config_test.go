package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestLoadBaseline(t *testing.T) {
	baseline, err := LoadBaseline()
	if err != nil {
		t.Fatalf("LoadBaseline failed: %v", err)
	}

	if baseline.Permissions == nil {
		t.Fatal("baseline should have permissions")
	}

	if len(baseline.Permissions.Allow) == 0 {
		t.Error("baseline should have allowed permissions")
	}

	// Check for some expected permissions
	allowSet := make(map[string]bool)
	for _, p := range baseline.Permissions.Allow {
		allowSet[p] = true
	}

	expectedPerms := []string{
		"Bash(git add:*)",
		"Bash(git commit:*)",
		"Bash(go test:*)",
		"Bash(gofmt:*)",
	}

	for _, p := range expectedPerms {
		if !allowSet[p] {
			t.Errorf("baseline should include permission %q", p)
		}
	}
}

func TestLoadExistingNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	settings, err := LoadExisting()
	if err != nil {
		t.Fatalf("LoadExisting should not error for missing file: %v", err)
	}

	if settings == nil {
		t.Fatal("LoadExisting should return empty settings, not nil")
	}
}

func TestLoadExisting(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	os.MkdirAll(ClaudeDir, 0755)

	existingSettings := &ClaudeSettings{
		Permissions: &Permissions{
			Allow: []string{"Bash(custom:*)"},
		},
	}
	data, _ := json.Marshal(existingSettings)
	os.WriteFile(SettingsPath(), data, 0644)

	loaded, err := LoadExisting()
	if err != nil {
		t.Fatalf("LoadExisting failed: %v", err)
	}

	if len(loaded.Permissions.Allow) != 1 {
		t.Errorf("expected 1 permission, got %d", len(loaded.Permissions.Allow))
	}
	if loaded.Permissions.Allow[0] != "Bash(custom:*)" {
		t.Errorf("expected custom permission, got %q", loaded.Permissions.Allow[0])
	}
}

func TestMergeSettings(t *testing.T) {
	baseline := &ClaudeSettings{
		Permissions: &Permissions{
			Allow: []string{"Bash(git:*)", "Bash(go:*)"},
			Deny:  []string{"Bash(heredoc:*)", "Bash(rm -rf:*)"}, // baseline deny rules
		},
	}

	existing := &ClaudeSettings{
		Permissions: &Permissions{
			Allow: []string{"Bash(custom:*)", "Bash(git:*)"}, // git duplicated
			Deny:  []string{"Bash(rm -rf:*)"},                // overlaps with baseline
		},
	}

	merged := MergeSettings(baseline, existing)

	if merged.Permissions == nil {
		t.Fatal("merged should have permissions")
	}

	// Check all permissions are present (no duplicates)
	allowSet := make(map[string]bool)
	for _, p := range merged.Permissions.Allow {
		if allowSet[p] {
			t.Errorf("duplicate permission: %s", p)
		}
		allowSet[p] = true
	}

	if !allowSet["Bash(git:*)"] {
		t.Error("should have git permission")
	}
	if !allowSet["Bash(go:*)"] {
		t.Error("should have go permission")
	}
	if !allowSet["Bash(custom:*)"] {
		t.Error("should have custom permission")
	}

	// Check deny lists merged (baseline + existing, deduplicated)
	denySet := make(map[string]bool)
	for _, d := range merged.Permissions.Deny {
		if denySet[d] {
			t.Errorf("duplicate deny: %s", d)
		}
		denySet[d] = true
	}
	if !denySet["Bash(heredoc:*)"] {
		t.Error("should have baseline deny rule")
	}
	if !denySet["Bash(rm -rf:*)"] {
		t.Error("should have existing deny rule")
	}
	if len(denySet) != 2 {
		t.Errorf("expected 2 deny rules, got %d", len(denySet))
	}
}

func TestMergeSettingsEmpty(t *testing.T) {
	baseline := &ClaudeSettings{
		Permissions: &Permissions{
			Allow: []string{"Bash(git:*)"},
		},
	}

	existing := &ClaudeSettings{}

	merged := MergeSettings(baseline, existing)

	if len(merged.Permissions.Allow) != 1 {
		t.Error("should have baseline permission")
	}
}

func TestAddStopHook(t *testing.T) {
	settings := &ClaudeSettings{}

	AddStopHook(settings, "/path/to/autoclaude")

	if settings.Hooks == nil {
		t.Fatal("should have hooks")
	}
	if len(settings.Hooks.Stop) != 1 {
		t.Fatalf("expected 1 stop hook config, got %d", len(settings.Hooks.Stop))
	}
	if len(settings.Hooks.Stop[0].Hooks) != 1 {
		t.Fatal("expected 1 hook in config")
	}
	if settings.Hooks.Stop[0].Hooks[0].Command != "/path/to/autoclaude _continue" {
		t.Errorf("unexpected command: %s", settings.Hooks.Stop[0].Hooks[0].Command)
	}

	// Adding again should not duplicate
	AddStopHook(settings, "/path/to/autoclaude")
	if len(settings.Hooks.Stop) != 1 {
		t.Error("should not duplicate hook")
	}
}

func TestAddNotificationHooks(t *testing.T) {
	settings := &ClaudeSettings{}

	AddNotificationHooks(settings)

	if settings.Hooks == nil {
		t.Fatal("should have hooks")
	}

	// Check PreToolUse hook for AskUserQuestion
	if len(settings.Hooks.PreToolUse) != 1 {
		t.Fatalf("expected 1 PreToolUse hook, got %d", len(settings.Hooks.PreToolUse))
	}
	if settings.Hooks.PreToolUse[0].Matcher != "AskUserQuestion" {
		t.Errorf("expected AskUserQuestion matcher, got %v", settings.Hooks.PreToolUse[0].Matcher)
	}

	// Check Notification hook for permission_prompt
	if len(settings.Hooks.Notification) != 1 {
		t.Fatalf("expected 1 Notification hook, got %d", len(settings.Hooks.Notification))
	}
	if settings.Hooks.Notification[0].Matcher != "permission_prompt" {
		t.Errorf("expected permission_prompt matcher, got %v", settings.Hooks.Notification[0].Matcher)
	}

	// Adding again should not duplicate
	AddNotificationHooks(settings)
	if len(settings.Hooks.PreToolUse) != 1 {
		t.Error("should not duplicate PreToolUse hook")
	}
	if len(settings.Hooks.Notification) != 1 {
		t.Error("should not duplicate Notification hook")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	settings := &ClaudeSettings{
		Permissions: &Permissions{
			Allow: []string{"Bash(test:*)"},
		},
		Hooks: &Hooks{
			Stop: []HookConfig{
				{Hooks: []Hook{{Type: "command", Command: "echo done"}}},
			},
		},
	}

	err := Save(settings)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := LoadExisting()
	if err != nil {
		t.Fatalf("LoadExisting failed: %v", err)
	}

	if len(loaded.Permissions.Allow) != 1 {
		t.Error("permissions not preserved")
	}
	if len(loaded.Hooks.Stop) != 1 {
		t.Error("hooks not preserved")
	}
}

func TestFilterOutCommand(t *testing.T) {
	hookConfigs := []HookConfig{
		{Hooks: []Hook{{Type: "command", Command: "keep1"}, {Type: "command", Command: "remove"}}},
		{Hooks: []Hook{{Type: "command", Command: "remove"}}},
		{Hooks: []Hook{{Type: "command", Command: "keep2"}}},
	}

	filtered := filterOutCommand(hookConfigs, "remove")

	if len(filtered) != 2 {
		t.Fatalf("expected 2 configs after filter, got %d", len(filtered))
	}

	// First config should have only "keep1"
	if len(filtered[0].Hooks) != 1 || filtered[0].Hooks[0].Command != "keep1" {
		t.Error("first config should only have keep1")
	}

	// Second config (originally third) should have "keep2"
	if len(filtered[1].Hooks) != 1 || filtered[1].Hooks[0].Command != "keep2" {
		t.Error("second config should only have keep2")
	}
}

func TestPaths(t *testing.T) {
	if !strings.Contains(SettingsPath(), ".claude") {
		t.Error("SettingsPath should contain .claude")
	}
	if !strings.Contains(CoderPromptPath(), "coder") {
		t.Error("CoderPromptPath should contain coder")
	}
	if !strings.Contains(CriticPromptPath(), "critic") {
		t.Error("CriticPromptPath should contain critic")
	}
	if !strings.Contains(EvaluatorPromptPath(), "evaluator") {
		t.Error("EvaluatorPromptPath should contain evaluator")
	}
	if !strings.Contains(PlannerPromptPath(), "planner") {
		t.Error("PlannerPromptPath should contain planner")
	}
	if !strings.Contains(CurrentPromptPath(), "current") {
		t.Error("CurrentPromptPath should contain current")
	}
}

func TestEnsurePromptsDir(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	err := EnsurePromptsDir()
	if err != nil {
		t.Fatalf("EnsurePromptsDir failed: %v", err)
	}

	if _, err := os.Stat(PromptsDir()); os.IsNotExist(err) {
		t.Error("prompts directory should exist")
	}
}
