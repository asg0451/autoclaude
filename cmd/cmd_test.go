package cmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"go.coldcutz.net/autoclaude/internal/config"
	"go.coldcutz.net/autoclaude/internal/state"
)

func TestHasPendingTasks(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	os.MkdirAll(state.AutoclaudeDir, 0755)

	pendingTasksPath := state.AutoclaudeDir + "/pending_tasks"

	// No file
	if hasPendingTasks() {
		t.Error("should return false when no pending_tasks file")
	}

	// Empty file
	os.WriteFile(pendingTasksPath, []byte(""), 0644)
	if hasPendingTasks() {
		t.Error("should return false for empty file")
	}

	// File contains "no"
	os.WriteFile(pendingTasksPath, []byte("no"), 0644)
	if hasPendingTasks() {
		t.Error("should return false when file contains 'no'")
	}

	// File contains "yes"
	os.WriteFile(pendingTasksPath, []byte("yes"), 0644)
	if !hasPendingTasks() {
		t.Error("should return true when file contains 'yes'")
	}

	// File contains "yes" with whitespace
	os.WriteFile(pendingTasksPath, []byte("  yes  \n"), 0644)
	if !hasPendingTasks() {
		t.Error("should return true when file contains 'yes' with whitespace")
	}
}

func TestGetCommitHash(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// Not a git repo
	hash := getCommitHash()
	if hash != "" {
		t.Error("should return empty string when not a git repo")
	}

	// Initialize git repo
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@test.com").Run()
	exec.Command("git", "config", "user.name", "Test").Run()

	// Still no commits
	hash = getCommitHash()
	if hash != "" {
		t.Error("should return empty string when no commits")
	}

	// Create a commit
	os.WriteFile("test.txt", []byte("test"), 0644)
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "initial").Run()

	hash = getCommitHash()
	if hash == "" {
		t.Error("should return hash after commit")
	}
	if len(hash) < 7 {
		t.Errorf("hash should be at least 7 chars, got %q", hash)
	}
}

func TestForceCommit(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// Initialize git repo
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@test.com").Run()
	exec.Command("git", "config", "user.name", "Test").Run()

	// Initial commit
	os.WriteFile("initial.txt", []byte("initial"), 0644)
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "initial").Run()

	hashBefore := getCommitHash()

	// No changes - should not create new commit
	forceCommit("Test")
	hashAfter := getCommitHash()
	if hashBefore != hashAfter {
		t.Error("forceCommit should not create commit when no changes")
	}

	// Create changes
	os.WriteFile("new.txt", []byte("new file"), 0644)

	forceCommit("Coder")
	hashAfter = getCommitHash()
	if hashBefore == hashAfter {
		t.Error("forceCommit should create commit when changes exist")
	}

	// Check commit message
	out, _ := exec.Command("git", "log", "-1", "--format=%s").Output()
	msg := strings.TrimSpace(string(out))
	if !strings.Contains(msg, "coder") {
		t.Errorf("commit message should contain phase name, got %q", msg)
	}
}

func TestHasUncommittedChanges(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// Initialize git repo
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@test.com").Run()
	exec.Command("git", "config", "user.name", "Test").Run()

	// No changes
	if hasUncommittedChanges() {
		t.Error("should return false when no changes")
	}

	// Create untracked file
	os.WriteFile("new.txt", []byte("new"), 0644)
	if !hasUncommittedChanges() {
		t.Error("should return true for untracked file")
	}

	// Commit it
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "add").Run()

	if hasUncommittedChanges() {
		t.Error("should return false after commit")
	}

	// Modify tracked file
	os.WriteFile("new.txt", []byte("modified"), 0644)
	if !hasUncommittedChanges() {
		t.Error("should return true for modified file")
	}
}

func TestCleanWorkingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// Initialize git repo
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@test.com").Run()
	exec.Command("git", "config", "user.name", "Test").Run()

	// Create and commit a file
	os.WriteFile("tracked.txt", []byte("original"), 0644)
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "initial").Run()

	// Modify tracked file and create untracked file
	os.WriteFile("tracked.txt", []byte("modified"), 0644)
	os.WriteFile("untracked.txt", []byte("untracked"), 0644)

	if !hasUncommittedChanges() {
		t.Fatal("should have uncommitted changes")
	}

	err := cleanWorkingDirectory()
	if err != nil {
		t.Fatalf("cleanWorkingDirectory failed: %v", err)
	}

	if hasUncommittedChanges() {
		t.Error("should have no uncommitted changes after clean")
	}

	// Tracked file should be restored
	data, _ := os.ReadFile("tracked.txt")
	if string(data) != "original" {
		t.Error("tracked file should be restored to original")
	}

	// Untracked file should be removed
	if _, err := os.Stat("untracked.txt"); !os.IsNotExist(err) {
		t.Error("untracked file should be removed")
	}
}

func TestIntegrationInitCreatesFiles(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// Initialize git repo first
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@test.com").Run()
	exec.Command("git", "config", "user.name", "Test").Run()

	// Create go.mod to trigger Go language detection
	os.WriteFile("go.mod", []byte("module test"), 0644)

	// Run state.InitDir (simulating part of init)
	err := state.InitDir("test goal", "go test")
	if err != nil {
		t.Fatalf("InitDir failed: %v", err)
	}

	// Write guidelines
	err = state.WriteGuidelines()
	if err != nil {
		t.Fatalf("WriteGuidelines failed: %v", err)
	}

	// Check files exist
	expectedFiles := []string{
		state.StatePath(),
		state.NotesPath(),
		state.StatusPath(),
		state.GuidelinesPath(),
	}

	// Save state (part of init)
	s := state.NewState("test goal", "go test", "", 3)
	s.Save()

	for _, f := range expectedFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}

	// Check guidelines contain Go section
	data, _ := os.ReadFile(state.GuidelinesPath())
	if !strings.Contains(string(data), "## Go") {
		t.Error("guidelines should contain Go section")
	}
}

func TestIntegrationSetupPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	err := config.SetupPermissions()
	if err != nil {
		t.Fatalf("SetupPermissions failed: %v", err)
	}

	// Check settings file exists
	if _, err := os.Stat(config.SettingsPath()); os.IsNotExist(err) {
		t.Error("settings file should exist")
	}

	// Load and check content
	settings, err := config.LoadExisting()
	if err != nil {
		t.Fatalf("LoadExisting failed: %v", err)
	}

	if settings.Permissions == nil || len(settings.Permissions.Allow) == 0 {
		t.Error("settings should have permissions")
	}

	// Check notification hooks were added
	if settings.Hooks == nil {
		t.Fatal("settings should have hooks")
	}
	if len(settings.Hooks.PreToolUse) == 0 {
		t.Error("settings should have PreToolUse hooks")
	}
	if len(settings.Hooks.Notification) == 0 {
		t.Error("settings should have Notification hooks")
	}
}

func TestIntegrationStopHookLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	autoclaudePath := "/test/autoclaude"

	// Setup stop hook
	err := config.SetupStopHook(autoclaudePath)
	if err != nil {
		t.Fatalf("SetupStopHook failed: %v", err)
	}

	settings, _ := config.LoadExisting()
	if len(settings.Hooks.Stop) == 0 {
		t.Error("should have stop hook after setup")
	}

	// Check hook command
	found := false
	for _, hc := range settings.Hooks.Stop {
		for _, h := range hc.Hooks {
			if strings.Contains(h.Command, "_continue") {
				found = true
			}
		}
	}
	if !found {
		t.Error("stop hook should contain _continue command")
	}

	// Remove stop hook
	err = config.RemoveStopHook(autoclaudePath)
	if err != nil {
		t.Fatalf("RemoveStopHook failed: %v", err)
	}

	settings, _ = config.LoadExisting()
	for _, hc := range settings.Hooks.Stop {
		for _, h := range hc.Hooks {
			if strings.Contains(h.Command, "_continue") {
				t.Error("stop hook should be removed")
			}
		}
	}
}
