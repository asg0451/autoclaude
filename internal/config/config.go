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
	ClaudeDir        = ".claude"
	SettingsFile     = "settings.local.json"
	PromptsDir       = "prompts"
	CoderPromptFile  = "coder_prompt.md"
	CriticPromptFile = "critic.md"
	EvalPromptFile   = "evaluator.md"
)

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
	Stop []HookConfig `json:"Stop,omitempty"`
}

// HookConfig represents a hook configuration
type HookConfig struct {
	Hooks []Hook `json:"hooks"`
}

// Hook represents an individual hook
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
	return filepath.Join(PromptsDir, filename)
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
	hook := Hook{
		Type:    "command",
		Command: fmt.Sprintf("%s _continue", autoclaudePath),
	}

	hookConfig := HookConfig{
		Hooks: []Hook{hook},
	}

	if settings.Hooks == nil {
		settings.Hooks = &Hooks{}
	}

	// Check if we already have our hook
	for _, h := range settings.Hooks.Stop {
		for _, inner := range h.Hooks {
			if inner.Command == hook.Command {
				return // Already configured
			}
		}
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

// SetupSettings merges baseline with existing settings and adds the stop hook
func SetupSettings(autoclaudePath string) error {
	baseline, err := LoadBaseline()
	if err != nil {
		return err
	}

	existing, err := LoadExisting()
	if err != nil {
		return err
	}

	merged := MergeSettings(baseline, existing)
	AddStopHook(merged, autoclaudePath)

	return Save(merged)
}

// EnsurePromptsDir creates the prompts directory if it doesn't exist
func EnsurePromptsDir() error {
	return os.MkdirAll(PromptsDir, 0755)
}
