package claude

import (
	"reflect"
	"testing"
)

func TestBuildInteractiveArgs(t *testing.T) {
	tests := []struct {
		name           string
		prompt         string
		permissionMode string
		model          string
		want           []string
	}{
		{
			name:           "no options",
			prompt:         "test prompt",
			permissionMode: "",
			model:          "",
			want:           []string{"--", "test prompt"},
		},
		{
			name:           "with permission mode only",
			prompt:         "test prompt",
			permissionMode: "acceptEdits",
			model:          "",
			want:           []string{"--permission-mode", "acceptEdits", "--", "test prompt"},
		},
		{
			name:           "with model only",
			prompt:         "test prompt",
			permissionMode: "",
			model:          "sonnet",
			want:           []string{"--model", "sonnet", "--", "test prompt"},
		},
		{
			name:           "with both permission mode and model",
			prompt:         "test prompt",
			permissionMode: "acceptEdits",
			model:          "sonnet",
			want:           []string{"--permission-mode", "acceptEdits", "--model", "sonnet", "--", "test prompt"},
		},
		{
			name:           "with opus model",
			prompt:         "do something",
			permissionMode: "plan",
			model:          "opus",
			want:           []string{"--permission-mode", "plan", "--model", "opus", "--", "do something"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildInteractiveArgs(tt.prompt, tt.permissionMode, tt.model)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildInteractiveArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShellEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "'simple'"},
		{"with spaces", "'with spaces'"},
		{"with'quote", "'with'\"'\"'quote'"},
		{"", "''"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shellEscape(tt.input)
			if got != tt.want {
				t.Errorf("shellEscape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseCriticOutput(t *testing.T) {
	tests := []struct {
		name             string
		output           string
		wantApproved     bool
		wantInstructions string
	}{
		{
			name:             "approved uppercase",
			output:           "APPROVED",
			wantApproved:     true,
			wantInstructions: "",
		},
		{
			name:             "approved lowercase",
			output:           "approved with comments",
			wantApproved:     true,
			wantInstructions: "",
		},
		{
			name:             "not approved",
			output:           "Please fix the following issues:\n1. Error handling\n2. Tests",
			wantApproved:     false,
			wantInstructions: "Please fix the following issues:\n1. Error handling\n2. Tests",
		},
		{
			name:             "empty",
			output:           "",
			wantApproved:     false,
			wantInstructions: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotApproved, gotInstructions := ParseCriticOutput(tt.output)
			if gotApproved != tt.wantApproved {
				t.Errorf("ParseCriticOutput() approved = %v, want %v", gotApproved, tt.wantApproved)
			}
			if gotInstructions != tt.wantInstructions {
				t.Errorf("ParseCriticOutput() instructions = %q, want %q", gotInstructions, tt.wantInstructions)
			}
		})
	}
}

func TestParseEvaluatorOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{"complete uppercase", "GOAL_COMPLETE", true},
		{"complete lowercase", "goal_complete", true},
		{"complete with text", "The goal is GOAL_COMPLETE now", true},
		{"not complete", "Still working on it", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseEvaluatorOutput(tt.output)
			if got != tt.want {
				t.Errorf("ParseEvaluatorOutput(%q) = %v, want %v", tt.output, got, tt.want)
			}
		})
	}
}
