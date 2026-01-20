package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed baseline.json
var baselineJSON []byte

const (
	ClaudeDir              = ".claude"
	SettingsFile           = "settings.local.json"
	AutoclaudeDir          = ".autoclaude"
	PromptsSubdir          = "prompts"
	CoderPromptFile        = "coder_prompt.md"
	CriticPromptFile       = "critic.md"
	EvalPromptFile         = "evaluator.md"
	PlannerPromptFile      = "planner_prompt.md"
	CurrentPromptFile      = "current_prompt.md"
	PlanningCompleteFile   = "planning_complete"
	EvaluationCompleteFile = "evaluation_complete"
)

// PromptsDir returns the path to the prompts directory under .autoclaude
func PromptsDir() string {
	return filepath.Join(AutoclaudeDir, PromptsSubdir)
}

// ClaudeSettings represents the Claude settings file structure
type ClaudeSettings struct {
	Permissions *Permissions `json:"permissions,omitempty"`
	Hooks       *Hooks       `json:"hooks,omitempty"`
}

// Permissions represents the permissions section
type Permissions struct {
	Allow []string `json:"allow,omitempty"`
	Deny  []string `json:"deny,omitempty"`
}

// Hooks represents the hooks section
type Hooks struct {
	Stop              []HookConfig `json:"Stop,omitempty"`
	PreToolUse        []HookConfig `json:"PreToolUse,omitempty"`
	Notification      []HookConfig `json:"Notification,omitempty"`
}

// HookConfig represents a hook configuration with matcher
type HookConfig struct {
	Matcher interface{} `json:"matcher,omitempty"` // Tool name pattern string (e.g., "AskUserQuestion") or notification type (e.g., "permission_prompt")
	Hooks   []Hook      `json:"hooks"`
}

// Hook represents an individual hook action
type Hook struct {
	Type    string `json:"type"`
	Command string `json:"command,omitempty"`
}

// SettingsPath returns the path to the Claude settings file
func SettingsPath() string {
	return filepath.Join(ClaudeDir, SettingsFile)
}

// PromptsPath returns the path to a prompt file
func PromptsPath(filename string) string {
	return filepath.Join(PromptsDir(), filename)
}

// CoderPromptPath returns the path to the coder prompt
func CoderPromptPath() string {
	return PromptsPath(CoderPromptFile)
}

// CriticPromptPath returns the path to the critic prompt
func CriticPromptPath() string {
	return PromptsPath(CriticPromptFile)
}

// EvaluatorPromptPath returns the path to the evaluator prompt
func EvaluatorPromptPath() string {
	return PromptsPath(EvalPromptFile)
}

// PlannerPromptPath returns the path to the planner prompt
func PlannerPromptPath() string {
	return PromptsPath(PlannerPromptFile)
}

// CurrentPromptPath returns the path to the current prompt being executed
// This is used to pass prompts to Claude via file to avoid shell quoting issues
func CurrentPromptPath() string {
	return filepath.Join(AutoclaudeDir, CurrentPromptFile)
}

// PlanningCompletePath returns the path to the planning complete marker file
func PlanningCompletePath() string {
	return filepath.Join(AutoclaudeDir, PlanningCompleteFile)
}

// IsPlanningComplete checks if the planning_complete marker file exists
func IsPlanningComplete() bool {
	_, err := os.Stat(PlanningCompletePath())
	return err == nil
}

// RemovePlanningComplete removes the planning_complete marker file
func RemovePlanningComplete() error {
	path := PlanningCompletePath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// EvaluationCompletePath returns the path to the evaluation complete marker file
func EvaluationCompletePath() string {
	return filepath.Join(AutoclaudeDir, EvaluationCompleteFile)
}

// IsEvaluationComplete checks if the evaluation_complete marker file exists
func IsEvaluationComplete() bool {
	_, err := os.Stat(EvaluationCompletePath())
	return err == nil
}

// RemoveEvaluationComplete removes the evaluation_complete marker file
func RemoveEvaluationComplete() error {
	path := EvaluationCompletePath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// LoadBaseline loads the embedded baseline permissions
func LoadBaseline() (*ClaudeSettings, error) {
	var settings ClaudeSettings
	if err := json.Unmarshal(baselineJSON, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse baseline settings: %w", err)
	}
	return &settings, nil
}

// LoadExisting loads existing Claude settings if present
func LoadExisting() (*ClaudeSettings, error) {
	data, err := os.ReadFile(SettingsPath())
	if os.IsNotExist(err) {
		return &ClaudeSettings{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read settings file: %w", err)
	}

	var settings ClaudeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings file: %w", err)
	}
	return &settings, nil
}

// MergeSettings merges baseline settings with existing settings
// Existing settings take precedence for conflicts
func MergeSettings(baseline, existing *ClaudeSettings) *ClaudeSettings {
	result := &ClaudeSettings{}

	// Merge permissions
	allowSet := make(map[string]bool)

	// Add baseline permissions
	if baseline.Permissions != nil {
		for _, p := range baseline.Permissions.Allow {
			allowSet[p] = true
		}
	}

	// Add existing permissions (these take precedence conceptually, but we're just merging)
	if existing.Permissions != nil {
		for _, p := range existing.Permissions.Allow {
			allowSet[p] = true
		}
	}

	// Build merged allow list
	var allowList []string
	for p := range allowSet {
		allowList = append(allowList, p)
	}

	if len(allowList) > 0 {
		result.Permissions = &Permissions{Allow: allowList}

		// Preserve deny list from existing
		if existing.Permissions != nil && len(existing.Permissions.Deny) > 0 {
			result.Permissions.Deny = existing.Permissions.Deny
		}
	}

	// Preserve existing hooks (we'll add our stop hook separately)
	result.Hooks = existing.Hooks

	return result
}

// AddStopHook adds the autoclaude stop hook to settings
func AddStopHook(settings *ClaudeSettings, autoclaudePath string) {
	expectedCmd := fmt.Sprintf("%s _continue", autoclaudePath)

	if settings.Hooks == nil {
		settings.Hooks = &Hooks{}
	}

	// Check if we already have our hook
	for _, hc := range settings.Hooks.Stop {
		for _, h := range hc.Hooks {
			if h.Command == expectedCmd {
				return // Already configured
			}
		}
	}

	hookConfig := HookConfig{
		Hooks: []Hook{{Type: "command", Command: expectedCmd}},
	}
	settings.Hooks.Stop = append(settings.Hooks.Stop, hookConfig)
}

// Save saves the settings to the Claude settings file
func Save(settings *ClaudeSettings) error {
	if err := os.MkdirAll(ClaudeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .claude directory: %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.WriteFile(SettingsPath(), data, 0644); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

// SetupPermissions merges baseline permissions with existing settings (no stop hook)
func SetupPermissions() error {
	baseline, err := LoadBaseline()
	if err != nil {
		return err
	}

	existing, err := LoadExisting()
	if err != nil {
		return err
	}

	merged := MergeSettings(baseline, existing)
	AddNotificationHooks(merged)
	return Save(merged)
}

// AddNotificationHooks adds hooks to ring terminal bell on permission requests and user questions
func AddNotificationHooks(settings *ClaudeSettings) {
	if settings.Hooks == nil {
		settings.Hooks = &Hooks{}
	}

	bellCmd := "printf '\\a'"

	// Remove old-format PreToolUse hooks and check for new format
	var filteredPreToolUse []HookConfig
	hasAskUserHook := false
	for _, hc := range settings.Hooks.PreToolUse {
		if matcherStr, ok := hc.Matcher.(string); ok && matcherStr == "AskUserQuestion" {
			hasAskUserHook = true
			filteredPreToolUse = append(filteredPreToolUse, hc)
		} else if _, isMap := hc.Matcher.(map[string]interface{}); isMap {
			// Skip old-format hooks (object matchers) - they'll be replaced
			continue
		} else {
			filteredPreToolUse = append(filteredPreToolUse, hc)
		}
	}
	settings.Hooks.PreToolUse = filteredPreToolUse
	if !hasAskUserHook {
		settings.Hooks.PreToolUse = append(settings.Hooks.PreToolUse, HookConfig{
			Matcher: "AskUserQuestion",
			Hooks:   []Hook{{Type: "command", Command: bellCmd}},
		})
	}

	// Remove old-format Notification hooks and check for new format
	var filteredNotification []HookConfig
	hasPermissionHook := false
	for _, hc := range settings.Hooks.Notification {
		if matcherStr, ok := hc.Matcher.(string); ok && matcherStr == "permission_prompt" {
			hasPermissionHook = true
			filteredNotification = append(filteredNotification, hc)
		} else if _, isMap := hc.Matcher.(map[string]interface{}); isMap {
			// Skip old-format hooks (object matchers) - they'll be replaced
			continue
		} else {
			filteredNotification = append(filteredNotification, hc)
		}
	}
	settings.Hooks.Notification = filteredNotification
	if !hasPermissionHook {
		settings.Hooks.Notification = append(settings.Hooks.Notification, HookConfig{
			Matcher: "permission_prompt",
			Hooks:   []Hook{{Type: "command", Command: bellCmd}},
		})
	}
}

// SetupStopHook adds the stop hook to settings (called by run, not init)
func SetupStopHook(autoclaudePath string) error {
	existing, err := LoadExisting()
	if err != nil {
		return err
	}

	AddStopHook(existing, autoclaudePath)
	return Save(existing)
}

// RemoveStopHook removes the autoclaude stop hook from settings
func RemoveStopHook(autoclaudePath string) error {
	existing, err := LoadExisting()
	if err != nil {
		return err
	}

	if existing.Hooks == nil || len(existing.Hooks.Stop) == 0 {
		return nil
	}

	expectedCmd := fmt.Sprintf("%s _continue", autoclaudePath)
	existing.Hooks.Stop = filterOutCommand(existing.Hooks.Stop, expectedCmd)

	return Save(existing)
}

// SetupPlannerStopHook adds a stop hook that exits when planner asks for confirmation
func SetupPlannerStopHook(autoclaudePath string) error {
	// Remove any existing planner hook first
	RemovePlannerStopHook(autoclaudePath)

	existing, err := LoadExisting()
	if err != nil {
		return err
	}

	if existing.Hooks == nil {
		existing.Hooks = &Hooks{}
	}

	expectedCmd := fmt.Sprintf("%s _planner-done", autoclaudePath)
	hookConfig := HookConfig{
		Hooks: []Hook{{Type: "command", Command: expectedCmd}},
	}
	existing.Hooks.Stop = append(existing.Hooks.Stop, hookConfig)
	return Save(existing)
}

// RemovePlannerStopHook removes the planner stop hook
func RemovePlannerStopHook(autoclaudePath string) error {
	existing, err := LoadExisting()
	if err != nil {
		return err
	}

	if existing.Hooks == nil || len(existing.Hooks.Stop) == 0 {
		return nil
	}

	expectedCmd := fmt.Sprintf("%s _planner-done", autoclaudePath)
	existing.Hooks.Stop = filterOutCommand(existing.Hooks.Stop, expectedCmd)

	return Save(existing)
}

// SetupEvaluatorStopHook adds a stop hook for the evaluator phase
func SetupEvaluatorStopHook(autoclaudePath string) error {
	// Remove any existing evaluator hook first
	RemoveEvaluatorStopHook(autoclaudePath)

	existing, err := LoadExisting()
	if err != nil {
		return err
	}

	if existing.Hooks == nil {
		existing.Hooks = &Hooks{}
	}

	expectedCmd := fmt.Sprintf("%s _evaluator-done", autoclaudePath)
	hookConfig := HookConfig{
		Hooks: []Hook{{Type: "command", Command: expectedCmd}},
	}
	existing.Hooks.Stop = append(existing.Hooks.Stop, hookConfig)
	return Save(existing)
}

// RemoveEvaluatorStopHook removes the evaluator stop hook
func RemoveEvaluatorStopHook(autoclaudePath string) error {
	existing, err := LoadExisting()
	if err != nil {
		return err
	}

	if existing.Hooks == nil || len(existing.Hooks.Stop) == 0 {
		return nil
	}

	expectedCmd := fmt.Sprintf("%s _evaluator-done", autoclaudePath)
	existing.Hooks.Stop = filterOutCommand(existing.Hooks.Stop, expectedCmd)

	return Save(existing)
}

// filterOutCommand removes hook configs that contain only the specified command
func filterOutCommand(hookConfigs []HookConfig, cmd string) []HookConfig {
	var result []HookConfig
	for _, hc := range hookConfigs {
		var filteredHooks []Hook
		for _, h := range hc.Hooks {
			if h.Command != cmd {
				filteredHooks = append(filteredHooks, h)
			}
		}
		if len(filteredHooks) > 0 {
			result = append(result, HookConfig{Matcher: hc.Matcher, Hooks: filteredHooks})
		}
	}
	return result
}

// EnsurePromptsDir creates the prompts directory if it doesn't exist
func EnsurePromptsDir() error {
	return os.MkdirAll(PromptsDir(), 0755)
}
